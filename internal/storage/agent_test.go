package storage

import (
	"os"
	"testing"

	"callbell/internal/model"
)

func TestLegacyPreferencesKeepCodexAndContentWhileAddingClaude(t *testing.T) {
	p := Paths{Root: t.TempDir()}
	data := []byte(`{"auto_connect":false,"language":"zh-Hans","channels":{"feishu":true},"kinds":{"turn_ended":false,"input_required":true,"approval_required":true},"message_content":{"zh-Hans":{"title":"我的提醒","turn_ended":"原有 Codex 文案"}}}`)
	if err := os.WriteFile(p.Settings(), data, 0600); err != nil {
		t.Fatal(err)
	}
	pref, err := p.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	if pref.AutoConnect || !pref.ClaudeAutoConnect || !pref.Kinds[model.TurnFailed] || pref.Kinds[model.TurnEnded] || !pref.Channels[model.Feishu] || pref.MessageContent["zh-Hans"].TurnEnded != "原有 Codex 文案" {
		t.Fatal("migration changed saved choices", pref)
	}
	pref.ClaudeAutoConnect = false
	pref.Kinds[model.TurnFailed] = false
	if err = WriteJSON(p.Settings(), pref); err != nil {
		t.Fatal(err)
	}
	pref, err = p.Preferences()
	if err != nil || pref.ClaudeAutoConnect || pref.Kinds[model.TurnFailed] {
		t.Fatal("explicit disabled choice lost", pref, err)
	}
}
