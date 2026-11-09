package engine

import (
	"os"
	"time"

	"callbell/internal/locale"
	"callbell/internal/model"
)

func (e *Engine) resumeIfDue(now time.Time) error {
	e.mu.Lock()
	due := e.pref.Paused && !e.pref.PauseUntil.IsZero() && !now.Before(e.pref.PauseUntil)
	e.mu.Unlock()
	if !due {
		return nil
	}
	err := e.UpdatePreferences(func(p *model.Preferences) {
		if p.Paused && !p.PauseUntil.IsZero() && !now.Before(p.PauseUntil) {
			p.Resume(now)
		}
	})
	if err != nil {
		e.setError("Could not resume reminders. Check folder permissions and disk space.")
	} else {
		e.clearError("Could not resume reminders. Check folder permissions and disk space.")
	}
	return err
}

// CheckStorage only checks Nudilo's queue and a disposable private write probe.
// It does not consume events, inspect conversations or send a notification.
func (e *Engine) CheckStorage() error {
	if _, err := os.ReadDir(e.paths.Inbox()); err != nil {
		return locale.M("Cannot read the local event queue.")
	}
	for _, dir := range []string{e.paths.Root, e.paths.Inbox()} {
		if err := probeWrite(dir); err != nil {
			return err
		}
	}
	return nil
}

func probeWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".nudilo-check-*")
	if err != nil {
		return locale.M("Cannot write local notification data. Check folder permissions and disk space.")
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write([]byte("nudilo"))
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return locale.M("Cannot write local notification data. Check folder permissions and disk space.")
	}
	return nil
}
