package desktop

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func TestPrivateFormsAndSwitchingKeepDrafts(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, dark := range []bool{false, true} {
			a := newApp()
			a.page = "channels"
			a.snapshot.Preferences.Language = language
			tt := ui.NewTester(a.view, 1040, 760)
			tt.SetDark(dark)
			for _, ch := range model.Channels {
				a.chooseChannel(ch)
				tt.Frame()
				form := a.forms[ch]
				form.Webhook = "https://keep-group-draft.invalid"
				form.AppID = "keep-app-id"
				form.AppSecret = "keep-secret"
				form.RecipientID = "recipient@example.com"
				a.pairedApps[ch] = pairingApp(ch, *form)
				if err := tt.Click(a.tr("Personal DM")); err != nil {
					t.Fatal(ch, err)
				}
				if form.Mode != model.Direct || tt.HasText(a.tr("Webhook URL")) {
					t.Fatal("private mode still shows group webhook", ch)
				}
				want := "App Secret"
				if ch == model.DingTalk {
					want = "Client Secret (AppSecret)"
				}
				if ch == model.Slack {
					want = "Bot User OAuth Token"
				}
				if ch == model.Telegram {
					want = "Bot Token"
				}
				if !tt.HasText(a.tr(want)) {
					t.Fatal("private fields absent", ch)
				}
				if tt.HasText("keep-secret") {
					t.Fatal("secret rendered as plain text")
				}
				for _, hidden := range []string{"Robot Code", "Member ID", "Private Chat ID", "Recipient email", "Manual setup", "Show credentials", "Load saved credentials"} {
					if tt.HasText(a.tr(hidden)) {
						t.Fatal("removed configuration controls returned", ch, hidden)
					}
				}
				if !tt.HasText(a.tr("Pair again")) {
					t.Fatal("pairing entry absent", ch)
				}
				if dir := os.Getenv("NUDILO_TEST_ARTIFACTS"); dir != "" {
					theme := "light"
					if dark {
						theme = "dark"
					}
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(dir, language+"-"+theme+"-direct-"+string(ch)+".png"))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				if err := tt.Click(a.tr("Group / channel")); err != nil {
					t.Fatal(err)
				}
				if form.Mode != model.Group || form.Webhook != "https://keep-group-draft.invalid" {
					t.Fatal("group draft discarded")
				}
				if err := tt.Click(a.tr("Personal DM")); err != nil {
					t.Fatal(err)
				}
				if form.AppSecret != "keep-secret" || form.RecipientID != "recipient@example.com" {
					t.Fatal("private draft discarded")
				}
				tt.SetSize(880, 620)
				tt.Frame()
				if !tt.HasText(a.tr(want)) {
					t.Fatal("private form lost at minimum size")
				}
				tt.SetSize(1040, 760)
			}
		}
	}
}
