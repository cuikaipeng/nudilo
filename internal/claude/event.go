package claude

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"callbell/internal/model"
	"callbell/internal/storage"
)

const MaxPayload = 1024 * 1024

type HookInput struct {
	EventName     string          `json:"hook_event_name"`
	SessionID     string          `json:"session_id"`
	PromptID      string          `json:"prompt_id"`
	AgentID       string          `json:"agent_id"`
	Cwd           string          `json:"cwd"`
	ToolName      string          `json:"tool_name"`
	ToolUseID     string          `json:"tool_use_id"`
	ToolInput     json.RawMessage `json:"tool_input"`
	LastMessage   string          `json:"last_assistant_message"`
	ElicitationID string          `json:"elicitation_id"`
	MCPServer     string          `json:"mcp_server_name"`
	Message       string          `json:"message"`
	Error         string          `json:"error"`
}

type turnContext struct {
	ID string `json:"id"`
}

// Receive never reads a conversation transcript or sends a network request.
// On older runtimes without prompt_id, two quiet lifecycle hooks maintain an
// opaque turn token. No prompts, answers, commands or credentials are stored.
func Receive(paths storage.Paths, payload []byte, includeSummary bool, now time.Time) error {
	if len(payload) > MaxPayload {
		return fmt.Errorf("hook payload exceeds limit")
	}
	var input HookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return err
	}
	if input.SessionID == "" || len(input.SessionID) > 1024 || input.AgentID != "" {
		return nil
	}
	dir := filepath.Join(paths.Root, "claude-turns")
	file := filepath.Join(dir, hash(input.SessionID)+".json")
	var turn turnContext
	if input.EventName == "SessionStart" || input.EventName == "UserPromptSubmit" {
		turn.ID = input.PromptID
		if turn.ID == "" {
			var nonce [16]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				return err
			}
			turn.ID = hex.EncodeToString(nonce[:])
		}
		if err := storage.WriteJSON(file, turn); err != nil {
			return err
		}
		if input.EventName == "SessionStart" {
			pruneTurns(dir, now)
		}
		return nil
	}
	if input.PromptID == "" {
		// A corrupted or absent compatibility token must not stop reminders.
		_ = storage.ReadJSON(file, &turn)
	}
	event, err := Normalize(payload, includeSummary, now, turn.ID)
	if err != nil || event == nil {
		return err
	}
	return paths.QueueEvent(event)
}

func Normalize(payload []byte, includeSummary bool, now time.Time, fallbackTurn string) (*model.Event, error) {
	if len(payload) > MaxPayload {
		return nil, fmt.Errorf("hook payload exceeds limit")
	}
	var h HookInput
	if err := json.Unmarshal(payload, &h); err != nil {
		return nil, err
	}
	if h.SessionID == "" || len(h.SessionID) > 1024 || h.AgentID != "" {
		return nil, nil
	}
	var kind model.Kind
	switch h.EventName {
	case "Stop":
		kind = model.TurnEnded
	case "StopFailure":
		kind = model.TurnFailed
	case "PreToolUse":
		switch h.ToolName {
		case "AskUserQuestion":
			kind = model.InputRequired
		case "ExitPlanMode":
			kind = model.ApprovalRequired
		default:
			return nil, nil
		}
	case "PermissionRequest":
		// These have already emitted a reminder in PreToolUse. Keeping one
		// event also prevents a question from being mislabeled as an approval.
		if h.ToolName == "AskUserQuestion" || h.ToolName == "ExitPlanMode" {
			return nil, nil
		}
		kind = model.ApprovalRequired
	case "Elicitation":
		kind = model.InputRequired
	default:
		return nil, nil
	}
	var args struct {
		Description string `json:"description"`
		Questions   []struct {
			Question    string `json:"question"`
			Secret      bool   `json:"isSecret"`
			SecretSnake bool   `json:"is_secret"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(h.ToolInput, &args)
	summary := ""
	if includeSummary {
		switch h.EventName {
		case "Stop":
			summary = h.LastMessage
		case "PreToolUse":
			if h.ToolName == "AskUserQuestion" {
				questions := []string{}
				secret := false
				for _, q := range args.Questions {
					secret = secret || q.Secret || q.SecretSnake
					if q.Question != "" {
						questions = append(questions, q.Question)
					}
				}
				if !secret {
					summary = strings.Join(questions, "\n")
				}
			}
		case "PermissionRequest":
			summary = args.Description
		}
		// Elicitation can request credentials or carry authentication URLs;
		// API error details can contain account data. Neither becomes a summary.
	}
	project := path.Base(strings.ReplaceAll(h.Cwd, "\\", "/"))
	if project == "" || project == "." || project == "/" {
		project = "Claude Code"
	}
	turn := h.PromptID
	if turn == "" {
		turn = fallbackTurn
	}
	if turn == "" {
		turn = fmt.Sprintf("bucket:%d", now.Unix()/5)
	}
	turnID := "claude-" + hash(h.SessionID+"\n"+turn)
	discriminator := ""
	if kind == model.InputRequired || kind == model.ApprovalRequired {
		discriminator = h.ToolUseID
		if h.EventName == "Elicitation" {
			discriminator = h.ElicitationID
		}
		if discriminator == "" {
			var input any
			_ = json.Unmarshal(h.ToolInput, &input)
			canonical, _ := json.Marshal(input)
			// PermissionRequest has no tool_use_id. A short bucket groups duplicate
			// callbacks while allowing the same operation to ask again later.
			discriminator = hash(fmt.Sprintf("%s\n%s\n%s\n%s\n%d", h.ToolName, canonical, h.MCPServer, h.Message, now.Unix()/5))
		}
	}
	id := "claude-" + hash(strings.Join([]string{turnID, string(kind), discriminator}, "\n"))
	return &model.Event{ID: id, Agent: model.ClaudeCode, Kind: kind, SessionID: h.SessionID, TurnID: turnID, Project: truncate(project, 100), Summary: truncate(summary, 500), CreatedAt: now}, nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return value
}

func pruneTurns(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type file struct {
		path     string
		modified time.Time
	}
	var keep []file
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if now.Sub(info.ModTime()) > 30*24*time.Hour {
			_ = os.Remove(p)
		} else {
			keep = append(keep, file{p, info.ModTime()})
		}
	}
	if len(keep) <= 1000 {
		return
	}
	sort.Slice(keep, func(i, j int) bool { return keep[i].modified.After(keep[j].modified) })
	for _, f := range keep[1000:] {
		_ = os.Remove(f.path)
	}
}
