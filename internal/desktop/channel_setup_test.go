package desktop

import (
	"context"
	"errors"
	"testing"

	"callbell/internal/channels"
	"callbell/internal/engine"
	"callbell/internal/model"
	"callbell/internal/storage"
	"github.com/egoist/mygo/ui"
)

func setupFixture(ch model.Channel, mode model.DeliveryMode) model.Credentials {
	c := model.Credentials{Mode: mode, AppID: "cli_example", AppSecret: "fixture-app-secret", RecipientID: "ou_example", RecipientType: "open_id"}
	switch ch {
	case model.Feishu:
		c.Webhook = "https://open.feishu.cn/open-apis/bot/v2/hook/fixture"
	case model.Lark:
		c.Webhook = "https://open.larksuite.com/open-apis/bot/v2/hook/fixture"
	case model.DingTalk:
		c.AppID, c.RobotCode, c.RecipientID = "ding_example", "robot_example", "user_example"
		c.Webhook = "https://oapi.dingtalk.com/robot/send?access_token=fixture"
	case model.Slack:
		c.AppID, c.AppSecret, c.Token, c.RecipientID = "", "", "xoxb-fixture-secret", "U12345678"
		c.Webhook = "https://hooks.slack.com/services/T000/B000/fixture"
	case model.Telegram:
		c.AppID, c.AppSecret = "", ""
		c.Token, c.RecipientID, c.ChatID = "123456789:abcdefghijklmnopqrstuvwxy0123456", "123456789", "-10012345678"
	}
	c.Normalize(ch)
	return c
}

type setupSender struct{ err error }

func (s *setupSender) Send(context.Context, model.Channel, model.Credentials, model.Event) error {
	return s.err
}

func appWithSavedSetup(t *testing.T, ch model.Channel, mode model.DeliveryMode) (*app, storage.Paths, *setupSender) {
	t.Helper()
	paths := storage.Paths{Root: t.TempDir()}
	sender := &setupSender{}
	e, err := engine.New(paths, storage.NewLocalVault(paths), sender)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.SaveChannel(ch, setupFixture(ch, mode)); err != nil {
		t.Fatal(err)
	}
	// Reopen the local store, as on the next app launch. No OS vault is involved.
	e, err = engine.New(paths, storage.NewLocalVault(paths), sender)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp()
	a.engine, a.snapshot = e, e.Snapshot()
	a.applyLocalChannels(readLocalChannels(e, a.snapshot.Preferences))
	for _, channel := range model.Channels {
		a.restoreChannelForm(channel)
	}
	a.chooseChannel(ch)
	return a, paths, sender
}

func TestSavedChannelSummaryAndPrefilledEditing(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, mode := range []model.DeliveryMode{model.Direct, model.Group} {
			for _, ch := range model.Channels {
				t.Run(language+"/"+string(mode)+"/"+string(ch), func(t *testing.T) {
					a, _, _ := appWithSavedSetup(t, ch, mode)
					a.snapshot.Preferences.Language = language
					before := a.savedChannels[ch]
					tt := ui.NewTester(a.view, 1040, 760)
					for _, dark := range []bool{false, true} {
						tt.SetDark(dark)
						if !tt.HasText(a.tr("Edit setup")) || !tt.HasText(a.tr("Configured")) || tt.HasText(a.tr("Save & test")) {
							t.Fatal("saved setup still shown as empty form")
						}
						for _, secret := range []string{before.AppSecret, before.Token, before.Webhook, before.RecipientID} {
							if secret != "" && tt.HasText(secret) {
								t.Fatal("summary exposed secret or recipient identifier")
							}
						}
						for _, removed := range []string{"Load saved credentials", "Manual setup", "Show credentials"} {
							if tt.HasText(a.tr(removed)) {
								t.Fatal("removed control returned", removed)
							}
						}
						appearance := "light"
						if dark {
							appearance = "dark"
						}
						saveFeedbackPreview(t, tt, language+"-"+appearance+"-saved-"+string(mode)+"-"+string(ch))
					}
					tt.SetSize(880, 620)
					if err := tt.Click(a.tr("Edit setup")); err != nil {
						t.Fatal(err)
					}
					if *a.forms[ch] != before || !a.editingChannels[ch] {
						t.Fatal("edit did not restore saved values")
					}
					if !tt.HasText(a.tr("Cancel editing")) || !tt.HasText(a.tr("Save & test")) {
						t.Fatal("editor actions missing")
					}
					if mode == model.Direct && !a.hasPairedAccount(ch) {
						t.Fatal("existing account binding lost")
					}
					for _, secret := range []string{before.AppSecret, before.Token, before.Webhook} {
						if secret != "" && tt.HasText(secret) {
							t.Fatal("editing exposed a secret")
						}
					}
					saveFeedbackPreview(t, tt, language+"-edit-"+string(mode)+"-"+string(ch))
					a.forms[ch].AppSecret = "unsaved-change"
					if err := tt.Click(a.tr("Cancel editing")); err != nil {
						t.Fatal(err)
					}
					stored, err := a.engine.Credentials(ch)
					if err != nil || stored != before || *a.forms[ch] != before || a.editingChannels[ch] {
						t.Fatal("cancel changed saved configuration")
					}
				})
			}
		}
	}
}

func TestChangedApplicationRequiresNewPairing(t *testing.T) {
	for _, ch := range model.Channels {
		t.Run(string(ch), func(t *testing.T) {
			a, _, _ := appWithSavedSetup(t, ch, model.Direct)
			a.editChannel()
			if ch == model.Telegram || ch == model.Slack {
				a.forms[ch].Token += "new"
			} else {
				a.forms[ch].AppID += "new"
			}
			if a.hasPairedAccount(ch) {
				t.Fatal("changed app kept old binding")
			}
			if draft := a.channelDraft(ch); draft.RecipientID != "" || channels.Validate(ch, draft) == nil {
				t.Fatal("old recipient accepted for new app")
			}
			tt := ui.NewTester(a.view, 880, 620)
			if err := tt.Click(a.tr("Save")); err != nil {
				t.Fatal(err)
			}
			if a.fieldError("pairing") == "" {
				t.Fatal("missing re-pair hint")
			}
			credentials := *a.forms[ch]
			a.pairCancel = func() {}
			a.finishPair(a.pairAttempt, ch, credentials, channels.PairResult{RecipientID: credentials.RecipientID, RecipientType: credentials.RecipientType, RobotCode: credentials.RobotCode}, nil)
			if !a.hasPairedAccount(ch) || channels.Validate(ch, a.channelDraft(ch)) != nil {
				t.Fatal("fresh binding not accepted")
			}
		})
	}
}

func TestSaveReturnsToSummaryEvenWhenTestFails(t *testing.T) {
	a, _, sender := appWithSavedSetup(t, model.Feishu, model.Direct)
	a.editChannel()
	submitted := *a.forms[a.channel]
	sender.err = errors.New("fixture send failed")
	result := saveChannelSetup(context.Background(), a.engine, a.channel, a.channelDraft(a.channel), true)
	if !result.saved || result.err == nil {
		t.Fatal("fixture must save successfully and fail its test")
	}
	a.finishChannelSave(a.channel, submitted, result)
	tt := ui.NewTester(a.view, 1040, 760)
	if !tt.HasText(a.tr("Edit setup")) || a.editingChannels[a.channel] {
		t.Fatal("test failure hid successfully saved setup")
	}
	if len(a.snapshot.Records) != 1 || a.snapshot.Records[0].State() != model.Failed {
		t.Fatal("test failure not recorded")
	}
}

func TestSaveFailureKeepsDraftAndEditingState(t *testing.T) {
	a, _, _ := appWithSavedSetup(t, model.Feishu, model.Direct)
	a.editChannel()
	a.forms[a.channel].AppSecret = ""
	submitted := *a.forms[a.channel]
	result := saveChannelSetup(context.Background(), a.engine, a.channel, a.channelDraft(a.channel), false)
	if result.saved || result.err == nil {
		t.Fatal("invalid draft saved")
	}
	a.finishChannelSave(a.channel, submitted, result)
	if !a.editingChannels[a.channel] || *a.forms[a.channel] != submitted {
		t.Fatal("failed save discarded the draft")
	}
	if a.savedChannels[a.channel].AppSecret != "fixture-app-secret" {
		t.Fatal("failed save changed persisted setup")
	}
}
