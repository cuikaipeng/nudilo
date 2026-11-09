package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

func rolloutRecord(t *testing.T, kind string, payload any, at time.Time) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"timestamp": at, "type": kind, "payload": payload})
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}
func rolloutMeta(t *testing.T, source any) string {
	return rolloutRecord(t, "session_meta", map[string]any{"id": "chat", "cwd": "/work/project", "source": source}, time.Now())
}
func rolloutDone(t *testing.T, turn string, at time.Time) string {
	return rolloutRecord(t, "event_msg", map[string]any{"type": "task_complete", "turn_id": turn, "last_agent_message": "private response"}, at)
}
func appendRollout(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(data); err != nil {
		t.Fatal(err)
	}
}
func TestCompletionMonitorExistingChatsAndNoReplay(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "sessions", "2026", "01", "01", "rollout-old.jsonl")
	appendRollout(t, path, rolloutMeta(t, "vscode")+rolloutDone(t, "historical", time.Now().Add(-time.Hour)))
	m, err := NewCompletionMonitor(home)
	if err != nil {
		t.Fatal(err)
	}
	var events []*model.Event
	emit := func(e *model.Event) error { events = append(events, e); return nil }
	poll := func() {
		t.Helper()
		if err := m.Poll(context.Background(), false, emit); err != nil {
			t.Fatal(err)
		}
	}
	poll()
	if len(events) != 0 {
		t.Fatal("replayed history on connect")
	}
	appendRollout(t, path, rolloutRecord(t, "turn_context", map[string]string{"cwd": "/work/current-project"}, time.Now())+rolloutDone(t, "current", time.Now()))
	poll()
	poll()
	if len(events) != 1 || events[0].Summary != "" || events[0].Project != "current-project" {
		t.Fatalf("wrong events: %+v", events)
	}
	hook, _ := Normalize([]byte(`{"hook_event_name":"Stop","session_id":"chat","turn_id":"current","cwd":"/work/current-project"}`), false, time.Now())
	if hook.ID != events[0].ID {
		t.Fatal("hook and fallback have different identities")
	}
	restarted, err := NewCompletionMonitor(home)
	if err != nil {
		t.Fatal(err)
	}
	if err = restarted.Poll(context.Background(), false, emit); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatal("replayed completed events on restart")
	}
	lateFile := filepath.Join(home, "sessions", "rollout-imported.jsonl")
	appendRollout(t, lateFile, rolloutMeta(t, "cli")+rolloutDone(t, "old-import", m.StartedAt.Add(-time.Minute))+rolloutDone(t, "fresh", time.Now()))
	poll()
	if len(events) != 2 || events[1].TurnID != "fresh" {
		t.Fatal("late-discovered file lost new event or replayed old event")
	}
}

func TestCompletionMonitorRejectsOtherRecordsAndSubagents(t *testing.T) {
	home := t.TempDir()
	m, _ := NewCompletionMonitor(home)
	for i, source := range []any{map[string]any{"subagent": map[string]any{"thread_spawn": true}}, "unknown"} {
		appendRollout(t, filepath.Join(home, "sessions", "rollout-"+string(rune('a'+i))+".jsonl"), rolloutMeta(t, source)+rolloutDone(t, "sub-turn", time.Now()))
	}
	path := filepath.Join(home, "sessions", "rollout-main.jsonl")
	appendRollout(t, path, rolloutMeta(t, "vscode")+"not json\n"+rolloutRecord(t, "event_msg", map[string]any{"type": "task_started", "turn_id": "x"}, time.Now())+rolloutRecord(t, "event_msg", map[string]any{"type": "task_complete"}, time.Now())+rolloutRecord(t, "response_item", map[string]any{"type": "message", "role": "assistant", "text": "done"}, time.Now()))
	if err := m.Poll(context.Background(), true, func(*model.Event) error { t.Fatal("unrelated record became a notification"); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionMonitorSplitLinesOversizeAndRetry(t *testing.T) {
	home := t.TempDir()
	m, _ := NewCompletionMonitor(home)
	path := filepath.Join(home, "sessions", "rollout-parts.jsonl")
	appendRollout(t, path, rolloutMeta(t, "cli"))
	line := rolloutDone(t, "split", time.Now())
	appendRollout(t, path, line[:len(line)/2])
	var events []*model.Event
	emit := func(e *model.Event) error { events = append(events, e); return nil }
	if err := m.Poll(context.Background(), true, emit); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatal("partial record emitted")
	}
	appendRollout(t, path, line[len(line)/2:]+strings.Repeat("x", MaxPayload+100)+"\n"+rolloutDone(t, "after-large", time.Now()))
	if err := m.Poll(context.Background(), true, func(*model.Event) error { return errors.New("disk full") }); err == nil {
		t.Fatal("queue failure swallowed")
	}
	if err := m.Poll(context.Background(), false, emit); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].TurnID != "split" || events[1].TurnID != "after-large" || events[0].Summary != "" {
		t.Fatalf("lost pending event or privacy setting: %+v", events)
	}
	// A truncated/replaced file is read again, but still obeys the timestamp gate.
	if err := os.WriteFile(path, []byte(rolloutMeta(t, "cli")+rolloutDone(t, "replacement", time.Now())), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Poll(context.Background(), true, emit); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[2].Summary != "private response" {
		t.Fatal("replacement or opted-in summary lost")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	appendRollout(t, path, rolloutDone(t, "cancelled", time.Now()))
	_ = m.Poll(ctx, true, emit)
	if len(events) != 3 {
		t.Fatal("cancelled monitor emitted event")
	}
}

func TestCompletionMonitorBaselinePartialLine(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "sessions", "rollout-partial.jsonl")
	appendRollout(t, path, rolloutMeta(t, "vscode")+`{"type":"event_msg"`)
	m, _ := NewCompletionMonitor(home)
	appendRollout(t, path, "}\n"+rolloutDone(t, "new", time.Now()))
	var turns []string
	if err := m.Poll(context.Background(), false, func(e *model.Event) error { turns = append(turns, e.TurnID); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0] != "new" {
		t.Fatal("baseline partial line consumed a fresh event", turns)
	}
}

func TestCompletionMonitorBaselinePartialMetadata(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "sessions", "rollout-header.jsonl")
	meta := rolloutMeta(t, "vscode")
	appendRollout(t, path, meta[:len(meta)/2])
	m, err := NewCompletionMonitor(home)
	if err != nil {
		t.Fatal(err)
	}
	appendRollout(t, path, meta[len(meta)/2:]+rolloutDone(t, "after-header", time.Now()))
	count := 0
	if err := m.Poll(context.Background(), false, func(e *model.Event) error { count++; return nil }); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("partially written metadata made the chat permanently ineligible")
	}
}
