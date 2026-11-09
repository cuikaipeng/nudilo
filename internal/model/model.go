package model

import (
	"callbell/internal/locale"
	"strings"
	"time"
)

type Channel string

const (
	Feishu   Channel = "feishu"
	DingTalk Channel = "dingtalk"
	Lark     Channel = "lark"
	Telegram Channel = "telegram"
	Slack    Channel = "slack"
)

var Channels = []Channel{Feishu, DingTalk, Lark, Telegram, Slack}

func (c Channel) Name(language ...string) string {
	switch c {
	case Feishu:
		return locale.Text(lang(language), "Feishu")
	case DingTalk:
		return locale.Text(lang(language), "DingTalk")
	case Lark:
		return locale.Text(lang(language), "Lark")
	case Telegram:
		return locale.Text(lang(language), "Telegram")
	case Slack:
		return locale.Text(lang(language), "Slack")
	}
	return string(c)
}

type Kind string

const (
	TurnEnded        Kind = "turn_ended"
	InputRequired    Kind = "input_required"
	ApprovalRequired Kind = "approval_required"
	TurnFailed       Kind = "turn_failed"
	Test             Kind = "test"
)

var Kinds = []Kind{TurnEnded, InputRequired, ApprovalRequired, TurnFailed}

func (k Kind) Name(language ...string) string {
	switch k {
	case TurnEnded:
		return locale.Text(lang(language), "Turn ended")
	case InputRequired:
		return locale.Text(lang(language), "Answer needed")
	case ApprovalRequired:
		return locale.Text(lang(language), "Approval needed")
	case TurnFailed:
		return locale.Text(lang(language), "Interrupted")
	case Test:
		return locale.Text(lang(language), "Connection test")
	}
	return locale.Text(lang(language), "Codex notification")
}
func (k Kind) DescriptionFor(agent, language string) string {
	switch k {
	case TurnEnded:
		return locale.Text(language, "{agent} has finished this turn. Open the chat to review the result.", agent)
	case InputRequired:
		return locale.Text(language, "{agent} has a question for you. Open the chat to answer.", agent)
	case ApprovalRequired:
		return locale.Text(language, "{agent} is waiting for your approval. Open the chat to continue.", agent)
	case TurnFailed:
		return locale.Text(language, "{agent} could not finish this turn. Open the chat to check the error.", agent)
	case Test:
		return locale.Text(language, "Your notification channel is connected.")
	}
	return locale.Text(language, "Open {agent} to continue.", agent)
}

type Event struct {
	ID        string    `json:"id"`
	Agent     Agent     `json:"agent,omitempty"`
	Language  string    `json:"language,omitempty"`
	Kind      Kind      `json:"kind"`
	SessionID string    `json:"session_id,omitempty"`
	TurnID    string    `json:"turn_id,omitempty"`
	Project   string    `json:"project"`
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// Render only at delivery time; never cache message text in the queue/history.
	RenderedText string `json:"-"`
}

func (e Event) Message() string {
	if e.RenderedText != "" {
		return e.RenderedText
	}
	text, _ := RenderNotification(e, NotificationContent{}, true)
	return text
}

type Preferences struct {
	Theme             string                         `json:"theme"`
	Language          string                         `json:"language"`
	Paused            bool                           `json:"paused"`
	PauseStarted      time.Time                      `json:"pause_started,omitempty"`
	PauseUntil        time.Time                      `json:"pause_until,omitempty"`
	SetupDismissed    bool                           `json:"setup_dismissed,omitempty"`
	ChannelSince      map[Channel]time.Time          `json:"channel_since,omitempty"`
	Routes            []ProjectRoute                 `json:"project_routes,omitempty"`
	IncludeSummary    bool                           `json:"include_summary"`
	AutoConnect       bool                           `json:"auto_connect"`
	AutomaticUpdates  bool                           `json:"automatic_updates"`
	ClaudeAutoConnect bool                           `json:"claude_auto_connect"`
	ClaudeHome        string                         `json:"claude_home,omitempty"`
	ClaudeBinary      string                         `json:"claude_binary,omitempty"`
	CodexHome         string                         `json:"codex_home,omitempty"`
	CodexBinary       string                         `json:"codex_binary,omitempty"`
	Channels          map[Channel]bool               `json:"channels"`
	ChannelModes      map[Channel]DeliveryMode       `json:"channel_modes,omitempty"`
	Kinds             map[Kind]bool                  `json:"kinds"`
	MessageContent    map[string]NotificationContent `json:"message_content,omitempty"`
}

func Defaults() Preferences {
	return Preferences{Theme: "system", Language: "system", AutoConnect: true, ClaudeAutoConnect: true, AutomaticUpdates: true, Channels: map[Channel]bool{}, Kinds: map[Kind]bool{TurnEnded: true, InputRequired: true, ApprovalRequired: true, TurnFailed: true}}
}

// Stored only in the encrypted local credential file, never in preferences/history.
type Credentials struct {
	Mode          DeliveryMode `json:"mode,omitempty"`
	Webhook       string       `json:"webhook,omitempty"`
	Secret        string       `json:"secret,omitempty"`
	Token         string       `json:"token,omitempty"`
	ChatID        string       `json:"chat_id,omitempty"`
	AppID         string       `json:"app_id,omitempty"`
	AppSecret     string       `json:"app_secret,omitempty"`
	RobotCode     string       `json:"robot_code,omitempty"`
	RecipientID   string       `json:"recipient_id,omitempty"`
	RecipientType string       `json:"recipient_type,omitempty"`
}

type DeliveryMode string

const (
	Group  DeliveryMode = "group"
	Direct DeliveryMode = "direct"
)

func (m DeliveryMode) Name(language ...string) string {
	if m == Direct {
		return locale.Text(lang(language), "Personal DM")
	}
	return locale.Text(lang(language), "Group / channel")
}

// Normalize migrates old credentials without changing their destination. Both
// modes' fields stay in the vault so switching mode does not discard setup.
func (c *Credentials) Normalize(channel Channel) {
	for _, value := range []*string{&c.Webhook, &c.Secret, &c.Token, &c.ChatID, &c.AppID, &c.AppSecret, &c.RobotCode, &c.RecipientID, &c.RecipientType} {
		*value = strings.TrimSpace(*value)
	}
	if c.Mode == "" {
		c.Mode = Group
		if channel == Telegram && !strings.HasPrefix(c.ChatID, "-") && !strings.HasPrefix(c.ChatID, "@") {
			c.Mode = Direct
			if c.RecipientID == "" {
				c.RecipientID = c.ChatID
			}
			c.ChatID = ""
		}
	}
	if c.RecipientType == "" {
		c.RecipientType = "email"
		if strings.HasPrefix(c.RecipientID, "ou_") {
			c.RecipientType = "open_id"
		}
	}
}

type State string

const (
	Pending State = "pending"
	Sent    State = "sent"
	Failed  State = "failed"
	Skipped State = "skipped"
	Expired State = "expired"
)

func (s State) Name(language ...string) string {
	switch s {
	case Pending:
		return locale.Text(lang(language), "Pending")
	case Sent:
		return locale.Text(lang(language), "Delivered to platform")
	case Failed:
		return locale.Text(lang(language), "Failed")
	case Skipped:
		return locale.Text(lang(language), "Skipped")
	case Expired:
		return locale.Text(lang(language), "Expired")
	}
	return string(s)
}

type Delivery struct {
	State        State           `json:"state"`
	Attempts     int             `json:"attempts"`
	LastAttempt  time.Time       `json:"last_attempt,omitempty"`
	NextAttempt  time.Time       `json:"next_attempt,omitempty"`
	Error        string          `json:"error,omitempty"`
	ErrorMessage *locale.Message `json:"error_message,omitempty"`
}
type Record struct {
	Event      Event                `json:"event"`
	Deliveries map[Channel]Delivery `json:"deliveries"`
	Note       string               `json:"note,omitempty"`
	// Cleared records retain only event identity and time for deduplication.
	Cleared bool `json:"cleared,omitempty"`
}

func (r Record) State() State {
	if len(r.Deliveries) == 0 {
		return Skipped
	}
	result := Sent
	for _, d := range r.Deliveries {
		if d.State == Pending {
			return Pending
		}
		if d.State == Failed {
			result = Failed
		} else if d.State == Expired && result != Failed {
			result = Expired
		} else if d.State == Skipped && result == Sent {
			result = Skipped
		}
	}
	return result
}

func lang(values []string) string {
	if len(values) > 0 && values[0] != "" {
		return values[0]
	}
	return "en"
}

func (p *Preferences) Normalize() {
	p.Language = locale.Normalize(p.Language)
	switch p.Theme {
	case "light", "dark":
	default:
		p.Theme = "system"
	}
	if p.Channels == nil {
		p.Channels = map[Channel]bool{}
	}
	if p.ChannelModes == nil {
		p.ChannelModes = map[Channel]DeliveryMode{}
	}
	if p.Kinds == nil {
		p.Kinds = Defaults().Kinds
	}
	if _, exists := p.Kinds[TurnFailed]; !exists {
		p.Kinds[TurnFailed] = true
	}
	if p.MessageContent == nil {
		p.MessageContent = map[string]NotificationContent{}
	}
	if p.ChannelSince == nil {
		p.ChannelSince = map[Channel]time.Time{}
	}
}
