package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGoOutputWithColdModuleCache(t *testing.T) {
	// Serve a real module locally so go list emits its download progress on
	// stderr. The license scanner must receive only the JSON from stdout.
	const module = "example.com/noticefixture"
	const version = "v1.0.0"
	const goMod = "module " + module + "\n\ngo 1.21\n"
	proxy := t.TempDir()
	versionDir := filepath.Join(proxy, module, "@v")
	fixtureFile(t, filepath.Join(versionDir, version+".mod"), goMod)
	fixtureFile(t, filepath.Join(versionDir, version+".info"), `{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}`)
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	for name, content := range map[string]string{"go.mod": goMod, "fixture.go": "package fixture\n"} {
		entry, err := w.Create(module + "@" + version + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, filepath.Join(versionDir, version+".zip"), archive.String())
	server := httptest.NewServer(http.FileServer(http.Dir(proxy)))
	t.Cleanup(server.Close)
	cache := t.TempDir()
	t.Setenv("GOMODCACHE", cache)
	t.Setenv("GOPROXY", server.URL)
	t.Setenv("GONOPROXY", "none")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	// Let TempDir remove the downloaded module during test cleanup.
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Chdir(t.TempDir())
	fixtureFile(t, "go.mod", "module fixture.local/app\n\ngo 1.21\n\nrequire "+module+" "+version+"\n")
	fixtureFile(t, "main.go", "package main\nimport _ \""+module+"\"\nfunc main() {}\n")
	output, err := goOutput("", "list", "-mod=mod", "-deps", "-json", ".")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	err = jsonStream(output, func(raw json.RawMessage) error {
		var p struct{ ImportPath string }
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		found = found || p.ImportPath == module
		return nil
	})
	if err != nil || !found {
		t.Fatalf("cold-cache dependency output was not valid JSON: %v; found module: %v", err, found)
	}
	if _, err := os.Stat(filepath.Join(cache, "cache/download", module, "@v", version+".zip")); err != nil {
		t.Fatal("fixture was not downloaded into the empty cache:", err)
	}
	_, err = goOutput("", "list", "-json", "./missing-package")
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || !strings.Contains(err.Error(), "missing-package") {
		t.Fatalf("failed Go command lost its exit error or stderr diagnostics: %v", err)
	}
}

func TestModuleNoticesPreserveNestedLicensesAndAttribution(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "LICENSE"), "root license\n")
	fixtureFile(t, filepath.Join(root, "harfbuzz/LICENSE"), "nested permission and warranty\n")
	fixtureFile(t, filepath.Join(root, "PATENTS"), "patent grant\n")
	fixtureFile(t, filepath.Join(root, "font.go"), "// Copyright 2026 Example Authors\npackage font\nconst privateFixture = 123\n")
	fixtureFile(t, filepath.Join(root, "testdata/font.bin"), "unrelated binary fixture")
	text, files, err := moduleText(dependency{Path: "example.org/font", Version: "v1.2.3", Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"root license", "nested permission and warranty", "patent grant", "Copyright 2026 Example Authors"} {
		if !bytes.Contains(text, []byte(want)) {
			t.Errorf("missing %q", want)
		}
	}
	if len(files) != 3 || files["harfbuzz/LICENSE"] != digest([]byte("nested permission and warranty\n")) {
		t.Fatalf("incomplete source notice inventory: %v", files)
	}
	if bytes.Contains(text, []byte("privateFixture")) || bytes.Contains(text, []byte(root)) || bytes.Contains(text, []byte("unrelated binary")) {
		t.Fatal("notice collector included unrelated source contents or machine paths")
	}
}

func TestMissingAndEmptyModuleLicensesFail(t *testing.T) {
	root := t.TempDir()
	m := dependency{Path: "example.org/missing", Dir: root}
	if _, _, err := moduleText(m); err == nil {
		t.Fatal("accepted a dependency with no license")
	}
	fixtureFile(t, filepath.Join(root, "LICENSE"), " \n")
	if _, _, err := moduleText(m); err == nil {
		t.Fatal("accepted an empty license")
	}
}

func TestDependencyUpgradeRequiresExplicitReview(t *testing.T) {
	reviewed := map[string]reviewedModule{"example.org/lib": {Version: "v1.0.0", License: "MIT"}}
	for _, used := range []map[string]dependency{
		{"example.org/lib": {Version: "v2.0.0"}},
		{"example.org/other": {Version: "v1.0.0"}},
		{},
	} {
		if validateReviewedModules(used, reviewed) == nil {
			t.Fatal("accepted an unreviewed dependency graph")
		}
	}
	if err := validateReviewedModules(map[string]dependency{"example.org/lib": {Version: "v1.0.0"}}, reviewed); err != nil {
		t.Fatal(err)
	}
}

func TestGoHomebrewAndStandardLayoutsProduceSameNotices(t *testing.T) {
	regular, brew := t.TempDir(), filepath.Join(t.TempDir(), "libexec")
	fixtureFile(t, filepath.Join(regular, "LICENSE"), "Go BSD license\n")
	fixtureFile(t, filepath.Join(filepath.Dir(brew), "LICENSE"), "Go BSD license\n")
	for _, root := range []string{regular, brew} {
		fixtureFile(t, filepath.Join(root, "PATENTS"), "Go patent grant\n")
		fixtureFile(t, filepath.Join(root, "src/vendor/example/LICENSE"), "vendor notice\n")
	}
	a, af, err := goNotices(regular)
	if err != nil {
		t.Fatal(err)
	}
	b, bf, err := goNotices(brew)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) || len(af) != 3 || len(bf) != 3 || af["@go/LICENSE"] != bf["@go/LICENSE"] {
		t.Fatal("build host layout changed the license output")
	}
}

func TestChangedAssetOrSourceRequiresReview(t *testing.T) {
	before := licenseReview{Files: map[string]string{"asset.svg": "original", "source.tar.gz": "original"}}
	for _, file := range []string{"asset.svg", "source.tar.gz"} {
		after := licenseReview{Files: map[string]string{"asset.svg": "original", "source.tar.gz": "original"}}
		after.Files[file] = "changed"
		if checkLicenseReview(before, after) == nil {
			t.Fatalf("accepted changed %s without review", file)
		}
	}
}

func TestOutputCheckDoesNotRepairStaleOrUnexpectedCopies(t *testing.T) {
	t.Chdir(t.TempDir())
	path := "resources/THIRD_PARTY_NOTICES.txt"
	fixtureFile(t, path, "stale copy")
	outputs := map[string][]byte{path: []byte("reviewed copy")}
	if checkLicenseOutputs(outputs) == nil {
		t.Fatal("accepted stale output")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "stale copy" {
		t.Fatal("check unexpectedly modified output")
	}
	fixtureFile(t, path, "reviewed copy")
	if err := checkLicenseOutputs(outputs); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, "resources/third-party-sources/obsolete.txt", "old dependency")
	if err := checkLicenseOutputs(outputs); err == nil || !strings.Contains(err.Error(), "unexpected generated") {
		t.Fatalf("did not catch obsolete source copy: %v", err)
	}
}
