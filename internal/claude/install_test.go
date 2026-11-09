package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"

	"callbell/internal/storage"
)

func installFixture(t *testing.T) (Installer, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "Claude config")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "packaged-helper")
	if err := os.WriteFile(helper, []byte("helper-fixture-v1"), 0700); err != nil {
		t.Fatal(err)
	}
	return Installer{Paths: storage.Paths{Root: filepath.Join(root, "Nudilo's data")}, HelperSource: helper, detect: func(context.Context, string) (string, string, error) { return helper, "2.1.295 (Claude Code)", nil }}, home
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestConnectPreservesForeignConfigurationAndDisconnectsOnlyOwnedHooks(t *testing.T) {
	i, home := installFixture(t)
	p := filepath.Join(home, "settings.json")
	original := []byte(`{
  // Preserve this exact original in the backup.
  "model":"user-choice", "permissions":{"deny":["Read(.env)"]}, "unknown":{"nested":42},
  "hooks":{
    "Stop":[{"matcher":"", "hooks":[{"type":"command","command":"another-tool --stop","timeout":45}]}],
    "SessionEnd":[{"hooks":[{"type":"command","command":"other-cleanup"}]}],
    "PreToolUse":[{"matcher":"Bash","hooks":[]}]
  }
}`)
	if err := os.WriteFile(p, original, 0600); err != nil {
		t.Fatal(err)
	}
	connection, err := i.Connect(context.Background(), home, "")
	if err != nil || !connection.Ready {
		t.Fatal(connection, err)
	}
	installed := read(t, p)
	if !bytes.Contains(installed, []byte("another-tool --stop")) || !bytes.Contains(installed, []byte("Read(.env)")) {
		t.Fatal("foreign settings lost")
	}
	backups, _ := filepath.Glob(filepath.Join(i.Paths.Root, "backups", "*.json"))
	if len(backups) != 1 || !bytes.Equal(read(t, backups[0]), original) {
		t.Fatal("original backup missing")
	}
	if _, err = i.Connect(context.Background(), home, ""); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, read(t, p)) {
		t.Fatal("reconnect changed a correct configuration")
	}
	backups, _ = filepath.Glob(filepath.Join(i.Paths.Root, "backups", "*.json"))
	if len(backups) != 1 {
		t.Fatal("unnecessary backup on reconnect")
	}
	// Another tool adds a handler to a group owned by Nudilo after installation.
	root, hooks, _ := parseSettings(installed)
	var groups []object
	_ = json.Unmarshal(hooks["Stop"], &groups)
	last := groups[len(groups)-1]
	var handlers []object
	_ = json.Unmarshal(last["hooks"], &handlers)
	handlers = append(handlers, object{"type": json.RawMessage(`"command"`), "command": json.RawMessage(`"added-after-install"`)})
	last["hooks"], _ = json.Marshal(handlers)
	hooks["Stop"], _ = json.Marshal(groups)
	root["hooks"], _ = json.Marshal(hooks)
	modified, _ := json.Marshal(root)
	if err = os.WriteFile(p, modified, 0600); err != nil {
		t.Fatal(err)
	}
	if err = i.Disconnect(); err != nil {
		t.Fatal(err)
	}
	remaining := read(t, p)
	for _, wanted := range []string{"another-tool --stop", "added-after-install", "other-cleanup", "Read(.env)", "user-choice", "unknown"} {
		if !bytes.Contains(remaining, []byte(wanted)) {
			t.Fatal("disconnect lost", wanted)
		}
	}
	if bytes.Contains(remaining, []byte("--claude-event")) {
		t.Fatal("owned hook retained")
	}
	if _, err = os.Stat(i.helperPath()); err != nil {
		t.Fatal("shared Codex helper removed", err)
	}
	if err = i.Disconnect(); err != nil {
		t.Fatal("disconnect not idempotent", err)
	}
}

func TestCheckIsReadOnlyAndReconnectRepairsChanges(t *testing.T) {
	i, home := installFixture(t)
	ctx := context.Background()
	if _, err := i.Connect(ctx, home, ""); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, "settings.json")
	b := bytes.ReplaceAll(read(t, p), []byte("^(AskUserQuestion|ExitPlanMode)$"), []byte("Bash"))
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if result, err := i.Check(ctx, home, ""); err == nil || result.Ready {
		t.Fatal("changed matcher accepted")
	}
	if !bytes.Equal(b, read(t, p)) {
		t.Fatal("Check mutated settings")
	}
	if _, err := i.Connect(ctx, home, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(i.HelperSource, []byte("helper-v2"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Check(ctx, home, ""); err == nil {
		t.Fatal("stale helper accepted")
	}
	if _, err := i.Connect(ctx, home, ""); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read(t, i.HelperSource), read(t, i.helperPath())) {
		t.Fatal("helper not updated")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(i.helperPath(), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := i.Check(ctx, home, ""); err == nil {
			t.Fatal("non-executable helper accepted")
		}
		if _, err := i.Connect(ctx, home, ""); err != nil {
			t.Fatal("helper permissions not repaired", err)
		}
	}
	if _, err := i.Connect(ctx, t.TempDir(), ""); err == nil {
		t.Fatal("home changed without disconnect")
	}
}

func TestUnsupportedSettingsAndVersionsDoNotChangeFiles(t *testing.T) {
	for _, settings := range []string{`{broken`, `null`, `{"hooks":null}`, `{"hooks":{"Stop":{}}}`, `{"hooks":{"Stop":[{"hooks":null}]}}`, `{"disableAllHooks":true}`} {
		t.Run(settings, func(t *testing.T) {
			i, home := installFixture(t)
			p := filepath.Join(home, "settings.json")
			_ = os.WriteFile(p, []byte(settings), 0600)
			if _, err := i.Connect(context.Background(), home, ""); err == nil {
				t.Fatal("accepted malformed/disabled settings")
			}
			if string(read(t, p)) != settings {
				t.Fatal("original settings changed")
			}
		})
	}
	i, home := installFixture(t)
	i.detect = func(context.Context, string) (string, string, error) {
		return "", "", errors.New("unsupported version")
	}
	if _, err := i.Connect(context.Background(), home, ""); err == nil {
		t.Fatal("version failure ignored")
	}
	if _, err := os.Stat(filepath.Join(home, "settings.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("settings created before version check", err)
	}
}

func TestConcurrentSettingsEditIsPreserved(t *testing.T) {
	i, home := installFixture(t)
	p := filepath.Join(home, "settings.json")
	original := []byte(`{"model":"keep"}`)
	if err := os.WriteFile(p, []byte(`{"model":"new edit"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := i.writeSettings(p, original, []byte(`{}`)); err == nil {
		t.Fatal("concurrent settings change overwritten")
	}
	if !bytes.Contains(read(t, p), []byte("new edit")) {
		t.Fatal("concurrent edit lost")
	}
}

func TestManagedPolicyAndHomeDetection(t *testing.T) {
	p := filepath.Join(t.TempDir(), "managed.json")
	for _, data := range []string{`{"disableAllHooks":true}`, `{"allowManagedHooksOnly":true}`, `{"strictPluginOnlyCustomization":true}`, `{"strictPluginOnlyCustomization":["hooks","skills"]}`} {
		_ = os.WriteFile(p, []byte(data), 0600)
		if err := hooksAllowed([]byte(`{}`), []string{p}); err == nil {
			t.Fatal("policy ignored", data)
		}
	}
	_ = os.WriteFile(p, []byte(`{"strictPluginOnlyCustomization":["skills"]}`), 0600)
	if err := hooksAllowed(nil, []string{p}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	home, err := Home("")
	if err != nil || home == "" {
		t.Fatal(home, err)
	}
	if _, err := Home(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing configuration not recognized", err)
	}
}

func TestWindowsCommandKeepsPathsLiteralAndUTF8(t *testing.T) {
	helper := `C:\工具 O'Brien\$env\helper.exe`
	root := `C:\Data & quotes'\$(secret)`
	command := hookCommand(helper, root, "windows")
	encoded := command[strings.LastIndex(command, " ")+1:]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	units := make([]uint16, len(data)/2)
	for n := range units {
		units[n] = binary.LittleEndian.Uint16(data[n*2:])
	}
	script := string(utf16.Decode(units))
	if !strings.Contains(script, "'"+strings.ReplaceAll(helper, "'", "''")+"'") || !strings.Contains(script, "'"+strings.ReplaceAll(root, "'", "''")+"'") || !strings.Contains(script, "UTF8Encoding") {
		t.Fatal("unsafe Windows command", script)
	}
}
