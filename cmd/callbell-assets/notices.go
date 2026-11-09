package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var licenseTargets = []string{"darwin/amd64", "darwin/arm64", "windows/amd64", "linux/amd64", "linux/arm64"}
var noticeName = regexp.MustCompile(`(?i)^(licen[sc]e|copying|notice|copyright|patents|unlicense|ofl)([._-].*)?$|^.+[._-]license$`)
var copyrightLine = regexp.MustCompile(`(?i)^\s*(//|/\*|\*)?\s*(SPDX-FileCopyrightText:\s*)?Copyright\b.+`)

type reviewedModule struct {
	Version string `json:"version"`
	License string `json:"license"`
}
type component struct {
	Name    string   `json:"name"`
	License string   `json:"license"`
	Source  string   `json:"source"`
	Notes   string   `json:"notes,omitempty"`
	Files   []string `json:"files"`
}
type asset struct {
	File         string `json:"file"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"source_sha256"`
	Adaptation   string `json:"adaptation"`
}
type licenseRegistry struct {
	Modules    map[string]reviewedModule `json:"modules"`
	Components []component               `json:"components"`
	Assets     []asset                   `json:"assets"`
	Sources    []string                  `json:"bundled_sources"`
}
type dependency struct {
	Path, Version, Dir string
	Main               bool
	Replace            *dependency
}
type moduleNotice struct {
	Path    string            `json:"path"`
	Version string            `json:"version"`
	License string            `json:"license"`
	Files   map[string]string `json:"upstream_files"`
	Copy    string            `json:"copy"`
}
type licenseReview struct {
	Schema    int               `json:"schema"`
	GoVersion string            `json:"go_version"`
	Targets   []string          `json:"targets"`
	Graph     map[string]string `json:"selected_module_graph"`
	Modules   []moduleNotice    `json:"compiled_modules"`
	Files     map[string]string `json:"files"`
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func jsonFile(path string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, value)
}
func jsonStream(data []byte, visit func(json.RawMessage) error) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var raw json.RawMessage
		err := dec.Decode(&raw)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = visit(raw); err != nil {
			return err
		}
	}
}
func goOutput(target string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GOOS=") && !strings.HasPrefix(v, "GOARCH=") && !strings.HasPrefix(v, "CGO_ENABLED=") && !strings.HasPrefix(v, "GOWORK=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0", "GOWORK=off")
	if target != "" {
		parts := strings.Split(target, "/")
		cmd.Env = append(cmd.Env, "GOOS="+parts[0], "GOARCH="+parts[1])
	}
	// Go writes download progress to stderr even when the command succeeds.
	// Keep it separate from the JSON stream consumed by the license scanner.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("license dependency scan: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

// Scan the actual app/helper, project tests and pinned build tool on every
// supported architecture. Selected-but-unused modules stay visible in Graph.
func licenseDependencies() (map[string]dependency, map[string]string, error) {
	used := map[string]dependency{}
	graph := map[string]string{}
	raw, err := goOutput("", "list", "-m", "-json", "all")
	if err != nil {
		return nil, nil, err
	}
	err = jsonStream(raw, func(raw json.RawMessage) error {
		var m dependency
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		if m.Replace != nil {
			return fmt.Errorf("review module replacements before packaging: %s", m.Path)
		}
		if !m.Main {
			graph[m.Path] = m.Version
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	for _, target := range licenseTargets {
		raw, err = goOutput(target, "list", "-deps", "-test", "-json", "./...", "github.com/egoist/mygo/cmd/mygo")
		if err != nil {
			return nil, nil, err
		}
		err = jsonStream(raw, func(raw json.RawMessage) error {
			var p struct{ Module *dependency }
			if err := json.Unmarshal(raw, &p); err != nil {
				return err
			}
			if m := p.Module; m != nil && !m.Main {
				if m.Replace != nil {
					return fmt.Errorf("review replaced module: %s", m.Path)
				}
				used[m.Path] = *m
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return used, graph, nil
}

func validateReviewedModules(used map[string]dependency, reviewed map[string]reviewedModule) error {
	if len(used) != len(reviewed) {
		return fmt.Errorf("compiled dependencies changed; review third_party/components.json before refreshing licenses")
	}
	for path, m := range used {
		r, ok := reviewed[path]
		if !ok || r.Version != m.Version || r.License == "" {
			return fmt.Errorf("unreviewed dependency %s %s; review third_party/components.json", path, m.Version)
		}
	}
	return nil
}

// Keep nested notices and patent grants, not just the module's root license.
// Attribution headers preserve names that generic Apache license texts omit.
func moduleText(m dependency) ([]byte, map[string]string, error) {
	files := map[string]string{}
	attributions := map[string]bool{}
	var names []string
	err := filepath.WalkDir(m.Dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(m.Dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if noticeName.MatchString(entry.Name()) {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(data)) == 0 {
				return fmt.Errorf("empty notice: %s/%s", m.Path, rel)
			}
			files[rel] = digest(data)
			names = append(names, rel)
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".s", ".h", ".c", ".m", ".mm", ".js", ".metal", ".glsl", ".hlsl":
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(f, 4096))
			f.Close()
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(data), "\n") {
				if copyrightLine.MatchString(line) {
					attributions[strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "/*"))] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no license found for %s", m.Path)
	}
	sort.Strings(names)
	var out strings.Builder
	fmt.Fprintf(&out, "%s %s\nSource: https://proxy.golang.org/%s/@v/%s.zip\n\n", m.Path, m.Version, m.Path, m.Version)
	out.WriteString("Preserved module notices include nested components; this does not mean every upstream package is shipped.\n\n")
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(m.Dir, filepath.FromSlash(name)))
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(&out, "--- %s ---\n\n%s\n\n", name, data)
	}
	var lines []string
	for line := range attributions {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	if len(lines) > 0 {
		out.WriteString("--- Source-file copyright notices (including upstream test/build files) ---\n\n" + strings.Join(lines, "\n") + "\n")
	}
	return []byte(out.String()), files, nil
}

func localLicensePath(path string) error {
	if !filepath.IsLocal(path) || strings.Contains(path, `\`) {
		return fmt.Errorf("non-local license path: %s", path)
	}
	return nil
}

// Homebrew places Go's LICENSE one level above GOROOT. Keep the logical
// filenames stable so the inventory is identical on all build hosts.
func goNotices(root string) ([]byte, map[string]string, error) {
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if os.IsNotExist(err) && filepath.Base(root) == "libexec" {
		license, err = os.ReadFile(filepath.Join(filepath.Dir(root), "LICENSE"))
	}
	if err != nil {
		return nil, nil, err
	}
	files := map[string]string{"@go/LICENSE": digest(license)}
	var out strings.Builder
	fmt.Fprintf(&out, "--- LICENSE ---\n\n%s\n", license)
	paths := []string{"PATENTS"}
	// These vendored notices also cover platform/experiment-specific code.
	for _, dir := range []string{"src/vendor", "src/crypto/internal/boring"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() && noticeName.MatchString(entry.Name()) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				paths = append(paths, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, nil, err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return nil, nil, fmt.Errorf("empty Go notice: %s", path)
		}
		files["@go/"+path] = digest(data)
		fmt.Fprintf(&out, "\n--- %s ---\n\n%s\n", path, data)
	}
	return []byte(out.String()), files, nil
}

// currentLicenses is read-only. Output and lockfiles contain no machine paths.
func currentLicenses() (licenseReview, map[string][]byte, error) {
	var registry licenseRegistry
	if err := jsonFile("third_party/components.json", &registry); err != nil {
		return licenseReview{}, nil, err
	}
	used, graph, err := licenseDependencies()
	if err != nil {
		return licenseReview{}, nil, err
	}
	if err = validateReviewedModules(used, registry.Modules); err != nil {
		return licenseReview{}, nil, err
	}
	// Use the same Go installation as the dependency/build commands, even if
	// this asset tool was compiled elsewhere or copied to another machine.
	goEnv, err := goOutput("", "env", "-json", "GOROOT", "GOVERSION")
	if err != nil {
		return licenseReview{}, nil, err
	}
	var toolchain struct{ GOROOT, GOVERSION string }
	if err = json.Unmarshal(goEnv, &toolchain); err != nil || toolchain.GOROOT == "" || toolchain.GOVERSION == "" {
		return licenseReview{}, nil, fmt.Errorf("cannot determine the Go toolchain for license review")
	}
	review := licenseReview{Schema: 1, GoVersion: toolchain.GOVERSION, Targets: licenseTargets, Graph: graph, Files: map[string]string{}}
	outputs := map[string][]byte{}
	var out strings.Builder
	out.WriteString("Nudilo — licenses, attributions and source availability\n\nNudilo's own code is MIT licensed. Third-party components retain the licenses below.\nThis file covers all five desktop targets; some notices apply only to a platform or a build-time component.\nCodex and Claude Code clients are not redistributed by Nudilo. Service names and marks identify compatibility; no affiliation or endorsement is implied. Copyright licenses do not grant trademark rights.\n\n")
	include := func(label, path string) error {
		if err := localLicensePath(path); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return fmt.Errorf("empty notice: %s", path)
		}
		review.Files[path] = digest(data)
		fmt.Fprintf(&out, "%s\n%s\n\n%s\n\n", label, strings.Repeat("=", 60), data)
		return nil
	}
	if err = include("Nudilo (MIT)", "LICENSE"); err != nil {
		return review, nil, err
	}
	registryBytes, err := os.ReadFile("third_party/components.json")
	if err != nil {
		return review, nil, err
	}
	review.Files["third_party/components.json"] = digest(registryBytes)
	for _, c := range registry.Components {
		if c.Name == "" || c.License == "" || !strings.HasPrefix(c.Source, "https://") || len(c.Files) == 0 {
			return review, nil, fmt.Errorf("incomplete third-party attribution: %s", c.Name)
		}
		fmt.Fprintf(&out, "%s (%s)\nSource: %s\n%s\n\n", c.Name, c.License, c.Source, c.Notes)
		for _, file := range c.Files {
			if err = include(file, file); err != nil {
				return review, nil, err
			}
		}
	}
	for _, a := range registry.Assets {
		if err = localLicensePath(a.File); err != nil {
			return review, nil, err
		}
		data, err := os.ReadFile(a.File)
		if err != nil {
			return review, nil, err
		}
		review.Files[a.File] = digest(data)
		fmt.Fprintf(&out, "Asset: %s\nSource: %s\nUpstream SHA-256: %s\nChanges: %s\n\n", a.File, a.Source, a.SourceSHA256, a.Adaptation)
	}
	for _, file := range registry.Sources {
		if err = localLicensePath(file); err != nil {
			return review, nil, err
		}
		if !strings.HasPrefix(file, "third_party/sources/") {
			return review, nil, fmt.Errorf("source copy must be under third_party/sources: %s", file)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return review, nil, err
		}
		review.Files[file] = digest(data)
		target := "resources/third-party-sources/" + strings.TrimPrefix(file, "third_party/sources/")
		outputs[target] = data
		fmt.Fprintf(&out, "Bundled source: %s\nSHA-256: %s\n\n", strings.TrimPrefix(target, "resources/"), digest(data))
	}
	goLicense, goFiles, err := goNotices(toolchain.GOROOT)
	if err != nil {
		return review, nil, err
	}
	for file, hash := range goFiles {
		review.Files[file] = hash
	}
	outputs["third_party/licenses/go.txt"] = goLicense
	fmt.Fprintf(&out, "Go standard library and vendored notices (%s)\nSource: https://go.googlesource.com/go/+/refs/tags/%s/\n%s\n\n%s\n\n", toolchain.GOVERSION, toolchain.GOVERSION, strings.Repeat("=", 60), goLicense)
	var paths []string
	for path := range used {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		m := used[path]
		data, files, err := moduleText(m)
		if err != nil {
			return review, nil, err
		}
		file := "third_party/licenses/modules/" + strings.ReplaceAll(path, "/", "_") + "_" + m.Version + ".txt"
		outputs[file] = data
		review.Files[file] = digest(data)
		r := registry.Modules[path]
		review.Modules = append(review.Modules, moduleNotice{path, m.Version, r.License, files, file})
		fmt.Fprintf(&out, "%s (%s)\n%s\n\n%s\n\n", path, r.License, strings.Repeat("=", 60), data)
	}
	outputs["resources/THIRD_PARTY_NOTICES.txt"] = []byte(out.String())
	return review, outputs, nil
}

func checkLicenseReview(expected, actual licenseReview) error {
	a, _ := json.Marshal(expected)
	b, _ := json.Marshal(actual)
	if !bytes.Equal(a, b) {
		return fmt.Errorf("license inventory changed; review dependency/asset notices, then run go run ./cmd/callbell-assets -refresh-licenses")
	}
	return nil
}
func checkLicenseOutputs(outputs map[string][]byte) error {
	for path, want := range outputs {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			return fmt.Errorf("missing or stale license copy: %s; run go run ./cmd/callbell-assets after reviewing notices", path)
		}
	}
	return checkGeneratedFiles(outputs)
}

func checkGeneratedFiles(outputs map[string][]byte) error {
	for _, dir := range []string{"third_party/licenses/modules", "resources/third-party-sources"} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if !entry.IsDir() && outputs[filepath.ToSlash(path)] == nil {
				return fmt.Errorf("unexpected generated license file: %s; review and remove obsolete copies", path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func notices(mode string) error {
	current, outputs, err := currentLicenses()
	if err != nil {
		return err
	}
	if err = checkGeneratedFiles(outputs); err != nil {
		return err
	}
	if mode != "refresh" {
		var reviewed licenseReview
		if err = jsonFile("third_party/inventory.json", &reviewed); err != nil {
			return err
		}
		if err = checkLicenseReview(reviewed, current); err != nil {
			return err
		}
	}
	if mode == "check" {
		if err = checkLicenseOutputs(outputs); err != nil {
			return err
		}
		fmt.Printf("License checks passed: %d compiled modules, %d selected modules, %d desktop targets.\n", len(current.Modules), len(current.Graph), len(current.Targets))
		return nil
	}
	for path, data := range outputs {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	if mode == "refresh" {
		data, err := json.MarshalIndent(current, "", "  ")
		if err != nil {
			return err
		}
		if err = os.WriteFile("third_party/inventory.json", append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}
