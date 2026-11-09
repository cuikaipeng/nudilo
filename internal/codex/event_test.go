package codex

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestNormalizePrivacyAndKinds(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cases := []struct {
		event, tool string
		kind        model.Kind
	}{{"Stop", "", model.TurnEnded}, {"PermissionRequest", "exec_command", model.ApprovalRequired}, {"PreToolUse", "functions.request_user_input_async", model.InputRequired}}
	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"hook_event_name": tc.event, "tool_name": tc.tool, "session_id": "s1", "turn_id": "t1", "cwd": `C:\Users\Someone\secret-project`, "last_assistant_message": "secret code", "tool_input": map[string]any{"description": "secret command", "questions": []map[string]any{{"question": "private question"}}}})
			e, err := Normalize(b, false, now)
			if err != nil || e == nil {
				t.Fatalf("%v", err)
			}
			if e.Kind != tc.kind || e.Project != "secret-project" || e.Summary != "" {
				t.Fatalf("%+v", e)
			}
			serialized, _ := json.Marshal(e)
			if strings.Contains(string(serialized), "Someone") || strings.Contains(string(serialized), "secret code") {
				t.Fatal("private content leaked")
			}
			e2, _ := Normalize(b, false, now.Add(time.Minute))
			if e.ID != e2.ID {
				t.Fatal("same turn not deduplicated")
			}
		})
	}
	b := []byte(`{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"secret","isSecret":true}]}}`)
	e, _ := Normalize(b, true, now)
	if e.Summary != "" {
		t.Fatal("secret question included")
	}
	e, err := Normalize([]byte(`{"hook_event_name":"PreToolUse","tool_name":"exec_command"}`), false, now)
	if err != nil || e != nil {
		t.Fatal("unrelated tool became a notification")
	}
	if _, err = Normalize([]byte(`not json`), false, now); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestNoTurnIDDoesNotSuppressFutureEvents(t *testing.T) {
	b := []byte(`{"hook_event_name":"Stop","session_id":"s"}`)
	e, _ := Normalize(b, false, time.Unix(100, 0))
	e2, _ := Normalize(b, false, time.Unix(120, 0))
	if e.ID == e2.ID {
		t.Fatal("future turn suppressed")
	}
}
