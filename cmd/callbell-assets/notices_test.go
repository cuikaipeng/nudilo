package main

import (
	"bytes"
	"os"
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
