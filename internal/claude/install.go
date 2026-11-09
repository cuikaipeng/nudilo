// Package claude adapts local Claude Code hooks without starting a model turn.
package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"callbell/internal/locale"
	"callbell/internal/storage"
	"github.com/tidwall/jsonc"
)

var ErrNotFound = errors.New("No local Claude Code configuration found. Install and open Claude Code first.")

type Connection struct {
	Home    string
	Binary  string
	Version string
	Ready   bool
}
type Installer struct {
	Paths        storage.Paths
	HelperSource string
	detect       func(context.Context, string) (string, string, error)
}
type receipt struct {
	Home    string `json:"home"`
	Command string `json:"command"`
}

var hookSpecs = []struct{ event, matcher string }{
	{"SessionStart", ""}, {"UserPromptSubmit", ""}, {"Stop", ""}, {"StopFailure", ""},
	{"PreToolUse", "^(AskUserQuestion|ExitPlanMode)$"}, {"PermissionRequest", ""}, {"Elicitation", ""},
}

func Home(override string) (string, error) {
	home := strings.TrimSpace(override)
	if home == "" {
		home = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(user, ".claude")
	}
	absolute, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if errors.Is(err, os.ErrNotExist) || err == nil && !info.IsDir() {
		return "", ErrNotFound
	}
	if err != nil {
		return "", locale.M("Cannot access the Claude Code configuration folder.")
	}
	return filepath.EvalSymlinks(absolute)
}

func (i Installer) receiptPath() string { return filepath.Join(i.Paths.Root, "claude-connection.json") }
func (i Installer) helperPath() string {
	name := "callbell-hook"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(i.Paths.Root, "bin", name)
}

func settingsPath(home string) string {
	p := filepath.Join(home, "settings.json")
	if actual, err := filepath.EvalSymlinks(p); err == nil {
		return actual
	}
	return p
}

func (i Installer) Connect(ctx context.Context, override, binaryOverride string) (Connection, error) {
	home, err := Home(override)
	result := Connection{Home: home}
	if err != nil {
		return result, err
	}
	detect := i.detect
	if detect == nil {
		detect = Detect
	}
	result.Binary, result.Version, err = detect(ctx, binaryOverride)
	if err != nil {
		return result, err
	}
	var previous receipt
	if err := storage.ReadJSON(i.receiptPath(), &previous); err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	if previous.Home != "" && previous.Home != home {
		return result, locale.M("Disconnect Claude Code before changing its configuration folder.")
	}
	p := settingsPath(home)
	before, err := readSettings(p)
	if err != nil {
		return result, err
	}
	if err := hooksAllowed(before, managedPaths()); err != nil {
		return result, err
	}
	command := hookCommand(i.helperPath(), i.Paths.Root, runtime.GOOS)
	after, err := mergeHooks(before, command, false, versionAtLeast(result.Version, 2, 1, 295))
	if err != nil {
		return result, err
	}
	data, err := os.ReadFile(i.HelperSource)
	if err != nil {
		return result, locale.M("The event helper is missing. Reinstall Nudilo.")
	}
	if err := i.Paths.Prepare(); err != nil {
		return result, err
	}
	currentHelper, _ := os.ReadFile(i.helperPath())
	if !bytes.Equal(currentHelper, data) {
		if err := storage.AtomicWrite(i.helperPath(), data, 0700); err != nil {
			return result, err
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(i.helperPath(), 0700); err != nil {
			return result, err
		}
	}
	changed := !sameJSON(before, after)
	if changed {
		if err := i.writeSettings(p, before, after); err != nil {
			return result, err
		}
	}
	if err := storage.WriteJSON(i.receiptPath(), receipt{home, command}); err != nil {
		// Leave an existing installation intact if its receipt could not be saved.
		if changed {
			current, _ := os.ReadFile(p)
			if bytes.Equal(current, after) {
				if before == nil {
					_ = os.Remove(p)
				} else {
					_ = storage.AtomicWrite(p, before, 0600)
				}
			}
		}
		return result, err
	}
	return i.Check(ctx, home, result.Binary)
}

func (i Installer) Check(ctx context.Context, override, binaryOverride string) (Connection, error) {
	home, err := Home(override)
	result := Connection{Home: home}
	if err != nil {
		return result, err
	}
	detect := i.detect
	if detect == nil {
		detect = Detect
	}
	result.Binary, result.Version, err = detect(ctx, binaryOverride)
	if err != nil {
		return result, err
	}
	data, err := readSettings(settingsPath(home))
	if err != nil {
		return result, err
	}
	if err := hooksAllowed(data, managedPaths()); err != nil {
		return result, err
	}
	command := hookCommand(i.helperPath(), i.Paths.Root, runtime.GOOS)
	if !hasHooks(data, command, versionAtLeast(result.Version, 2, 1, 295)) {
		return result, locale.M("Claude Code listeners are missing or changed. Reconnect to repair them.")
	}
	installed, err := os.ReadFile(i.helperPath())
	expected, sourceErr := os.ReadFile(i.HelperSource)
	if err != nil || sourceErr != nil || !bytes.Equal(installed, expected) {
		return result, locale.M("The Claude Code event helper needs updating. Reconnect to repair it.")
	}
	if info, err := os.Stat(i.helperPath()); err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0100 == 0 {
		return result, locale.M("The Claude Code event helper needs updating. Reconnect to repair it.")
	}
	result.Ready = true
	return result, nil
}

func (i Installer) Disconnect() error {
	var saved receipt
	if err := storage.ReadJSON(i.receiptPath(), &saved); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	p := settingsPath(saved.Home)
	before, err := readSettings(p)
	if err != nil {
		return err
	}
	if before != nil {
		after, err := mergeHooks(before, saved.Command, true, false)
		if err != nil {
			return err
		}
		if !sameJSON(before, after) {
			if err := i.writeSettings(p, before, after); err != nil {
				return err
			}
		}
	}
	err = os.Remove(i.receiptPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readSettings(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, locale.M("Cannot read Claude Code settings. Check folder permissions.")
	}
	if len(b) > 4*1024*1024 {
		return nil, locale.M("Claude Code settings are too large to edit safely.")
	}
	return b, nil
}

func (i Installer) writeSettings(path string, before, after []byte) error {
	if before != nil {
		backup := filepath.Join(i.Paths.Root, "backups", fmt.Sprintf("claude-settings-%d.json", time.Now().UnixNano()))
		if err := storage.AtomicWrite(backup, before, 0600); err != nil {
			return err
		}
	}
	current, err := readSettings(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, before) {
		return locale.M("Claude Code settings changed. Reconnect and try again.")
	}
	return storage.AtomicWrite(path, after, 0600)
}

type object = map[string]json.RawMessage

func parseSettings(data []byte) (object, map[string]json.RawMessage, error) {
	root := object{}
	if len(data) > 0 && (json.Unmarshal(jsonc.ToJSON(data), &root) != nil || root == nil) {
		return nil, nil, locale.M("Claude Code settings are invalid. The original file was preserved.")
	}
	hooks := map[string]json.RawMessage{}
	if raw, exists := root["hooks"]; exists && (json.Unmarshal(raw, &hooks) != nil || hooks == nil) {
		return nil, nil, locale.M("Claude Code hooks have an unsupported format. The original file was preserved.")
	}
	return root, hooks, nil
}

func mergeHooks(data []byte, command string, remove, continueOnFailure bool) ([]byte, error) {
	root, hooks, err := parseSettings(data)
	if err != nil {
		return nil, err
	}
	for _, spec := range hookSpecs {
		var groups []object
		if raw, exists := hooks[spec.event]; exists && (json.Unmarshal(raw, &groups) != nil || groups == nil) {
			return nil, locale.M("Claude Code hooks have an unsupported format. The original file was preserved.")
		}
		kept := []object{}
		for _, group := range groups {
			var handlers []object
			if group == nil || json.Unmarshal(group["hooks"], &handlers) != nil || handlers == nil {
				return nil, locale.M("Claude Code hooks have an unsupported format. The original file was preserved.")
			}
			foreign := []object{}
			owned := false
			for _, handler := range handlers {
				if stringField(handler, "command") == command && stringField(handler, "type") == "command" {
					owned = true
				} else {
					foreign = append(foreign, handler)
				}
			}
			if owned {
				if len(foreign) == 0 {
					continue
				}
				group["hooks"], _ = json.Marshal(foreign)
			}
			kept = append(kept, group)
		}
		if !remove {
			group := object{}
			if spec.matcher != "" {
				group["matcher"], _ = json.Marshal(spec.matcher)
			}
			handler := map[string]any{"type": "command", "command": command, "timeout": 3}
			if continueOnFailure {
				handler["onFailure"] = "continue"
			}
			group["hooks"], _ = json.Marshal([]any{handler})
			kept = append(kept, group)
		}
		if len(kept) == 0 {
			delete(hooks, spec.event)
		} else {
			hooks[spec.event], _ = json.Marshal(kept)
		}
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	} else {
		root["hooks"], _ = json.Marshal(hooks)
	}
	result, err := json.MarshalIndent(root, "", "  ")
	return append(result, '\n'), err
}

func hasHooks(data []byte, command string, continueOnFailure bool) bool {
	_, hooks, err := parseSettings(data)
	if err != nil {
		return false
	}
	for _, spec := range hookSpecs {
		var groups []object
		if json.Unmarshal(hooks[spec.event], &groups) != nil {
			return false
		}
		found := false
		for _, group := range groups {
			if stringField(group, "matcher") != spec.matcher {
				continue
			}
			var handlers []object
			_ = json.Unmarshal(group["hooks"], &handlers)
			for _, handler := range handlers {
				if stringField(handler, "type") == "command" && stringField(handler, "command") == command && len(handler["args"]) == 0 && (!continueOnFailure || stringField(handler, "onFailure") == "continue") {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func stringField(value object, key string) string {
	var s string
	_ = json.Unmarshal(value[key], &s)
	return s
}
func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(jsonc.ToJSON(a), &x) != nil || json.Unmarshal(jsonc.ToJSON(b), &y) != nil {
		return false
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return bytes.Equal(xb, yb)
}

func managedPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/Library/Application Support/ClaudeCode/managed-settings.json"}
	case "windows":
		base := os.Getenv("ProgramFiles")
		if base == "" {
			base = `C:\Program Files`
		}
		return []string{filepath.Join(base, "ClaudeCode", "managed-settings.json")}
	default:
		return []string{"/etc/claude-code/managed-settings.json"}
	}
}

// Observe locally available policy; never turn hooks on or weaken restrictions.
// Project and remotely managed restrictions are still enforced by Claude Code.
func hooksAllowed(user []byte, managed []string) error {
	values := [][]byte{user}
	for _, path := range managed {
		b, err := readSettings(path)
		if err != nil {
			return err
		}
		if b != nil {
			values = append(values, b)
		}
	}
	for index, b := range values {
		root, _, err := parseSettings(b)
		if err != nil {
			return err
		}
		for _, key := range []string{"disableAllHooks", "allowManagedHooksOnly"} {
			var disabled bool
			if raw, ok := root[key]; ok {
				if json.Unmarshal(raw, &disabled) != nil {
					return locale.M("Claude Code settings are invalid. The original file was preserved.")
				}
				if disabled {
					return locale.M("Local hooks are disabled in Claude Code settings or organizational policy.")
				}
			}
		}
		if index > 0 {
			if raw, ok := root["strictPluginOnlyCustomization"]; ok {
				var locked bool
				var surfaces []string
				if json.Unmarshal(raw, &locked) != nil && json.Unmarshal(raw, &surfaces) != nil {
					return locale.M("Claude Code settings are invalid. The original file was preserved.")
				}
				for _, surface := range surfaces {
					locked = locked || surface == "hooks"
				}
				if locked {
					return locale.M("Local hooks are disabled in Claude Code settings or organizational policy.")
				}
			}
		}
	}
	return nil
}

func hookCommand(helper, root, platform string) string {
	if platform != "windows" {
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
		return quote(helper) + " --claude-event --data-dir " + quote(root)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := "$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false); [Console]::InputEncoding = $OutputEncoding; [Console]::In.ReadToEnd() | & " + quote(helper) + " --claude-event --data-dir " + quote(root)
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for n, u := range units {
		binary.LittleEndian.PutUint16(data[n*2:], u)
	}
	return "powershell.exe -NoLogo -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(data)
}
