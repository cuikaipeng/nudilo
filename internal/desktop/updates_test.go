package desktop

import (
	"testing"
	"time"

	"callbell/internal/updates"
	"github.com/egoist/mygo/ui"
)

func TestUpdateViewsBothLanguagesAndThemes(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, dark := range []bool{false, true} {
			a := newApp()
			a.page = "settings"
			a.updatesOpen = true
			a.snapshot.Preferences.Language = language
			tt := ui.NewTester(a.view, 880, 620)
			tt.SetDark(dark)
			tt.SetLocale(language)
			for _, phase := range []updates.Phase{updates.Checking, updates.Current, updates.Available, updates.Downloading, updates.Ready, updates.CheckFailed, updates.DownloadFailed, updates.NotPublished} {
				a.updateState = updates.State{Phase: phase, Version: "0.2.0", CanInstall: true, Bytes: 25, Total: 100, Checked: time.Date(2026, 11, 5, 14, 30, 0, 0, time.Local)}
				tt.Frame()
				if !tt.HasText(a.updateStatus()) {
					t.Fatal(language, phase, "missing update status")
				}
				for _, label := range []string{a.tr("Automatic updates"), a.updateStatus()} {
					r, ok := tt.Find(label)
					if !ok || r.X < 192 || r.X+r.W > 880 || r.Y+r.H > 620 {
						t.Fatal("update content clipped", language, phase, r)
					}
				}
			}
			a.updateState.Phase = updates.Ready
			tt.Frame()
			suffix := "light"
			if dark {
				suffix = "dark"
			}
			saveFeedbackPreview(t, tt, "updates-"+language+"-"+suffix)
			for i := 0; i < 24 && !tt.Focused(a.tr("Automatic updates")); i++ {
				tt.Key(0, ui.KeyTab)
			}
			if !tt.Focused(a.tr("Automatic updates")) {
				t.Fatal("automatic-update switch is not keyboard accessible")
			}
			tt.Key(0, ui.KeySpace)
			if a.snapshot.Preferences.AutomaticUpdates {
				t.Fatal("automatic preference did not turn off")
			}
		}
	}
}
func TestUpdateRestartWaitsForExplicitActionAndProtectsDrafts(t *testing.T) {
	old := restartForUpdate
	t.Cleanup(func() { restartForUpdate = old })
	restarts := 0
	restartForUpdate = func() { restarts++ }
	a := newApp()
	a.page = "settings"
	a.updatesOpen = true
	a.updateState = updates.State{Phase: updates.Ready, Version: "0.2.0", CanInstall: true}
	tt := ui.NewTester(a.view, 1040, 760)
	if restarts != 0 {
		t.Fatal("restarted automatically")
	}
	a.notificationDraft("en").content.Title = "unsaved"
	if err := tt.Click("Restart to update"); err != nil {
		t.Fatal(err)
	}
	if restarts != 0 || !a.updateRestartOpen {
		t.Fatal("discarded drafts without asking")
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	a.notificationDrafts = nil
	tt.Frame()
	if err := tt.Click("Restart to update"); err != nil {
		t.Fatal(err)
	}
	if restarts != 1 {
		t.Fatal("explicit restart was not performed")
	}
}
