package desktop

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"callbell/internal/locale"
	"callbell/internal/model"
	"callbell/internal/updates"
	"github.com/egoist/mygo/ui"
)

func scrollSettings(t *testing.T, tt *ui.Tester, a *app, label string) {
	t.Helper()
	tt.Scroll(600, 300, 0, -10000)
	tt.Advance(300 * time.Millisecond)
	for i := 0; i < 60; i++ {
		if r, ok := tt.Find(a.tr(label)); ok {
			tt.Scroll(600, 300, 0, r.Y-64)
			tt.Advance(300 * time.Millisecond)
			return
		}
		tt.Scroll(600, 300, 0, 250)
		tt.Advance(300 * time.Millisecond)
	}
	t.Fatalf("settings section %s cannot be reached by scrolling", label)
}
func TestSnoozeMenuAndResumeDeadline(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.SetupDismissed = true
	tt := ui.NewTester(a.view, 1040, 760)
	if err := tt.Click("Pause duration"); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := tt.Click("For 15 minutes"); err != nil {
		t.Fatal(err)
	}
	p := a.snapshot.Preferences
	if !p.Paused || p.PauseUntil.Before(before.Add(15*time.Minute)) || !strings.Contains(strings.Join(tt.Texts(), " "), "Resumes") {
		t.Fatal("timed pause not visible/persisted", p)
	}
	if err := tt.Click("Enable reminders"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.Paused || a.snapshot.Preferences.PauseUntil.After(time.Now()) {
		t.Fatal("manual resume retained future pause")
	}
}
func TestGuideAndOverallStatusNeedRealConfigurationAndEvidence(t *testing.T) {
	a := newApp()
	a.connection.Ready = true
	a.snapshot.Preferences.Channels[model.Feishu] = true
	if _, configured, _ := a.setupSteps(); configured {
		t.Fatal("saved preference alone counted as usable credentials")
	}
	a.savedChannels[model.Feishu] = setupFixture(model.Feishu, model.Direct)
	if _, _, tested := a.setupSteps(); tested {
		t.Fatal("configuration falsely passed send test")
	}
	a.snapshot.Records = []model.Record{{Event: model.Event{Kind: model.Test, CreatedAt: time.Now()}, Deliveries: map[model.Channel]model.Delivery{model.Feishu: {State: model.Sent, Attempts: 1}}}}
	if _, _, tested := a.setupSteps(); !tested {
		t.Fatal("successful test not recognized")
	}
	if !a.channelHealth(model.Feishu).LastRealSuccess.IsZero() {
		t.Fatal("test treated as a real event")
	}
	a.snapshot.Preferences.Channels[model.Lark] = true
	a.localMissing = map[model.Channel]bool{model.Lark: true}
	if key, _ := a.overallStatus(); key != "Running · some channels need attention" {
		t.Fatal("one broken channel hid working channel", key)
	}
	if a.assistantVisible(model.ClaudeCode) {
		t.Fatal("uninstalled unused assistant shown by default")
	}
}
func TestHistoryFiltersAndCollapsibleDetails(t *testing.T) {
	a := newApp()
	a.page = "history"
	now := time.Now()
	a.snapshot.Records = []model.Record{
		{Event: model.Event{ID: "a", Project: "alpha", Agent: model.Codex, Kind: model.TurnEnded, CreatedAt: now}, Deliveries: map[model.Channel]model.Delivery{model.Feishu: {State: model.Sent}}},
		{Event: model.Event{ID: "b", Project: "beta", Agent: model.ClaudeCode, Kind: model.InputRequired, CreatedAt: now}, Deliveries: map[model.Channel]model.Delivery{model.Slack: {State: model.Failed, Error: "offline"}}},
	}
	tt := ui.NewTester(a.view, 1040, 760)
	if err := tt.Click("Search projects"); err != nil {
		t.Fatal(err)
	}
	tt.Type("beta")
	if got := a.filteredHistory(); len(got) != 1 || got[0].Event.ID != "b" {
		t.Fatal("project search incorrect", got)
	}
	if err := tt.Click("Delivery details"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Slack · Failed · offline") {
		t.Fatal("failure details missing")
	}
	a.historyQuery = ""
	tt.Frame()
	if err := tt.Click("Assistant filter"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if a.historyAgent != model.Codex {
		t.Fatal("assistant filter is not keyboard operable", a.historyAgent)
	}
	if err := tt.Click("Status filter"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if a.historyState != "failed" || len(a.filteredHistory()) != 0 || !tt.HasText("No matching notifications") {
		t.Fatal("filters not combined")
	}
	if err := tt.Click("Clear filters"); err != nil {
		t.Fatal(err)
	}
	if len(a.filteredHistory()) != 2 {
		t.Fatal("clearing filters lost history")
	}
}
func TestRouteEditorValidatesAndSavesExplicitDestination(t *testing.T) {
	a := newApp()
	a.page = "settings"
	a.routesOpen = true
	a.settingsTarget = "routing"
	a.snapshot.Preferences.Channels[model.Feishu] = true
	a.savedChannels[model.Feishu] = setupFixture(model.Feishu, model.Direct)
	a.editRoute(-1)
	tt := ui.NewTester(a.view, 1040, 900)
	if err := tt.Click("Project name"); err != nil {
		t.Fatal(err)
	}
	tt.Type("work")
	if err := tt.Click("Save rule"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Choose at least one notification channel.") {
		t.Fatal("empty destination accepted")
	}
	if err := tt.Click("Feishu"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Save rule"); err != nil {
		t.Fatal(err)
	}
	if a.routeDraft != nil || len(a.snapshot.Preferences.Routes) != 1 || a.snapshot.Preferences.Routes[0].Channels[0] != model.Feishu {
		t.Fatal("rule not saved", a.routeDraft, a.snapshot.Preferences.Routes, a.routeError)
	}
}

func TestDiagnosticsContinueAfterFailureAndOnlyCheckConfiguredChannels(t *testing.T) {
	p := model.Defaults()
	p.Channels[model.Feishu], p.Channels[model.Slack] = true, true
	checks := []model.Channel{}
	results := collectDiagnostics(p, func() error { return fmt.Errorf("fixture hook missing") }, nil,
		func() error { return nil }, func(ch model.Channel) (model.Credentials, error) {
			checks = append(checks, ch)
			if ch == model.Slack {
				return model.Credentials{}, fmt.Errorf("fixture credentials missing")
			}
			return setupFixture(ch, model.Direct), nil
		})
	if len(results) != 4 || results[0].ok || !results[1].ok || !results[2].ok || results[3].ok || len(checks) != 2 {
		t.Fatal("diagnosis stopped early or reported a false success", results, checks)
	}
}

func TestProjectRuleDraftProtectsUpdateRestart(t *testing.T) {
	old := restartForUpdate
	t.Cleanup(func() { restartForUpdate = old })
	restarted := false
	restartForUpdate = func() { restarted = true }
	a := newApp()
	a.updateState.Phase = updates.Ready
	a.editRoute(-1)
	a.routeDraft.Project = "unsaved"
	a.restartUpdate()
	if restarted || !a.updateRestartOpen {
		t.Fatal("update discarded unsaved rule")
	}
}

func TestSmallWindowTextScalingAndProductViews(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, dark := range []bool{false, true} {
			for _, scale := range []float32{1, 1.5} {
				t.Run(fmt.Sprintf("%s-dark%v-scale%.1f", language, dark, scale), func(t *testing.T) {
					a := productFixture()
					a.snapshot.Preferences.Language = language
					tt := ui.NewTester(a.view, 880, 620)
					tt.SetDark(dark)
					tt.SetLocale(language)
					tt.SetPreferences(ui.Preferences{TextScale: scale, HighContrast: scale > 1, ReduceMotion: true})
					for _, page := range pages {
						a.page = page.id
						tt.Frame()
						for _, text := range tt.Texts() {
							r, _ := tt.Find(text)
							if r.Y < 28 || r.Y > 608 || r.W == 0 {
								continue
							}
							if r.X < 0 || r.X+r.W > 880.5 {
								t.Fatalf("%s text clipped horizontally: %q %v", page.id, text, r)
							}
						}
						saveFeedbackPreview(t, tt, fmt.Sprintf("product-%s-%v-%.1f-%s", language, dark, scale, page.id))
					}
					scrollSettings(t, tt, a, "Appearance")
					if err := tt.Click(a.tr("Dark")); err != nil {
						t.Fatal(err)
					}
					if a.snapshot.Preferences.Theme != "dark" {
						t.Fatal("scaled appearance controls inaccessible")
					}
					a.page = "overview"
					tt.Frame()
					if err := tt.Click(a.tr("Pause reminders")); err != nil {
						t.Fatal(err)
					}
					if !a.snapshot.Preferences.Paused {
						t.Fatal("scaled pause control inaccessible")
					}
				})
			}
		}
	}
}

func productFixture() *app {
	a := newApp()
	a.connection.Ready = true
	a.snapshot.Preferences.Channels[model.Feishu] = true
	a.savedChannels[model.Feishu] = setupFixture(model.Feishu, model.Direct)
	a.restoreChannelForm(model.Feishu)
	if data, err := os.ReadFile("../../resources/logo.png"); err == nil {
		a.logo, _ = ui.DecodeBitmap(data)
	}
	now := time.Now().Add(-5 * time.Minute)
	a.snapshot.Records = []model.Record{
		{Event: model.Event{ID: "sample-turn", Kind: model.TurnEnded, Agent: model.Codex, Project: "nudilo", CreatedAt: now}, Deliveries: map[model.Channel]model.Delivery{model.Feishu: {State: model.Sent, Attempts: 1, LastAttempt: now.Add(time.Second)}}},
		{Event: model.Event{ID: "sample-input", Kind: model.InputRequired, Agent: model.Codex, Project: "nudilo", CreatedAt: now.Add(-12 * time.Minute)}, Deliveries: map[model.Channel]model.Delivery{model.Feishu: {State: model.Sent, Attempts: 1}}},
	}
	a.snapshot.LastEvents = map[model.Agent]time.Time{model.Codex: now}
	return a
}

func TestProductGuideAndHealthPreviews(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		a := productFixture()
		a.snapshot.Preferences.Language = language
		tt := ui.NewTester(a.view, 1040, 760)
		tt.SetLocale(language)
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			saveFeedbackPreview(t, tt, fmt.Sprintf("ready-%s-%v", language, dark))
		}
		a.setupRequested = true
		tt.Frame()
		if !tt.HasText(a.tr("Get ready in three steps")) {
			t.Fatal("completed setup guide cannot be reopened")
		}
		if err := tt.Click(a.tr("Later")); err != nil {
			t.Fatal(err)
		}
		if tt.HasText(a.tr("Get ready in three steps")) {
			t.Fatal("guide cannot be dismissed")
		}
		a = newApp()
		a.snapshot.Preferences.Language = language
		tt = ui.NewTester(a.view, 880, 620)
		tt.SetLocale(language)
		saveFeedbackPreview(t, tt, "first-use-"+language)
		if err := tt.Click(a.tr("Set up")); err != nil {
			t.Fatal(err)
		}
		if a.page != "channels" {
			t.Fatal("guide does not lead to channel setup")
		}
	}
}

func TestCompletedSetupDoesNotReturnWhenHistoryIsCleared(t *testing.T) {
	a := productFixture()
	tt := ui.NewTester(a.view, 1040, 760)
	if !a.snapshot.Preferences.SetupDismissed {
		t.Fatal("setup completion was not remembered")
	}
	a.snapshot.Records = nil
	tt.Frame()
	if tt.HasText("Get ready in three steps") {
		t.Fatal("clearing history reset onboarding")
	}
}

func TestDiagnosticAndRoutingPanelsCanBeReached(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		a := productFixture()
		a.snapshot.Preferences.Language = language
		a.diagnosisOpen, a.diagnosisReveal = true, true
		a.diagnosis = []diagnosticResult{
			{title: locale.M("Codex"), detail: locale.M("Event listeners are ready."), ok: true},
			{title: locale.M("Local event queue"), detail: locale.M("Cannot read the local event queue.")},
			{title: locale.M("Feishu"), detail: locale.M("Saved setup is valid. Use Send test to check delivery."), ok: true},
		}
		tt := ui.NewTester(a.view, 880, 620)
		tt.SetLocale(language)
		saveFeedbackPreview(t, tt, "diagnostics-"+language)
		if err := tt.Click(a.tr("AI assistant settings")); err != nil {
			t.Fatal(err)
		}
		if a.page != "settings" {
			t.Fatal("diagnostics did not lead to assistant settings")
		}
		scrollSettings(t, tt, a, "Project routing")
		if err := tt.Click(a.tr("Choose channels by project")); err != nil {
			t.Fatal(err)
		}
		tt.Advance(300 * time.Millisecond)
		for i := 0; i < 45 && !tt.Focused(a.tr("Add rule")); i++ {
			tt.Key(0, ui.KeyTab)
		}
		if !tt.Focused(a.tr("Add rule")) {
			t.Fatal("Add rule cannot receive keyboard focus")
		}
		tt.Key(0, ui.KeyEnter)
		if a.routeDraft == nil {
			t.Fatal("Add rule did not open the editor")
		}
		a.routeDraft.Project = "nudilo"
		a.routeDraft.Channels = []model.Channel{model.Feishu}
		a.settingsTarget = "routing"
		tt.Frame()
		saveFeedbackPreview(t, tt, "routing-"+language)
		// The form may extend below a small window; keyboard focus reveals Save.
		for i := 0; i < 45 && !tt.Focused(a.tr("Save rule")); i++ {
			tt.Key(0, ui.KeyTab)
		}
		if !tt.Focused(a.tr("Save rule")) {
			t.Fatal("rule form cannot be saved by keyboard")
		}
		tt.Key(0, ui.KeyEnter)
		if a.routeDraft != nil || len(a.snapshot.Preferences.Routes) != 1 {
			t.Fatal("rule not saved", language, a.routeDraft, a.snapshot.Preferences.Routes, a.routeError)
		}
	}
}

func TestManualRetryShowsWaitingWithoutAnInventedDeadline(t *testing.T) {
	a := productFixture()
	a.snapshot.Records[0].Deliveries[model.Feishu] = model.Delivery{State: model.Pending, Error: "offline"}
	if got := a.deliveryStatus(model.Feishu); got != "Waiting to send" {
		t.Fatal("manual retry displayed a zero deadline", got)
	}
}
