package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestNotificationContentLanguageFallbackAndPrivacy(t *testing.T) {
	p := Defaults()
	p.SetNotificationContent("en", NotificationContent{Title: "My assistant", TurnEnded: "Done: {project}\n{summary}\n{time}"})
	p.SetNotificationContent("zh-Hans", NotificationContent{Title: "我的助手", TurnEnded: "完成：{project}\n{summary}\n{time}"})
	event := Event{Kind: TurnEnded, Project: "work-{summary}", Summary: "private {time}", CreatedAt: time.Date(2026, 11, 5, 14, 30, 0, 0, time.Local)}
	for _, language := range []string{"en", "zh-Hans"} {
		p.Language = language
		for _, kind := range Kinds {
			event.Kind = kind
			got := p.PrepareNotification(event, "en")
			if !strings.Contains(got.Message(), "Codex | "+kind.Name(language)) || strings.Contains(got.Message(), "private") {
				t.Fatalf("missing event label or leaked summary: %q", got.Message())
			}
			if !strings.Contains(got.Message(), "work-{summary}") {
				t.Fatal("project was recursively expanded", got.Message())
			}
			if kind != TurnEnded && !strings.Contains(got.Message(), kind.DescriptionFor(event.AgentName(), language)) {
				t.Fatal("unset type did not use its own language default", got.Message())
			}
			if strings.Contains(got.Message(), "\n\n\n") {
				t.Fatal("empty summary left excess spacing")
			}
		}
	}
	p.IncludeSummary = true
	event.Kind = TurnEnded
	got := p.PrepareNotification(event, "en")
	if !strings.Contains(got.Message(), "完成：work-{summary}") || !strings.Contains(got.Message(), "private {time}") {
		t.Fatal("variables did not expand exactly once", got.Message())
	}
	data, err := json.Marshal(got)
	if err != nil || strings.Contains(string(data), "完成：") || strings.Contains(string(data), "我的助手") {
		t.Fatal("rendered message was persisted", string(data), err)
	}
	p.SetNotificationContent("zh-Hans", DefaultNotificationContent("zh-Hans"))
	if _, ok := p.MessageContent["zh-Hans"]; ok || p.MessageContent["en"].Title != "My assistant" {
		t.Fatal("reset changed another language or stored default text")
	}
	if !strings.Contains(p.PrepareNotification(event, "en").Message(), TurnEnded.DescriptionFor(event.AgentName(), "zh-Hans")) {
		t.Fatal("fell back to custom English instead of default Chinese")
	}
	p.Language = "system"
	if !strings.Contains(p.PrepareNotification(event, "en").Message(), "Done:") {
		t.Fatal("system language ignored")
	}
	event.Kind = Test
	if text := p.PrepareNotification(event, "en").Message(); !strings.Contains(text, "My assistant · Codex / Claude Code | Connection test") || strings.Contains(text, "Done:") {
		t.Fatal("connection test used a turn-completion body", text)
	}
}

func TestNotificationValidationAndUnicodeBudget(t *testing.T) {
	for _, invalid := range []string{"{unknown}", "{project", "summary}", "{{project}}", strings.Repeat("中", NotificationBodyLimit+1)} {
		if ValidateNotificationBody(invalid) == nil {
			t.Fatalf("accepted invalid body %q", invalid)
		}
	}
	for _, invalid := range []string{"title\nnext", strings.Repeat("中", NotificationTitleLimit+1)} {
		if ValidateNotificationTitle(invalid) == nil {
			t.Fatal("accepted invalid title")
		}
	}
	event := Event{Kind: ApprovalRequired, Language: "zh-Hans", Project: "项目", Summary: strings.Repeat("🙂", 500), CreatedAt: time.Now()}
	content := NotificationContent{Title: "提醒", ApprovalRequired: strings.Repeat("{summary}", 4)}
	text, truncated := RenderNotification(event, content, true)
	if !truncated || !utf8.ValidString(text) || len(text) > NotificationByteLimit || !strings.HasSuffix(text, "…") || !strings.HasPrefix(text, "提醒 · Codex | 需要授权\n") {
		t.Fatalf("invalid truncation (%d bytes): %q", len(text), text)
	}
	content.ApprovalRequired = "{unknown}"
	text, _ = RenderNotification(event, content, false)
	if !strings.Contains(text, ApprovalRequired.DescriptionFor(event.AgentName(), "zh-Hans")) || strings.Contains(text, "🙂") {
		t.Fatal("malformed persisted template did not safely fall back", text)
	}
}

func TestAssistantAwareMessagesFollowLanguageAndKeepLegacyOverrides(t *testing.T) {
	p := Defaults()
	event := Event{Agent: ClaudeCode, Kind: TurnFailed, Project: "demo-{agent}", Summary: "private", CreatedAt: time.Now()}
	for _, language := range []string{"en", "zh-Hans"} {
		p.Language = language
		message := p.PrepareNotification(event, "en").Message()
		if !strings.Contains(message, "Claude Code | "+TurnFailed.Name(language)) || strings.Contains(message, "Codex") || strings.Contains(message, "private") {
			t.Fatal("wrong failure source/language/privacy", message)
		}
		p.SetNotificationContent(language, NotificationContent{TurnFailed: "{agent}: {project}"})
		message = p.PrepareNotification(event, "en").Message()
		if !strings.Contains(message, "Claude Code: demo-{agent}") {
			t.Fatal("variables recursively expanded", message)
		}
	}
	p.SetNotificationContent("en", NotificationContent{TurnEnded: "My existing Codex wording"})
	p.Language = "en"
	event.Kind = TurnEnded
	message := p.PrepareNotification(event, "en").Message()
	if !strings.Contains(message, "Claude Code | Turn ended") || !strings.Contains(message, "My existing Codex wording") {
		t.Fatal("existing customization was rewritten", message)
	}
	if (Event{}).AgentName() != "Codex" {
		t.Fatal("legacy event source not Codex")
	}
}
