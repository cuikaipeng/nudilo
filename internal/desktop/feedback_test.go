package desktop

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func TestFeedbackExpiresAndDoesNotFollowNavigationOrLateWork(t *testing.T) {
	a := newApp()
	a.chooseChannel(model.Feishu)
	tt := ui.NewTester(a.view, 1040, 760)
	a.showFeedback(locale.M("Test sent."), false)
	tt.Frame()
	if !tt.HasText("Test sent.") {
		t.Fatal("test outcome absent")
	}
	tt.Advance(250 * time.Millisecond)
	saveFeedbackPreview(t, tt, "feedback-success")
	tt.Advance(4 * time.Second)
	if tt.HasText("Test sent.") || a.message.Key != "" {
		t.Fatal("success did not expire")
	}
	a.showFeedback(locale.M("Saved."), false)
	tt.Frame()
	scope := a.scope()
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Saved.") {
		t.Fatal("feedback followed navigation")
	}
	if err := tt.Click("Channels"); err != nil {
		t.Fatal(err)
	}
	a.finishWork(scope, locale.M("Network request failed. Check your connection and proxy settings."), "Saved.")
	tt.Frame()
	if a.message.Key != "" {
		t.Fatal("late async result survived navigation away and back")
	}
}

func TestValidationIsInlineAndClearedWhenLeavingForm(t *testing.T) {
	a := newApp()
	a.chooseChannel(model.Feishu)
	tt := ui.NewTester(a.view, 1040, 760)
	if err := tt.Click("Save & test"); err != nil {
		t.Fatal(err)
	}
	if a.message.Key != "" || a.fieldError("webhook") == "" {
		t.Fatal("validation became a global banner")
	}
	saveFeedbackPreview(t, tt, "feedback-validation")
	input, ok := tt.Find("Webhook URL")
	if !ok {
		t.Fatal("webhook field missing")
	}
	feedback, ok := tt.Find(a.fieldError("webhook"))
	if !ok || feedback.Y < input.Y+input.H {
		t.Fatal("validation is not below the field", input, feedback)
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if len(a.fieldErrors) != 0 {
		t.Fatal("validation followed navigation")
	}
	if err := tt.Click("Channels"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Paste the complete HTTPS bot URL.") {
		t.Fatal("old validation returned")
	}
}

func TestChannelStatusDoesNotHideMissingCredentialsBehindDestination(t *testing.T) {
	a := newApp()
	a.snapshot.Preferences.Channels[model.Feishu] = true
	a.snapshot.Preferences.ChannelModes = map[model.Channel]model.DeliveryMode{model.Feishu: model.Direct}
	a.localMissing = map[model.Channel]bool{model.Feishu: true}
	tt := ui.NewTester(a.view, 880, 620)
	if _, ok := tt.Find("Feishu · Needs setup"); !ok {
		t.Fatal("missing credentials hidden by saved destination")
	}
	a.localMissing[model.Feishu] = false
	tt.Frame()
	if _, ok := tt.Find("Feishu · Configured"); !ok {
		t.Fatal("configured channel not marked")
	}
	if tt.HasText("A little nudge from your AI.") {
		t.Fatal("slogan still in overview")
	}
	if err := tt.Click("Pause reminders"); err != nil {
		t.Fatal(err)
	}
	if !a.snapshot.Preferences.Paused || !tt.HasText("Notifications paused") {
		t.Fatal("pause control did not update")
	}
	tt.Advance(300 * time.Millisecond)
	saveFeedbackPreview(t, tt, "overview-paused-small")
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if err := tt.Click("Enable reminders"); err != nil {
		t.Fatal(err)
	}
	if a.snapshot.Preferences.Paused {
		t.Fatal("enable control did not update")
	}
}

func TestClearHistoryRequiresConfirmationAndCanCancel(t *testing.T) {
	a := newApp()
	a.page = "history"
	a.snapshot.Records = []model.Record{{Event: model.Event{ID: "keep", Kind: model.TurnEnded, CreatedAt: time.Now()}}}
	tt := ui.NewTester(a.view, 880, 620)
	if err := tt.Click("Clear all"); err != nil {
		t.Fatal(err)
	}
	if !a.clearHistoryOpen || !tt.HasText("Clear all activity?") {
		t.Fatal("confirmation absent")
	}
	saveFeedbackPreview(t, tt, "clear-history-confirmation")
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if a.clearHistoryOpen || len(a.snapshot.Records) != 1 {
		t.Fatal("cancel cleared history")
	}
	a.snapshot.Records = nil
	tt.Frame()
	if err := tt.Click("Clear all"); err != nil {
		t.Fatal(err)
	}
	if a.clearHistoryOpen {
		t.Fatal("empty history allowed clearing")
	}
}

func saveFeedbackPreview(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("NUDILO_TEST_ARTIFACTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
