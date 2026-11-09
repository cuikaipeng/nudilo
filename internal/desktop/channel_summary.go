package desktop

import (
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func (a *app) channelHeading(c *ui.Context, ch model.Channel, editing bool) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Image(c, channelLogos[ch]).Size(28, 28).TextColor(ui.Hex("#0089FF"))
		ui.Text(c, ch.Name(a.locale())).FontSize(fontSize(c, 18)).FontWeight(650)
		ui.Spacer(c)
		switch {
		case editing:
			ui.Badge(c, a.tr("Editing setup")).TextColor(a.palette(c).accentInk)
		case a.hasSavedChannel(ch):
			ui.Badge(c, a.tr("Configured")).TextColor(c.Theme().Success).Background(c.Theme().Success.Alpha(.09))
		case a.snapshot.Preferences.Channels[ch]:
			ui.Badge(c, a.tr("Needs setup")).TextColor(c.Theme().Warning)
		}
	})
	if a.localMissing[ch] {
		ui.Text(c, a.tr("The saved setup is unavailable. Check your local configuration.")).TextColor(c.Theme().Warning).Wrap()
	}
}

func (a *app) savedChannelView(c *ui.Context, ch model.Channel, credentials model.Credentials) {
	p := a.palette(c)
	card(c, func() {
		a.channelHeading(c, ch, false)
		ui.Column(c).Gap(10).Padding(14).Radius(10).Background(p.selected.Alpha(.45)).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconCheck).Size(18, 18).TextColor(c.Theme().Success)
				ui.Text(c, a.tr("Destination")).TextColor(p.muted)
				ui.Text(c, ch.Name(a.locale())+" · "+credentials.Mode.Name(a.locale())).Grow(1).MinWidth(0).Wrap().FontWeight(600)
			})
			if credentials.Mode == model.Direct {
				ui.Text(c, a.tr("Account paired")).TextColor(c.Theme().Success)
			}
			a.channelDeliveryView(c, ch)
		})
		ui.Collapsible(c.Key("channel-details"), a.tr("Configuration details"), &a.channelDetails, func() {
			ui.Column(c).Gap(8).Padding(8, 0).Children(func() {
				if ch == model.Telegram {
					a.savedSetupRow(c, "Bot Token", a.tr("Saved"), true)
					if credentials.Mode == model.Group {
						a.savedSetupRow(c, "Group / channel ID", credentials.ChatID, false)
					}
				} else if credentials.Mode == model.Group {
					a.savedSetupRow(c, "Webhook URL", a.tr("Saved"), true)
					if credentials.Secret != "" {
						a.savedSetupRow(c, "Signing", a.tr("Enabled"), true)
					}
				} else if ch == model.Slack {
					a.savedSetupRow(c, "Bot User OAuth Token", a.tr("Saved"), true)
				} else {
					id, secret := "App ID", "App Secret"
					if ch == model.DingTalk {
						id, secret = "Client ID (AppKey)", "Client Secret (AppSecret)"
					}
					a.savedSetupRow(c, id, credentials.AppID, false)
					a.savedSetupRow(c, secret, a.tr("Saved"), true)
				}
			})
		})
		ui.Divider(c)
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Button(c.Key("remove-channel"), a.tr("Remove channel")).TextColor(c.Theme().Danger).
				Disabled(a.busy).OnClick(func() { a.removeChannelOpen = true })
			ui.Spacer(c)
			ui.Button(c.Key("edit-channel"), a.tr("Edit setup")).Disabled(a.busy).OnClick(a.editChannel)
			ui.PrimaryButton(c.Key("test-channel"), a.tr("Send test")).Disabled(a.busy).OnClick(a.testSavedChannel)
		})
	})
}
func (a *app) savedSetupRow(c *ui.Context, label, value string, saved bool) {
	ui.Row(c).FillWidth().Gap(18).MinHeight(30).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.tr(label)).Width(180).Shrink(0).TextColor(c.Theme().TextMuted).Wrap()
		ui.Row(c).Grow(1).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
			if saved {
				ui.Icon(c, iconCheck).Size(14, 14).TextColor(c.Theme().Success)
			}
			ui.Text(c, value).MinWidth(0).Wrap()
		})
	})
}
