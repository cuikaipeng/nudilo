package desktop

import (
	"context"
	_ "embed"
	"time"

	"callbell/internal/channels"
	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed slack-manifest.json
var slackManifest string

func (a *app) cancelPair() {
	a.pairAttempt++
	if a.pairCancel != nil {
		a.pairCancel()
	}
	a.pairCancel = nil
	a.pairCode, a.pairURL, a.pairReady = "", "", false
}

func (a *app) startPair() {
	if a.busy || a.pairCancel != nil {
		return
	}
	ch, credentials := a.channel, *a.forms[a.channel]
	credentials.Normalize(ch)
	if err := channels.ValidatePairCredentials(ch, credentials); err != nil {
		a.formError(err)
		return
	}
	code, err := channels.NewPairCode()
	if err != nil {
		a.showFeedback(locale.FromError(err), true)
		return
	}
	a.cancelPair()
	ctx, cancel := context.WithCancel(a.ctx)
	a.pairCancel, a.pairCode = cancel, code
	a.resetFeedback()
	id := a.pairAttempt
	go func() {
		defer cancel()
		result, err := a.client.Pair(ctx, ch, credentials, code, time.Now(), func(link string) {
			a.window.Update(func() {
				if a.pairAttempt == id && a.pairCancel != nil {
					a.pairReady, a.pairURL = true, link
				}
			})
		})
		a.window.Update(func() { a.finishPair(id, ch, credentials, result, err) })
	}()
}

// The generation and credential snapshot prevent a late result from changing
// another platform, a newly edited form, or a newer pairing attempt.
func (a *app) finishPair(id uint64, ch model.Channel, credentials model.Credentials, result channels.PairResult, err error) {
	if a.pairAttempt != id || a.page != "channels" || a.channel != ch || a.pairCancel == nil {
		return
	}
	current := *a.forms[ch]
	current.Normalize(ch)
	a.cancelPair()
	if current != credentials {
		return
	}
	if err != nil {
		a.showFeedback(locale.FromError(err), true)
		return
	}
	a.forms[ch].RecipientID, a.forms[ch].RecipientType = result.RecipientID, result.RecipientType
	if ch == model.DingTalk {
		a.forms[ch].RobotCode = result.RobotCode
	}
	a.pairedApps[ch] = pairingApp(ch, *a.forms[ch])
	a.fieldErrors = nil
	a.showFeedback(locale.M("Account identified. Save and test to finish."), false)
}

func (a *app) pairingFields(c *ui.Context, ch model.Channel, form *model.Credentials) {
	p := a.palette(c)
	pairing := a.pairCancel != nil
	ui.Column(c.Key("pairing-" + string(ch))).FillWidth().Gap(10).Padding(14).Background(p.selected.Alpha(.5)).Radius(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			label := "Pair your account"
			if pairing {
				label = "Connecting…"
				if a.pairReady {
					label = "Waiting for your code…"
				}
			} else if a.hasPairedAccount(ch) {
				label = "Account paired"
			} else if form.RecipientID != "" {
				label = "Pairing required"
			}
			ui.Text(c, a.tr(label)).FontWeight(600).Grow(1)
			if pairing {
				ui.Button(c.Key("cancel-pair"), a.tr("Cancel")).OnClick(a.cancelPair)
			} else {
				label = "Pair account"
				if a.hasPairedAccount(ch) {
					label = "Pair again"
				}
				button := ui.Button(c.Key("pair"), a.tr(label)).Disabled(a.busy).OnClick(a.startPair)
				a.fieldInput(button, "pairing")
			}
		})
		if message := a.fieldError("pairing"); message != "" {
			ui.Text(c, message).TextColor(c.Theme().Danger).Wrap().Role(ui.RoleStatus)
		}
		if pairing && a.pairReady {
			ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.pairCode).FontSize(fontSize(c, 17)).FontWeight(600)
				ui.Button(c.Key("copy-pair"), a.tr("Copy")).OnClick(func() { mygo.Clipboard.WriteText(a.pairCode) })
				if a.pairURL != "" {
					ui.Link(c, a.tr("Open bot"), a.pairURL)
				}
			})
			ui.Text(c, a.tr("Send this code privately to your bot. Nudilo will detect your account automatically. Valid for 3 minutes.")).TextColor(p.muted).FontSize(fontSize(c, 12)).Wrap()
		}
	})
}
