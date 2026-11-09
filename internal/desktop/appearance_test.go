package desktop

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"callbell/internal/codex"
	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func TestLanguageAndAppearanceSwitchWithoutLosingForm(t *testing.T) {
	a := newApp()
	a.page = "settings"
	a.snapshot.Preferences.Language = "en"
	a.forms[model.Slack].Webhook = "https://hooks.slack.com/services/keep-this-draft"
	a.showFeedback(locale.M("Settings saved."), false)
	tt := ui.NewTester(a.view, 1040, 760)
	scrollSettings(t, tt, a, "Appearance")
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.Theme != "dark" {
		t.Fatal("appearance choice was not applied")
	}
	if err := tt.Click("English"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("简体中文"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.Language != "zh-Hans" || !tt.HasText("外观") || !tt.HasText("设置已保存。") {
		t.Fatal("UI and existing feedback did not switch language", tt.Texts())
	}
	if !strings.Contains(a.forms[model.Slack].Webhook, "keep-this-draft") {
		t.Fatal("language switch discarded unsaved form")
	}
	if err := tt.Click("浅色"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.Theme != "light" {
		t.Fatal("light mode was not applied")
	}
}

func TestConnectionWaitsForANewEvent(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.Language = "en"
	a.connection.Ready = true
	a.monitorSince = time.Now()
	a.snapshot.LastEvent = a.monitorSince.Add(-time.Hour)
	title, _ := a.connectionStatus()
	if title != "Listening for Codex" {
		t.Fatal("old history incorrectly confirms this connection", title)
	}
	a.snapshot.LastEvent = a.monitorSince.Add(time.Second)
	title, _ = a.connectionStatus()
	if title != "Codex connected" {
		t.Fatal("fresh event did not confirm connection", title)
	}
	a.monitorError = locale.M("Existing-chat monitoring failed. Check folder access and free disk space.")
	title, _ = a.connectionStatus()
	if title != "Codex needs attention" {
		t.Fatal("monitor failure hidden", title)
	}
}

func TestAllViewsInBothLanguagesAndAppearances(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, dark := range []bool{false, true} {
			theme := "light"
			if dark {
				theme = "dark"
			}
			t.Run(language+"-"+theme, func(t *testing.T) {
				a := newApp()
				a.snapshot.Preferences.Language = language
				a.snapshot.Preferences.Theme = "system"
				a.connection = codex.Connection{Ready: true}
				a.claudeConnection.Ready = true
				a.snapshot.LastEvents = map[model.Agent]time.Time{model.Codex: time.Date(2026, 11, 5, 14, 30, 0, 0, time.Local), model.ClaudeCode: time.Date(2026, 11, 5, 14, 31, 0, 0, time.Local)}
				a.snapshot.Preferences.Channels[model.Feishu] = true
				a.snapshot.Preferences.Channels[model.Lark] = true
				a.localMissing = map[model.Channel]bool{model.Lark: true}
				a.savedChannels[model.Feishu] = setupFixture(model.Feishu, model.Direct)
				a.restoreChannelForm(model.Feishu)
				if data, err := os.ReadFile("../../resources/logo.png"); err == nil {
					a.logo, _ = ui.DecodeBitmap(data)
				}
				a.snapshot.Records = []model.Record{{Event: model.Event{ID: "example", Kind: model.TurnEnded, Project: "sample-project", CreatedAt: time.Date(2026, 11, 4, 10, 42, 0, 0, time.Local)}, Deliveries: map[model.Channel]model.Delivery{model.Feishu: {State: model.Sent}}}}
				tt := ui.NewTester(a.view, 1040, 760)
				tt.SetDark(dark)
				tt.SetLocale(language)
				for _, page := range pages {
					a.page = page.id
					tt.Frame()
					if !tt.HasText(a.tr(page.title)) {
						t.Fatal("missing page", page.id)
					}
					if dir := os.Getenv("NUDILO_TEST_ARTIFACTS"); dir != "" {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						f, err := os.Create(filepath.Join(dir, language+"-"+theme+"-"+page.id+".png"))
						if err != nil {
							t.Fatal(err)
						}
						err = png.Encode(f, tt.Image())
						f.Close()
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				a.page = "channels"
				a.channel = model.Telegram
				tt.SetSize(880, 620)
				tt.Frame()
				if err := tt.Click(a.tr("Pair account")); err != nil {
					t.Fatal(err)
				}
				if !tt.HasText(a.tr("Enter the complete Bot Token from BotFather.")) {
					t.Fatal("pairing inaccessible at minimum window size")
				}
			})
		}
	}
}
