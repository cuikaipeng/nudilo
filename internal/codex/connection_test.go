package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
	"callbell/internal/storage"
)

func TestConnectPreservesPreviousHomeAndReceipt(t *testing.T) {
	paths := storage.Paths{Root: t.TempDir()}
	previousHome, nextHome := t.TempDir(), t.TempDir()
	previousHome, _ = filepath.EvalSymlinks(previousHome)
	receiptPath := filepath.Join(paths.Root, "connection.json")
	if err := storage.WriteJSON(receiptPath, receipt{Home: previousHome, Command: "previous helper"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	installer := Installer{Paths: paths}
	_, err = installer.Connect(context.Background(), nextHome, filepath.Join(t.TempDir(), "missing-codex"))
	if err == nil || !strings.Contains(err.Error(), "Disconnect Codex") {
		t.Fatal("switching homes was not rejected before starting Codex", err)
	}
	after, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected change replaced previous connection", err)
	}
	entries, err := os.ReadDir(nextHome)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected change modified new Codex home", err)
	}
	if err := os.WriteFile(receiptPath, []byte("broken receipt"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = installer.Connect(context.Background(), nextHome, filepath.Join(t.TempDir(), "missing-codex"))
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatal("corrupt connection receipt was silently ignored", err)
	}
}

func TestDisconnectClearsReceiptWhenHooksHaveBeenRemoved(t *testing.T) {
	paths := storage.Paths{Root: t.TempDir()}
	receiptPath := filepath.Join(paths.Root, "connection.json")
	if err := storage.WriteJSON(receiptPath, receipt{Home: t.TempDir(), Command: "helper"}); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Paths: paths}
	for attempt := 0; attempt < 2; attempt++ {
		if err := installer.Disconnect(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(receiptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("obsolete receipt still prevents changing configuration folders", err)
	}
}

func TestUpgradeReplacesOnlyTheRecordedHookCommand(t *testing.T) {
	foreign := []byte(`{"custom":true,"hooks":{"Stop":[{"hooks":[{"type":"command","command":"foreign helper"}]}]}}`)
	installed, err := mergeHooks(foreign, "old helper", false)
	if err != nil {
		t.Fatal(err)
	}
	upgraded, err := mergeInstalledHooks(installed, "old helper", "new helper")
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Custom bool `json:"custom"`
		Hooks  map[string][]struct {
			Hooks []struct{ Command string } `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(upgraded, &root); err != nil {
		t.Fatal(err)
	}
	if !root.Custom || root.Hooks["Stop"][0].Hooks[0].Command != "foreign helper" {
		t.Fatal("upgrade changed a foreign hook or its trust-key index")
	}
	for _, event := range []string{"Stop", "PermissionRequest", "PreToolUse"} {
		current := 0
		for _, group := range root.Hooks[event] {
			for _, hook := range group.Hooks {
				if hook.Command == "old helper" {
					t.Fatal("upgrade left the old helper active")
				}
				if hook.Command == "new helper" {
					current++
				}
			}
		}
		if current != 1 {
			t.Fatal("replacement must run exactly once per event", event, current)
		}
	}
	again, err := mergeInstalledHooks(upgraded, "new helper", "new helper")
	if err != nil || !jsonEqual(upgraded, again) {
		t.Fatal("reconnecting changed the upgraded hooks", err)
	}
}

// Execute the real wrapper and helper, using only temporary local files. The
// Windows packaging job also runs this against Windows PowerShell's stdin.
func TestHookCommandPreservesUTF8(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := filepath.Join(t.TempDir(), "用户's data")
	paths := storage.Paths{Root: root}
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	pref := model.Defaults()
	pref.IncludeSummary = true
	if err := storage.WriteJSON(paths.Settings(), pref); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "event helper")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", helper, "../../cmd/callbell-hook")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, output)
	}
	command := hookCommand(helper, root, runtime.GOOS)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if runtime.GOOS == "windows" {
		args := strings.Fields(command)
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}
	want := HookInput{EventName: "Stop", SessionID: "fixture-session", TurnID: "fixture-turn", Cwd: filepath.Join(root, "示例 工程"), LastMessage: "完成 café 🙂"}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = bytes.NewReader(payload)
	if output, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "{}" {
		t.Fatalf("wrapper failed or changed hook response: %v %q", err, output)
	}
	files, err := os.ReadDir(paths.Inbox())
	if err != nil || len(files) != 1 {
		t.Fatal("wrapper did not queue exactly one event", err, len(files))
	}
	var event model.Event
	if err := storage.ReadJSON(filepath.Join(paths.Inbox(), files[0].Name()), &event); err != nil {
		t.Fatal(err)
	}
	if event.Project != "示例 工程" || event.Summary != want.LastMessage || event.TurnID != want.TurnID {
		t.Fatal("wrapper corrupted UTF-8 project or summary", event)
	}
}
