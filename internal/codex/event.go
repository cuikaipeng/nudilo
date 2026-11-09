package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"callbell/internal/model"
)

const MaxPayload = 1024 * 1024

type HookInput struct {
	EventName   string          `json:"hook_event_name"`
	SessionID   string          `json:"session_id"`
	TurnID      string          `json:"turn_id"`
	Cwd         string          `json:"cwd"`
	ToolName    string          `json:"tool_name"`
	ToolUseID   string          `json:"tool_use_id"`
	LastMessage string          `json:"last_assistant_message"`
	ToolInput   json.RawMessage `json:"tool_input"`
}

func Normalize(b []byte, includeSummary bool, now time.Time) (*model.Event, error) {
	if len(b) > MaxPayload {
		return nil, fmt.Errorf("hook payload exceeds limit")
	}
	var h HookInput
	if err := json.Unmarshal(b, &h); err != nil {
		return nil, err
	}
	var kind model.Kind
	switch h.EventName {
	case "Stop":
		kind = model.TurnEnded
	case "PermissionRequest":
		kind = model.ApprovalRequired
	case "PreToolUse":
		name := h.ToolName
		if i := strings.LastIndexAny(name, ".:"); i >= 0 {
			name = name[i+1:]
		}
		if name != "request_user_input" && name != "request_user_input_async" {
			return nil, nil
		}
		kind = model.InputRequired
	default:
		return nil, nil
	}
	var args struct {
		Description string `json:"description"`
		Questions   []struct {
			Question    string `json:"question"`
			Title       string `json:"title"`
			Secret      bool   `json:"isSecret"`
			SecretSnake bool   `json:"is_secret"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(h.ToolInput, &args)
	secret := false
	questions := []string{}
	for _, q := range args.Questions {
		secret = secret || q.Secret || q.SecretSnake
		text := q.Question
		if text == "" {
			text = q.Title
		}
		if text != "" {
			questions = append(questions, text)
		}
	}
	summary := ""
	if includeSummary && !secret {
		switch kind {
		case model.TurnEnded:
			summary = h.LastMessage
		case model.InputRequired:
			summary = strings.Join(questions, "\n")
		case model.ApprovalRequired:
			summary = args.Description
		}
	}
	project := path.Base(strings.ReplaceAll(h.Cwd, "\\", "/"))
	if project == "." || project == "/" || project == "" {
		project = "Codex"
	}
	turnKey := h.TurnID
	if turnKey == "" {
		turnKey = fmt.Sprintf("bucket:%d", now.Unix()/5)
	}
	discriminator := h.ToolUseID
	if discriminator == "" && kind != model.TurnEnded {
		var v any
		_ = json.Unmarshal(h.ToolInput, &v)
		canonical, _ := json.Marshal(v)
		discriminator = hash(canonical)
	}
	key := strings.Join([]string{h.SessionID, turnKey, string(kind), discriminator}, "\n")
	return &model.Event{ID: hash([]byte(key)), Kind: kind, SessionID: h.SessionID, TurnID: h.TurnID, Project: truncate(project, 100), Summary: truncate(summary, 500), CreatedAt: now}, nil
}
func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
