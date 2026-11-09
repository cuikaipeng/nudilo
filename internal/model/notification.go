package model

import (
	"strings"
	"unicode/utf8"

	"callbell/internal/locale"
	"github.com/egoist/mygo/i18n"
)

const (
	NotificationTitleLimit = 80
	NotificationBodyLimit  = 1000
	// A conservative shared UTF-8 budget keeps the same plain text usable by
	// every channel. Truncate after expansion, without splitting a rune.
	NotificationByteLimit = 3000
)

// Empty fields use the active language's default. Each language is independent.
type NotificationContent struct {
	Title            string `json:"title,omitempty"`
	TurnEnded        string `json:"turn_ended,omitempty"`
	InputRequired    string `json:"input_required,omitempty"`
	ApprovalRequired string `json:"approval_required,omitempty"`
	TurnFailed       string `json:"turn_failed,omitempty"`
}

func (n *NotificationContent) Body(kind Kind) *string {
	switch kind {
	case InputRequired:
		return &n.InputRequired
	case ApprovalRequired:
		return &n.ApprovalRequired
	case TurnFailed:
		return &n.TurnFailed
	default:
		return &n.TurnEnded
	}
}

func DefaultNotificationContent(language string) NotificationContent {
	n := NotificationContent{Title: "Nudilo"}
	for _, kind := range Kinds {
		*n.Body(kind) = defaultNotificationBody(language, kind)
	}
	return n
}

func defaultNotificationBody(language string, kind Kind) string {
	return locale.Text(language, "Project: {project}\n{description}", "{project}", kind.DescriptionFor("{agent}", language)) + "\n\n{summary}\n\n{time}"
}

func ValidateNotificationTitle(value string) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > NotificationTitleLimit || strings.ContainsAny(value, "\r\n") {
		return locale.M("Use one line with up to {count} characters.", NotificationTitleLimit)
	}
	return nil
}

func ValidateNotificationBody(value string) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > NotificationBodyLimit {
		return locale.M("Use up to {count} characters.", NotificationBodyLimit)
	}
	// Parse just our supported placeholders. Do not execute expressions or expand
	// variables recursively: project names and summaries remain literal text.
	for rest := value; rest != ""; {
		i := strings.IndexAny(rest, "{}")
		if i < 0 {
			break
		}
		rest = rest[i:]
		end := strings.IndexByte(rest, '}')
		if rest[0] != '{' || end < 0 {
			return locale.M("Use the variable buttons to insert values; check any braces.")
		}
		switch rest[:end+1] {
		case "{project}", "{time}", "{summary}", "{agent}":
		default:
			return locale.M("Use the variable buttons to insert values; check any braces.")
		}
		rest = rest[end+1:]
	}
	return nil
}

func (p Preferences) ValidateNotificationContent() error {
	for _, language := range locale.Catalog.Languages() {
		n := p.MessageContent[language]
		if err := ValidateNotificationTitle(n.Title); err != nil {
			return err
		}
		for _, kind := range Kinds {
			if err := ValidateNotificationBody(*n.Body(kind)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n NotificationContent) Resolved(language string) NotificationContent {
	defaults := DefaultNotificationContent(language)
	if strings.TrimSpace(n.Title) == "" || ValidateNotificationTitle(n.Title) != nil {
		n.Title = defaults.Title
	}
	for _, kind := range Kinds {
		body := n.Body(kind)
		if strings.TrimSpace(*body) == "" || ValidateNotificationBody(*body) != nil {
			*body = *defaults.Body(kind)
		}
	}
	return n
}

// Store only overrides, so restored defaults continue to follow translations.
func (p *Preferences) SetNotificationContent(language string, content NotificationContent) {
	if p.MessageContent == nil {
		p.MessageContent = map[string]NotificationContent{}
	}
	defaults := DefaultNotificationContent(language)
	content.Title = strings.TrimSpace(content.Title)
	if content.Title == defaults.Title {
		content.Title = ""
	}
	for _, kind := range Kinds {
		body := content.Body(kind)
		*body = strings.TrimSpace(strings.ReplaceAll(*body, "\r\n", "\n"))
		if *body == *defaults.Body(kind) {
			*body = ""
		}
	}
	if content == (NotificationContent{}) {
		delete(p.MessageContent, language)
	} else {
		p.MessageContent[language] = content
	}
}

func (p Preferences) PrepareNotification(event Event, systemLanguage string) Event {
	event.Language = locale.Resolve(p.Language, []string{systemLanguage})
	if !p.IncludeSummary {
		event.Summary = ""
	}
	event.RenderedText, _ = RenderNotification(event, p.MessageContent[event.Language], p.IncludeSummary)
	return event
}

// RenderNotification is shared by the live preview and every IM transport.
// The event label and actual assistant name are always present in the header.
func RenderNotification(event Event, content NotificationContent, includeSummary bool) (text string, truncated bool) {
	content = content.Resolved(event.Language)
	var body string
	switch event.Kind {
	case TurnEnded, InputRequired, ApprovalRequired, TurnFailed:
		body = *content.Body(event.Kind)
	default:
		body = defaultNotificationBody(event.Language, event.Kind)
	}
	summary := ""
	if includeSummary {
		summary = limitRunes(event.Summary, 500)
	}
	body = strings.NewReplacer(
		"{agent}", event.AgentName(),
		"{project}", limitRunes(event.Project, 100),
		"{summary}", summary,
		"{time}", i18n.FormatDate(event.Language, event.CreatedAt.Local(), i18n.DateOptions{Skeleton: "MMMdHm"}),
	).Replace(body)
	header := locale.Text(event.Language, "{title} · {agent} | {kind}", strings.TrimSpace(content.Title), event.AgentName(), event.Kind.Name(event.Language))
	text = cleanNotification(header + "\n" + body)
	if len(text) > NotificationByteLimit {
		end := NotificationByteLimit - len("…")
		for !utf8.RuneStart(text[end]) {
			end--
		}
		text, truncated = strings.TrimRight(text[:end], " \n")+"…", true
	}
	return
}

func limitRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return string(runes)
}

func cleanNotification(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.Map(func(r rune) rune {
		if r < ' ' && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if line == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
