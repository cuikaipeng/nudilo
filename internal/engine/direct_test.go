package engine

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestPrivateDestinationSwitchPreservesGroupAndDoesNotForwardOldQueue(t *testing.T) {
	e, p, v, s := harness(t)
	group := model.Credentials{Webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/private-fixture", Secret: "group-secret"}
	if err := e.SaveChannel(model.Feishu, group); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, p, "old-target", now)
	if err := e.ingest(now); err != nil {
		t.Fatal(err)
	}
	direct := model.Credentials{Mode: model.Direct, AppID: "cli_fixture", AppSecret: "private-app-secret", RecipientID: "recipient@example.com", RecipientType: "email"}
	if err := e.SaveChannel(model.Feishu, direct); err != nil {
		t.Fatal(err)
	}
	saved, _ := v.Get(model.Feishu)
	if saved.Mode != model.Direct || saved.Webhook != group.Webhook || saved.Secret != group.Secret {
		t.Fatal("group configuration discarded")
	}
	if e.Snapshot().Preferences.ChannelModes[model.Feishu] != model.Direct {
		t.Fatal("saved mode not reflected")
	}
	e.Step(context.Background(), now)
	if len(s.events) != 0 || e.Snapshot().Records[0].State() != model.Skipped {
		t.Fatal("old notification forwarded to new recipient")
	}
	queue(t, p, "new-target", now.Add(time.Second))
	e.Step(context.Background(), now.Add(time.Second))
	if len(s.events) != 1 {
		t.Fatal("new destination inactive")
	}
	if err := e.SaveChannel(model.Feishu, group); err != nil {
		t.Fatal(err)
	}
	saved, _ = v.Get(model.Feishu)
	if saved.Mode != model.Group || saved.AppSecret != direct.AppSecret || saved.RecipientID != direct.RecipientID {
		t.Fatal("private setup discarded")
	}
	for _, file := range []string{p.Settings(), p.History()} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{direct.AppSecret, direct.RecipientID, group.Secret, group.Webhook} {
			if strings.Contains(string(data), secret) {
				t.Fatal("credential persisted outside vault")
			}
		}
	}
	e2, err := New(p, v, s)
	if err != nil {
		t.Fatal(err)
	}
	if e2.Snapshot().Preferences.ChannelModes[model.Feishu] != model.Group {
		t.Fatal("mode lost on restart")
	}
}

func TestGroupSaveDoesNotRestoreBindingForAnotherApplication(t *testing.T) {
	for _, ch := range []model.Channel{model.Feishu, model.Telegram} {
		t.Run(string(ch), func(t *testing.T) {
			e, _, vault, _ := harness(t)
			old := model.Credentials{Mode: model.Direct, AppID: "cli_old", AppSecret: "secret", RecipientType: "open_id", RecipientID: "ou_old"}
			if ch == model.Telegram {
				old = model.Credentials{Mode: model.Direct, Token: "123:abcdefghijklmnopqrstuvwxyz", RecipientID: "123456"}
			}
			if err := e.SaveChannel(ch, old); err != nil {
				t.Fatal(err)
			}
			updated := old
			updated.Mode = model.Group
			updated.RecipientID, updated.RecipientType = "", ""
			if ch == model.Telegram {
				updated.Token, updated.ChatID = "987:abcdefghijklmnopqrstuvwxyz", "-100123"
			} else {
				updated.AppID, updated.Webhook = "cli_new", "https://open.feishu.cn/open-apis/bot/v2/hook/new"
			}
			if err := e.SaveChannel(ch, updated); err != nil {
				t.Fatal(err)
			}
			saved, err := vault.Get(ch)
			if err != nil || saved.RecipientID != "" {
				t.Fatal("old account restored for new application")
			}
		})
	}
}
