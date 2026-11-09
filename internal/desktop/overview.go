package desktop

import (
	"time"

	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func (a *app) notificationControl(c *ui.Context) {
	p := a.palette(c)
	paused := a.snapshot.Preferences.Paused
	title, action, symbol := "Notifications on", "Pause reminders", iconPause
	target := float32(1)
	if paused {
		title, action, symbol, target = "Notifications paused", "Enable reminders", iconPlay, 0
	}
	row := ui.Row(c.Key("notification-control")).FillWidth().Padding(16).Gap(16).AlignItems(ui.Center).Radius(14)
	progress := row.Animate("enabled", target, 240*time.Millisecond)
	row.Background(p.control.Mix(p.selected, progress)).Border(1, p.border.Mix(ui.Hex(brandCoral).Alpha(.6), progress)).Children(func() {
		ui.Box(c).Size(40, 40).Shrink(0).Radius(20).Background(p.card.Alpha(.75)).Center().Children(func() {
			ui.Icon(c, iconBell).Size(26, 26).TextColor(p.muted.Mix(p.accentInk, progress)).Rotate(-12*(1-progress)).Scale(.88+.12*progress, .88+.12*progress)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, a.tr(title)).FontSize(fontSize(c, 19)).FontWeight(650).Wrap()
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				label, state := a.overallStatus()
				color := p.muted
				if state == "warning" {
					color = c.Theme().Warning
				}
				dot := color
				if state == "success" {
					dot = c.Theme().Success
				}
				ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(dot)
				text := a.tr(label)
				if paused && !a.snapshot.Preferences.PauseUntil.IsZero() {
					text = a.tr("Resumes {time}", a.date(a.snapshot.Preferences.PauseUntil))
				}
				ui.Text(c, text).FontSize(fontSize(c, 13)).TextColor(color).Wrap().Role(ui.RoleStatus)
			})
		})
		ui.Row(c.Key("reminder-actions")).Gap(0).Shrink(0).AlignItems(ui.Stretch).Background(c.Theme().Accent).Radius(24).Children(func() {
			ui.PrimaryButton(c.Key("pause"), "").Label(a.tr(action)).Padding(12, 16).Radius(24, 0, 0, 24).Disabled(a.busy).
				OnClick(a.togglePause).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, symbol).Size(16, 16).ContentTransition(ui.ContentTransition{})
					ui.Text(c, a.tr(action)).FontWeight(600)
				})
			})
			ui.Box(c).Width(1).Margin(10, 0).Background(c.Theme().AccentText.Alpha(.18))
			a.pauseMenu(c)
		})
	})
}

func (a *app) channelStatus(ch model.Channel) string {
	if a.localMissing[ch] {
		return "Needs setup"
	}
	if a.snapshot.Preferences.Channels[ch] {
		return "Configured"
	}
	return "Not configured"
}

func (a *app) channelCard(c *ui.Context, ch model.Channel) {
	p := a.palette(c)
	status := a.channelStatus(ch)
	color, symbol := p.muted, iconCircle
	if status == "Configured" {
		color, symbol = c.Theme().Success, iconCheck
	}
	if status == "Needs setup" {
		color, symbol = c.Theme().Warning, iconAlert
	}
	button := ui.Button(c.Key("quick-"+string(ch)), "").Label(ch.Name(a.locale())+" · "+a.tr(status)).
		Grow(1).Basis(110).MinWidth(100).AlignItems(ui.Start).Padding(12, 6).Radius(12).OnClick(func() { a.chooseChannel(ch) }).Children(func() {
		ui.Column(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(38, 38).Center().Children(func() {
				ui.Image(c, channelLogos[ch]).Size(32, 32).TextColor(ui.Hex("#0089FF"))
			})
			ui.Text(c, ch.Name(a.locale())).FontWeight(600).FontSize(fontSize(c, 13))
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, symbol).Size(12, 12).TextColor(color)
				ui.Text(c, a.tr(status)).FontSize(fontSize(c, 13)).TextColor(color).Wrap()
			})
		})
	})
	if a.hasSavedChannel(ch) {
		button.Tooltip(a.deliveryStatus(ch))
	}
}
