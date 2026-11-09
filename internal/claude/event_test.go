package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"callbell/internal/model"
	"callbell/internal/storage"
)

func input(event, tool string) HookInput {
	return HookInput{EventName: event, SessionID: "session", PromptID: "prompt", Cwd: `C:\work\示例工程`, ToolName: tool, ToolUseID: "tool1", LastMessage: "finished private reply", ToolInput: json.RawMessage(`{"description":"Short purpose","command":"secret command","questions":[{"question":"Which option?"}]}`)}
}
func payload(t *testing.T, h HookInput) []byte {
	t.Helper()
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNormalizeOnlyMainConversationAttentionEvents(t *testing.T) {
	for _, tc := range []struct {
		event, tool string
		kind        model.Kind
		summary     string
	}{
		{"Stop", "", model.TurnEnded, "finished private reply"},
		{"StopFailure", "", model.TurnFailed, ""},
		{"PreToolUse", "AskUserQuestion", model.InputRequired, "Which option?"},
		{"PreToolUse", "ExitPlanMode", model.ApprovalRequired, ""},
		{"PermissionRequest", "Bash", model.ApprovalRequired, "Short purpose"},
		{"Elicitation", "", model.InputRequired, ""},
		{"PreToolUse", "Read", "", ""},
		{"PermissionRequest", "AskUserQuestion", "", ""},
		{"PermissionRequest", "ExitPlanMode", "", ""},
		{"SubagentStop", "", "", ""},
		{"Notification", "", "", ""},
		{"SessionEnd", "", "", ""},
	} {
		t.Run(tc.event+tc.tool, func(t *testing.T) {
			h := input(tc.event, tc.tool)
			h.Message = "secret auth URL"
			h.Error = "account error details"
			for _, summary := range []bool{false, true} {
				e, err := Normalize(payload(t, h), summary, time.Now(), "")
				if err != nil {
					t.Fatal(err)
				}
				if tc.kind == "" {
					if e != nil {
						t.Fatal("unwanted event", e)
					}
					continue
				}
				if e == nil || e.Kind != tc.kind || e.Agent != model.ClaudeCode || e.Project != "示例工程" {
					t.Fatal(e)
				}
				want := ""
				if summary {
					want = tc.summary
				}
				if e.Summary != want {
					t.Fatal("summary privacy", e.Summary, want)
				}
			}
			h.AgentID = "subagent"
			e, err := Normalize(payload(t, h), true, time.Now(), "")
			if e != nil || err != nil {
				t.Fatal("subagent generated reminder", e, err)
			}
		})
	}
}

func TestSecretQuestionsAndSummaryLength(t *testing.T) {
	h := input("PreToolUse", "AskUserQuestion")
	for _, key := range []string{"isSecret", "is_secret"} {
		h.ToolInput = json.RawMessage(`{"questions":[{"question":"Enter credentials","` + key + `":true},{"question":"Other question"}]}`)
		e, _ := Normalize(payload(t, h), true, time.Now(), "")
		if e.Summary != "" {
			t.Fatal("secret question leaked")
		}
	}
	h = input("Stop", "")
	h.LastMessage = strings.Repeat("中", 600)
	e, _ := Normalize(payload(t, h), true, time.Now(), "")
	if utf8.RuneCountInString(e.Summary) != 500 || !utf8.ValidString(e.Summary) {
		t.Fatal("summary length")
	}
}

func TestEventIdentitySeparatesPromptsAndDeduplicatesCallbacks(t *testing.T) {
	now := time.Now()
	h := input("Stop", "")
	first, _ := Normalize(payload(t, h), false, now, "")
	repeated, _ := Normalize(payload(t, h), false, now.Add(time.Hour), "")
	if first.ID != repeated.ID || first.TurnID != repeated.TurnID {
		t.Fatal("duplicate callback has a new identity")
	}
	h.PromptID = "next-prompt"
	next, _ := Normalize(payload(t, h), false, now, "")
	if next.ID == first.ID || next.TurnID == first.TurnID {
		t.Fatal("two fast identical turns collapsed")
	}
	h = input("PreToolUse", "AskUserQuestion")
	a, _ := Normalize(payload(t, h), false, now, "")
	h.ToolUseID = "next-tool"
	b, _ := Normalize(payload(t, h), false, now, "")
	if a.ID == b.ID || a.TurnID != b.TurnID {
		t.Fatal("two questions in one turn collapsed")
	}
	h = input("Elicitation", "")
	h.ElicitationID = "request1"
	a, _ = Normalize(payload(t, h), false, now, "")
	h.ElicitationID = "request2"
	b, _ = Normalize(payload(t, h), false, now, "")
	if a.ID == b.ID {
		t.Fatal("MCP requests collapsed")
	}
}

func TestReceiveUsesOpaqueTokensForOlderRuntimes(t *testing.T) {
	p := storage.Paths{Root: t.TempDir()}
	now := time.Now()
	h := input("SessionStart", "")
	h.PromptID = ""
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "Stop", "UserPromptSubmit", "Stop"} {
		h.EventName = event
		if err := Receive(p, payload(t, h), false, now); err != nil {
			t.Fatal(err)
		}
	}
	files, err := os.ReadDir(p.Inbox())
	if err != nil || len(files) != 2 {
		t.Fatal("fallback turn IDs or duplicate suppression failed", len(files), err)
	}
	context := read(t, filepath.Join(p.Root, "claude-turns", hash(h.SessionID)+".json"))
	if bytes.Contains(context, []byte("private")) || bytes.Contains(context, []byte("session")) || bytes.Contains(context, []byte("secret")) {
		t.Fatal("conversation content in token file")
	}
	for _, file := range files {
		b := read(t, filepath.Join(p.Inbox(), file.Name()))
		if bytes.Contains(b, []byte("private")) || bytes.Contains(b, []byte("secret")) {
			t.Fatal("summary not opted in", string(b))
		}
	}
	h.SessionID = ""
	if err := Receive(p, payload(t, h), true, now); err != nil {
		t.Fatal(err)
	}
	if err := Receive(p, []byte(`{broken`), true, now); err == nil {
		t.Fatal("malformed payload accepted")
	}
	if err := Receive(p, bytes.Repeat([]byte("x"), MaxPayload+1), true, now); err == nil {
		t.Fatal("oversized payload accepted")
	}
}

func TestPruneOldCompatibilityTokens(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.json")
	recent := filepath.Join(dir, "recent.json")
	_ = os.WriteFile(old, []byte(`{}`), 0600)
	_ = os.WriteFile(recent, []byte(`{}`), 0600)
	now := time.Now()
	_ = os.Chtimes(old, now.Add(-31*24*time.Hour), now.Add(-31*24*time.Hour))
	pruneTurns(dir, now)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old token retained", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatal("current token removed", err)
	}
}

func TestVersionBoundaries(t *testing.T) {
	for _, value := range []string{"2.1.78 (Claude Code)", "2.1.300 (Claude Code)", "2.2.0 (Claude Code)", "3.0.0 (Claude Code)"} {
		if !versionAtLeast(value, 2, 1, 78) {
			t.Fatal("supported version rejected", value)
		}
	}
	for _, value := range []string{"2.1.77 (Claude Code)", "2.0.999 (Claude Code)", "1.9.999 (Claude Code)", "2.1.300 (Different tool)", "Claude Code unknown"} {
		if versionAtLeast(value, 2, 1, 78) {
			t.Fatal("unsupported version accepted", value)
		}
	}
	if versionAtLeast("2.1.294 (Claude Code)", 2, 1, 295) {
		t.Fatal("onFailure unsupported")
	}
}
