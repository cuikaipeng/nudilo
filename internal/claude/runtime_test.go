package claude

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"callbell/internal/engine"
	"callbell/internal/model"
	"callbell/internal/storage"
)

type testVault struct{}

func (testVault) Get(model.Channel) (model.Credentials, error) { return model.Credentials{}, nil }
func (testVault) Set(model.Channel, model.Credentials) error   { return nil }
func (testVault) Delete(model.Channel) error                   { return nil }

type capturedSender struct {
	events   []model.Event
	channels []model.Channel
}

func (s *capturedSender) Send(_ context.Context, ch model.Channel, _ model.Credentials, event model.Event) error {
	s.events = append(s.events, event)
	s.channels = append(s.channels, ch)
	return nil
}

// Exercise the installed command, stdin, on-disk queue, dispatcher and localized
// rendering together. The sender is in memory: no model or IM service is called.
func TestInstalledHelperToAllChannels(t *testing.T) {
	i, home := installFixture(t)
	if err := os.Remove(i.HelperSource); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		i.HelperSource += ".exe"
	}
	build := exec.Command("go", "build", "-o", i.HelperSource, "../../cmd/callbell-hook")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	if _, err := i.Connect(context.Background(), home, ""); err != nil {
		t.Fatal(err)
	}
	var installed receipt
	if err := storage.ReadJSON(i.receiptPath(), &installed); err != nil {
		t.Fatal(err)
	}
	sender := &capturedSender{}
	e, err := engine.New(i.Paths, testVault{}, sender)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.UpdatePreferences(func(p *model.Preferences) {
		p.Language = "zh-Hans"
		for _, channel := range model.Channels {
			p.Channels[channel] = true
		}
		p.SetNotificationContent("zh-Hans", model.NotificationContent{Title: "我的提醒", TurnEnded: "{agent} 完成了 {project} {summary}"})
	}); err != nil {
		t.Fatal(err)
	}
	run := func(data []byte, codex bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var cmd *exec.Cmd
		if codex {
			cmd = exec.CommandContext(ctx, i.helperPath(), "--codex-event", "--data-dir", i.Paths.Root)
		} else if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", installed.Command)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", installed.Command)
		}
		cmd.Stdin = bytes.NewReader(data)
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatal("helper failed", err, stderr.String())
		}
		want := ""
		if codex {
			want = "{}\n"
		}
		if out.String() != want || stderr.Len() != 0 {
			t.Fatal("helper returned content or a decision", out.String(), stderr.String())
		}
	}
	h := input("Stop", "")
	run(payload(t, h), false)
	e.Step(context.Background(), time.Now())
	if len(sender.events) != len(model.Channels) {
		t.Fatal("not delivered to all configured transports", len(sender.events))
	}
	for _, event := range sender.events {
		if event.Agent != model.ClaudeCode || !strings.Contains(event.Message(), "我的提醒 · Claude Code | 本轮已结束") || !strings.Contains(event.Message(), "Claude Code 完成了 示例工程") || strings.Contains(event.Message(), "private") {
			t.Fatal("wrong source/language/privacy", event.Message())
		}
	}
	// Cached hooks must become silent after disconnect, without disabling Codex.
	if err = e.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = false }); err != nil {
		t.Fatal(err)
	}
	h.PromptID = "next-prompt"
	run(payload(t, h), false)
	run([]byte(`{"hook_event_name":"Stop","session_id":"codex-session","turn_id":"codex-turn","cwd":"/tmp/example"}`), true)
	e.Step(context.Background(), time.Now())
	if len(sender.events) != 2*len(model.Channels) || sender.events[len(model.Channels)].AgentName() != "Codex" {
		t.Fatal("disconnected assistant affected Codex", sender.events)
	}
	if err = e.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = true }); err != nil {
		t.Fatal(err)
	}
	run([]byte(`{broken`), false)
	run(bytes.Repeat([]byte("x"), MaxPayload+1), false)
	files, _ := os.ReadDir(i.Paths.Inbox())
	if len(files) != 0 {
		t.Fatal("malformed or disabled event queued")
	}
	// A lifecycle prompt must never be injected back into Claude via stdout.
	run([]byte(`{"hook_event_name":"UserPromptSubmit","session_id":"s","prompt":"private user input"}`), false)
	state := read(t, filepath.Join(i.Paths.Root, "claude-turns", hash("s")+".json"))
	if bytes.Contains(state, []byte("private")) {
		t.Fatal("prompt persisted")
	}
}

func TestDetectReadsOnlyVersionAndHonorsCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixture")
	}
	p := filepath.Join(t.TempDir(), "Claude Code")
	fixture := []byte("#!/bin/sh\n[ \"$1\" = '--version' ] || exit 1\nprintf '2.1.295 (Claude Code)\\n'\n")
	if err := os.WriteFile(p, fixture, 0700); err != nil {
		t.Fatal(err)
	}
	binary, version, err := Detect(context.Background(), p)
	if err != nil || binary != p || version != "2.1.295 (Claude Code)" {
		t.Fatal(binary, version, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Detect(ctx, p); err == nil {
		t.Fatal("canceled detection ran")
	}
}
