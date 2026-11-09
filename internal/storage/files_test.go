package storage

import (
	"os"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestOldPreferencesAdoptSystemAppearanceAndLanguage(t *testing.T) {
	p := Paths{Root: t.TempDir()}
	if err := os.WriteFile(p.Settings(), []byte(`{"paused":true,"channels":{"slack":true},"kinds":{"turn_ended":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	pref, err := p.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	if pref.Theme != "system" || pref.Language != "system" || !pref.Paused || !pref.Channels[model.Slack] {
		t.Fatalf("migration lost settings: %+v", pref)
	}
	pref.Language = "zh-Hans"
	pref.Theme = "dark"
	if err := WriteJSON(p.Settings(), pref); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.Preferences()
	if err != nil || loaded.Language != "zh-Hans" || loaded.Theme != "dark" {
		t.Fatalf("preferences did not persist: %+v %v", loaded, err)
	}
}

func TestLegacySettingsAdoptReminderDefaultsAndRoundTripRules(t *testing.T) {
	p := Paths{Root: t.TempDir()}
	if err := os.WriteFile(p.Settings(), []byte(`{"paused":false,"channels":{"feishu":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	pref, err := p.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	if len(pref.Routes) != 0 || pref.Paused || !pref.PauseUntil.IsZero() || pref.ChannelSince == nil {
		t.Fatal("legacy settings gained routing or pause state")
	}
	now := time.Now().Truncate(time.Second)
	pref.Pause(now, now.Add(time.Hour))
	pref.Routes = []model.ProjectRoute{{Project: "work", Agent: model.ClaudeCode, Channels: []model.Channel{model.Feishu}}}
	pref.ChannelSince[model.Feishu] = now
	if err := WriteJSON(p.Settings(), pref); err != nil {
		t.Fatal(err)
	}
	saved, err := p.Preferences()
	if err != nil || !saved.PauseUntil.Equal(pref.PauseUntil) || len(saved.Routes) != 1 || saved.Routes[0].Agent != model.ClaudeCode || !saved.ChannelSince[model.Feishu].Equal(now) {
		t.Fatal("reminder settings not preserved", err, saved)
	}
}
