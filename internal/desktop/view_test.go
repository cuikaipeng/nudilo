package desktop

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestNativeChannelNavigationAndValidation(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1120, 800)
	if !tt.HasText("Notifications on") {
		t.Fatal("overview not shown")
	}
	if err := tt.Click("Channels"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DingTalk", "Lark", "Telegram", "Slack", "Feishu"} {
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := tt.Click("Save & test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.fieldError("webhook"), "HTTPS") {
		t.Fatal("invalid credentials were not rejected", a.message)
	}
	if err := tt.Click("Telegram"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Bot Token") || !tt.HasText("Pair account") {
		t.Fatal("Telegram setup missing")
	}
	if err := tt.Click("Pair account"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.fieldError("token"), "Bot Token") || a.pairCode != "" {
		t.Fatal("pairing must validate credentials before showing a code")
	}
	tt.SetSize(960, 660)
	tt.SetDark(true)
	if !tt.HasText("Bot Token") {
		t.Fatal("small dark layout lost fields")
	}
}
