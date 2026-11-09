package codex

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

type Connection struct {
	Home, Binary, Version string
	Ready                 bool
}
type Installer struct {
	Paths        storage.Paths
	HelperSource string
}
type receipt struct {
	Home    string `json:"home"`
	Command string `json:"command"`
}
type hookMeta struct {
	Command, Key, CurrentHash, TrustStatus, SourcePath, EventName string
	Enabled                                                       bool
}
type hookList struct {
	Data []struct {
		Hooks  []hookMeta
		Errors []struct{ Message string }
	}
}

func (i Installer) Connect(ctx context.Context, homeOverride, binaryOverride string) (result Connection, err error) {
	result.Home, err = Home(homeOverride)
	if err != nil {
		return result, err
	}
	var previous receipt
	if err = storage.ReadJSON(filepath.Join(i.Paths.Root, "connection.json"), &previous); err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	if previous.Home != "" && previous.Home != result.Home {
		return result, locale.M("Disconnect Codex before changing its configuration folder.")
	}
	result.Binary, result.Version, err = Detect(ctx, binaryOverride)
	if err != nil {
		return result, err
	}
	if err = i.Paths.Prepare(); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	r, err := startRPC(ctx, result.Binary, result.Home)
	if err != nil {
		return result, err
	}
	defer r.close()
	// Check support before touching the user's hook file.
	var initial hookList
	if err = r.call("hooks/list", map[string]any{"cwds": []string{result.Home}}, &initial); err != nil {
		return result, err
	}
	if err = hooksAllowed(r); err != nil {
		return result, err
	}
	helper := filepath.Join(i.Paths.Root, "bin", "callbell-hook")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	data, err := os.ReadFile(i.HelperSource)
	if err != nil {
		return result, errors.New("The event helper is missing. Reinstall Nudilo.")
	}
	oldHelper, _ := os.ReadFile(helper)
	if !bytes.Equal(oldHelper, data) {
		if err = storage.AtomicWrite(helper, data, 0700); err != nil {
			return result, err
		}
	}
	command := hookCommand(helper, i.Paths.Root, runtime.GOOS)
	path := filepath.Join(result.Home, "hooks.json")
	path = resolveExisting(path)
	before, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return result, readErr
	}
	after, err := mergeInstalledHooks(before, previous.Command, command)
	if err != nil {
		return result, err
	}
	changed := !jsonEqual(before, after)
	if changed {
		if err = i.backup(path, before); err != nil {
			return result, err
		}
		current, _ := os.ReadFile(path)
		if !bytes.Equal(current, before) {
			return result, errors.New("Codex settings changed. Reconnect and try again.")
		}
		if err = storage.AtomicWrite(path, after, 0600); err != nil {
			return result, err
		}
		defer func() {
			if err != nil {
				current, _ := os.ReadFile(path)
				if bytes.Equal(current, after) {
					if errors.Is(readErr, os.ErrNotExist) {
						_ = os.Remove(path)
					} else {
						_ = storage.AtomicWrite(path, before, 0600)
					}
				}
			}
		}()
	}
	var list hookList
	if err = r.call("hooks/list", map[string]any{"cwds": []string{result.Home}}, &list); err != nil {
		return result, err
	}
	owned := ownedHooks(list, path, command)
	if len(owned) != 3 {
		return result, errors.New("This Codex version does not support all notification events. Update Codex and try again.")
	}
	var edits []map[string]any
	for _, hook := range owned {
		if !hook.Enabled {
			return result, errors.New("Nudilo events are disabled. Enable them in Codex settings.")
		}
		if hook.TrustStatus != "trusted" {
			quoted, _ := json.Marshal(hook.Key)
			edits = append(edits, map[string]any{"keyPath": "hooks.state." + string(quoted) + ".trusted_hash", "value": hook.CurrentHash, "mergeStrategy": "replace"})
		}
	}
	if len(edits) > 0 {
		configPath := filepath.Join(result.Home, "config.toml")
		configPath = resolveExisting(configPath)
		existing, _ := os.ReadFile(configPath)
		if err = i.backup(configPath, existing); err != nil {
			return result, err
		}
		if err = r.call("config/batchWrite", map[string]any{"edits": edits, "filePath": configPath, "reloadUserConfig": true}, nil); err != nil {
			return result, err
		}
	}
	// A fresh process proves that trust survived a restart.
	verify, err := startRPC(ctx, result.Binary, result.Home)
	if err != nil {
		return result, err
	}
	defer verify.close()
	if err = verify.call("hooks/list", map[string]any{"cwds": []string{result.Home}}, &list); err != nil {
		return result, err
	}
	owned = ownedHooks(list, path, command)
	if len(owned) != 3 {
		return result, errors.New("Could not verify the Codex connection. Try again.")
	}
	for _, hook := range owned {
		if !hook.Enabled || hook.TrustStatus != "trusted" {
			return result, errors.New("Codex did not accept the event helper trust settings.")
		}
	}
	if err = storage.WriteJSON(filepath.Join(i.Paths.Root, "connection.json"), receipt{result.Home, command}); err != nil {
		return result, err
	}
	result.Ready = true
	return result, nil
}

func ownedHooks(list hookList, path, command string) []hookMeta {
	var found []hookMeta
	for _, entry := range list.Data {
		for _, h := range entry.Hooks {
			if filepath.Clean(h.SourcePath) == filepath.Clean(path) && h.Command == command {
				found = append(found, h)
			}
		}
	}
	return found
}

func (i Installer) Disconnect() error {
	var saved receipt
	if err := storage.ReadJSON(filepath.Join(i.Paths.Root, "connection.json"), &saved); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	path := filepath.Join(saved.Home, "hooks.json")
	path = resolveExisting(path)
	before, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.Remove(filepath.Join(i.Paths.Root, "connection.json"))
	}
	if err != nil {
		return err
	}
	after, err := mergeHooks(before, saved.Command, true)
	if err != nil {
		return err
	}
	if !jsonEqual(before, after) {
		if err = i.backup(path, before); err != nil {
			return err
		}
		current, _ := os.ReadFile(path)
		if !bytes.Equal(current, before) {
			return errors.New("Codex settings changed. Try again.")
		}
		if err = storage.AtomicWrite(path, after, 0600); err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(i.Paths.Root, "connection.json"))
}
func (i Installer) backup(path string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	name := fmt.Sprintf("%s-%d.bak", filepath.Base(path), time.Now().UnixNano())
	return storage.AtomicWrite(filepath.Join(i.Paths.Root, "backups", name), data, 0600)
}

// Upgrades may change the wrapper command. Remove only the command recorded by
// our previous installation before registering its replacement.
func mergeInstalledHooks(data []byte, previous, command string) ([]byte, error) {
	if previous != "" && previous != command {
		var err error
		data, err = mergeHooks(data, previous, true)
		if err != nil {
			return nil, err
		}
	}
	return mergeHooks(data, command, false)
}

// Keep every foreign group and its array index: Codex's trust keys contain indices.
func mergeHooks(data []byte, command string, remove bool) ([]byte, error) {
	root := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(data)) > 0 {
		if json.Unmarshal(jsonc.ToJSON(data), &root) != nil || root == nil {
			return nil, errors.New("The existing hooks.json is invalid. The original file was preserved.")
		}
	}
	groups := map[string][]map[string]json.RawMessage{}
	if raw, ok := root["hooks"]; ok {
		if json.Unmarshal(raw, &groups) != nil || groups == nil {
			return nil, errors.New("The existing Hooks format is unsupported. The original file was preserved.")
		}
	}
	for _, event := range []string{"Stop", "PermissionRequest", "PreToolUse"} {
		found := false
		for n, g := range groups[event] {
			var hooks []map[string]json.RawMessage
			if json.Unmarshal(g["hooks"], &hooks) != nil {
				return nil, errors.New("An existing Hook handler is unsupported. The original file was preserved.")
			}
			for j, h := range hooks {
				var cmd string
				_ = json.Unmarshal(h["command"], &cmd)
				if cmd == command {
					found = true
					if remove { // Keep the slot; a disabled no-op cannot shift foreign trust keys.
						hooks[j] = nil
					}
				}
			}
			if remove {
				// Our own groups become empty. Preserve mixed groups' later indices
				// with an inert command only if a user added handlers behind ours.
				for len(hooks) > 0 && hooks[len(hooks)-1] == nil {
					hooks = hooks[:len(hooks)-1]
				}
				for j, h := range hooks {
					if h == nil {
						noop := "true"
						if runtime.GOOS == "windows" {
							noop = "cmd.exe /c exit 0"
						}
						b, _ := json.Marshal(noop)
						hooks[j] = map[string]json.RawMessage{"type": json.RawMessage(`"command"`), "command": b}
					}
				}
				if hooks == nil {
					hooks = []map[string]json.RawMessage{}
				}
				g["hooks"], _ = json.Marshal(hooks)
				groups[event][n] = g
			}
		}
		if !remove && !found {
			h := map[string]any{"hooks": []map[string]any{{"type": "command", "command": command, "timeout": 3}}}
			if event == "PreToolUse" {
				h["matcher"] = "(^|.*[.:])request_user_input(_async)?$"
			}
			b, _ := json.Marshal(h)
			var group map[string]json.RawMessage
			_ = json.Unmarshal(b, &group)
			groups[event] = append(groups[event], group)
		}
	}
	root["hooks"], _ = json.Marshal(groups)
	out, err := json.MarshalIndent(root, "", "  ")
	return append(out, '\n'), err
}
func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(jsonc.ToJSON(a), &x) != nil || json.Unmarshal(jsonc.ToJSON(b), &y) != nil {
		return false
	}
	p, _ := json.Marshal(x)
	q, _ := json.Marshal(y)
	return bytes.Equal(p, q)
}

func resolveExisting(path string) string {
	if actual, err := filepath.EvalSymlinks(path); err == nil {
		return actual
	}
	return path
}

func hookCommand(helper, root, platform string) string {
	if platform != "windows" {
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
		return quote(helper) + " --codex-event --data-dir " + quote(root)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := "$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false); [Console]::InputEncoding = $OutputEncoding; [Console]::In.ReadToEnd() | & " + quote(helper) + " --codex-event --data-dir " + quote(root)
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, len(units)*2)
	for j, u := range units {
		binary.LittleEndian.PutUint16(encoded[j*2:], u)
	}
	return "powershell.exe -NoLogo -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(encoded)
}

// hooksAllowed checks both user preferences and managed policy without changing them.
func hooksAllowed(r *rpc) error {
	var config struct {
		Config struct {
			Features struct {
				Hooks       *bool `json:"hooks"`
				LegacyHooks *bool `json:"codex_hooks"`
			} `json:"features"`
			ManagedOnly bool `json:"allow_managed_hooks_only"`
			Hooks       struct {
				Enabled     *bool `json:"enabled"`
				ManagedOnly bool  `json:"allow_managed_hooks_only"`
			} `json:"hooks"`
		}
	}
	if err := r.call("config/read", map[string]any{"includeLayers": false}, &config); err != nil {
		return err
	}
	var policy struct {
		Requirements *struct {
			ManagedOnly bool            `json:"allowManagedHooksOnly"`
			Features    map[string]bool `json:"featureRequirements"`
		} `json:"requirements"`
	}
	if err := r.call("configRequirements/read", map[string]any{}, &policy); err != nil {
		return err
	}
	hooksEnabled := config.Config.Features.Hooks
	if hooksEnabled == nil {
		hooksEnabled = config.Config.Features.LegacyHooks
	}
	if policy.Requirements != nil {
		if value, ok := policy.Requirements.Features["hooks"]; ok {
			hooksEnabled = &value
		}
		if policy.Requirements.ManagedOnly {
			config.Config.ManagedOnly = true
		}
	}
	if hooksEnabled != nil && !*hooksEnabled || config.Config.ManagedOnly || config.Config.Hooks.Enabled != nil && !*config.Config.Hooks.Enabled || config.Config.Hooks.ManagedOnly {
		return errors.New("Local Hooks are disabled in Codex. Enable them in Codex settings.")
	}
	return nil
}
