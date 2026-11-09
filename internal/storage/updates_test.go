package storage

import (
	"testing"
)

func TestUpdatePreferenceMigrationAndOptOut(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	if err := WriteJSON(paths.Settings(), map[string]any{"paused": true}); err != nil {
		t.Fatal(err)
	}
	p, err := paths.Preferences()
	if err != nil || !p.AutomaticUpdates || !p.Paused {
		t.Fatal(p, err)
	}
	p.AutomaticUpdates = false
	if err := WriteJSON(paths.Settings(), p); err != nil {
		t.Fatal(err)
	}
	reloaded, err := paths.Preferences()
	if err != nil || reloaded.AutomaticUpdates {
		t.Fatal("opt-out lost on reload", err)
	}
}
