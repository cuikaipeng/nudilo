package desktop

import (
	"strings"
	"testing"
	"time"

	"callbell/internal/claude"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func TestAssistantStatusesUseTheirOwnRecentEvents(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.Language = "en"
	now := time.Now()
	a.connection.Ready = true
	a.claudeConnection.Ready = true
	a.monitorSince = now
	a.claudeSince = now
	a.snapshot.LastEvents = map[model.Agent]time.Time{model.Codex: now.Add(-time.Hour), model.ClaudeCode: now.Add(time.Second)}
	if title, _ := a.connectionStatus(); title != "Listening for Codex" {
		t.Fatal("Claude event confirmed Codex", title)
	}
	if title, _ := a.claudeStatus(); title != "Claude Code connected" {
		t.Fatal(title)
	}
	a.snapshot.LastEvents[model.Codex] = now.Add(time.Second)
	a.snapshot.LastEvents[model.ClaudeCode] = now.Add(-time.Hour)
	if title, _ := a.connectionStatus(); title != "Codex connected" {
		t.Fatal(title)
	}
	if title, _ := a.claudeStatus(); title != "Listening for Claude Code" {
		t.Fatal("old event confirmed Claude", title)
	}
	a.claudeError = claude.ErrNotFound
	if title, _ := a.claudeStatus(); title != "Claude Code not found" {
		t.Fatal("missing install shown as an error", title)
	}
	a.snapshot.Preferences.AutoConnect = false
	a.connection.Ready = false
	a.claudeError = nil
	a.snapshot.Preferences.Channels[model.Feishu] = true
	a.savedChannels[model.Feishu] = setupFixture(model.Feishu, model.Direct)
	tt := ui.NewTester(a.view, 880, 620)
	if !tt.HasText("Ready for your next reminder") {
		t.Fatal("Claude-only setup not ready", tt.Texts())
	}
}

func TestAssistantPreviewVariableAndFailureTemplate(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.Language = "en"
	draft := a.notificationDraft("en")
	draft.content.TurnEnded = ""
	tt := notificationEditorTester(a, 780, 620)
	if err := tt.Click("Message text"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Hello ")
	if err := tt.Click("Assistant"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Claude Code"); err != nil {
		t.Fatal(err)
	}
	if draft.content.TurnEnded != "Hello {agent}" || !strings.Contains(strings.Join(tt.Texts(), "\n"), "Hello Claude Code") {
		t.Fatal("assistant insertion or preview failed", tt.Texts())
	}
	if err := tt.Click("Interrupted"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Claude Code could not finish this turn") {
		t.Fatal("failure preview missing")
	}
	a.snapshot.Preferences.Language = "zh-Hans"
	tt.Frame()
	zh := a.notificationDraft("zh-Hans")
	zh.agent = 1
	zh.kind = 3
	tt.Frame()
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Claude Code") || !tt.HasText("异常中断") {
		t.Fatal("Chinese failure preview missing")
	}
	a.snapshot.Preferences.Language = "en"
	tt.Frame()
	if draft.content.TurnEnded != "Hello {agent}" || draft.agent != 1 {
		t.Fatal("language switch lost draft or assistant selection")
	}
}

func TestBothAssistantsFitMinimumWindowInBothLanguagesAndThemes(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+theme, func(t *testing.T) {
				a := newApp()
				a.snapshot.Preferences.Language = language
				a.snapshot.Preferences.Theme = theme
				a.connection.Ready = true
				a.claudeConnection.Ready = true
				a.snapshot.Preferences.Channels[model.Feishu] = true
				a.snapshot.LastEvents = map[model.Agent]time.Time{model.Codex: time.Now(), model.ClaudeCode: time.Now()}
				tt := ui.NewTester(a.view, 880, 620)
				for _, label := range []string{a.tr("Codex connected"), a.tr("Claude Code connected"), a.tr("Refresh {agent}", "Claude Code")} {
					r, ok := tt.Find(label)
					if !ok || r.X < 192 || r.X+r.W > 880 {
						t.Fatal("assistant status/control overflow", label, r)
					}
				}
				saveFeedbackPreview(t, tt, "assistants-"+language+"-"+theme)
				a.page = "settings"
				tt.Frame()
				if !tt.HasText("Claude Code") || !tt.HasText(a.tr("Codex")) {
					t.Fatal("assistant settings missing")
				}
			})
		}
	}
}
