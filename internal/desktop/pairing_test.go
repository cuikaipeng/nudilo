package desktop

import (
	"context"
	"testing"

	"callbell/internal/channels"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func TestPairingCancellationAndStaleResultDoNotChangeDestination(t *testing.T) {
	a := newApp()
	a.chooseChannel(model.DingTalk)
	a.forms[a.channel].Mode = model.Direct
	a.forms[a.channel].RecipientID = "original"
	credentials := *a.forms[a.channel]
	ctx, cancel := context.WithCancel(context.Background())
	a.pairCancel = cancel
	id := a.pairAttempt
	a.chooseChannel(model.Feishu)
	if ctx.Err() == nil {
		t.Fatal("switching platform did not cancel")
	}
	a.finishPair(id, model.DingTalk, credentials, channels.PairResult{RecipientID: "wrong", RobotCode: "wrong"}, nil)
	if a.forms[model.DingTalk].RecipientID != "original" {
		t.Fatal("stale callback changed recipient")
	}
	a.chooseChannel(model.DingTalk)
	a.pairCancel = func() {}
	a.forms[model.DingTalk].AppID = "edited-app"
	a.finishPair(a.pairAttempt, model.DingTalk, credentials, channels.PairResult{RecipientID: "wrong"}, nil)
	if a.forms[model.DingTalk].RecipientID != "original" {
		t.Fatal("callback overwrote edited form")
	}
	credentials = *a.forms[model.DingTalk]
	a.pairCancel = func() {}
	a.finishPair(a.pairAttempt, model.DingTalk, credentials, channels.PairResult{RecipientID: "user", RobotCode: "robot"}, nil)
	if a.forms[model.DingTalk].RecipientID != "user" || a.forms[model.DingTalk].RobotCode != "robot" || a.pairCancel != nil {
		t.Fatal("valid pairing not applied")
	}
}

func TestPairingWaitingUIWithoutManualRecipientControls(t *testing.T) {
	for _, lang := range []string{"en", "zh-Hans"} {
		for _, ch := range model.Channels {
			a := newApp()
			a.snapshot.Preferences.Language = lang
			a.chooseChannel(ch)
			a.forms[ch].Mode = model.Direct
			tt := ui.NewTester(a.view, 880, 620)
			for _, label := range []string{"Manual setup", "Recipient email", "Open ID", "Robot Code", "Member ID", "Private Chat ID"} {
				if tt.HasText(a.tr(label)) {
					t.Fatal("manual recipient field returned", ch, label)
				}
			}
			a.pairReady = true
			a.pairCode = "nudilo-0123456789ab"
			ctx, cancel := context.WithCancel(context.Background())
			a.pairCancel = cancel
			tt.Frame()
			if !tt.HasText(a.pairCode) || !tt.HasText(a.tr("Waiting for your code…")) {
				t.Fatal("waiting state absent")
			}
			if err := tt.Click(a.tr("Cancel")); err != nil {
				t.Fatal(err)
			}
			if ctx.Err() == nil || a.pairCode != "" || a.pairCancel != nil {
				t.Fatal("cancel left active pairing")
			}
			ctx, cancel = context.WithCancel(context.Background())
			a.pairCancel = cancel
			if err := tt.Click(a.tr("Group / channel")); err != nil {
				t.Fatal(err)
			}
			if ctx.Err() == nil || a.forms[ch].Mode != model.Group {
				t.Fatal("mode change left active pairing")
			}
		}
	}
}
