package desktop

import (
	"context"

	"callbell/internal/channels"
	"callbell/internal/engine"
	"callbell/internal/model"
)

// A recipient belongs to the application that identified it. Changing that
// application requires pairing again; changing only the destination mode does not.
type pairIdentity struct{ id, secret, token string }

func pairingApp(ch model.Channel, c model.Credentials) pairIdentity {
	c.Normalize(ch)
	if ch == model.Telegram || ch == model.Slack {
		return pairIdentity{token: c.Token}
	}
	return pairIdentity{id: c.AppID, secret: c.AppSecret}
}

func (a *app) hasPairedAccount(ch model.Channel) bool {
	identity, ok := a.pairedApps[ch]
	return ok && a.forms[ch].RecipientID != "" && identity == pairingApp(ch, *a.forms[ch])
}

func (a *app) hasSavedChannel(ch model.Channel) bool {
	_, loaded := a.savedChannels[ch]
	return a.snapshot.Preferences.Channels[ch] && loaded && !a.localMissing[ch]
}

func (a *app) restoreChannelForm(ch model.Channel) {
	credentials, saved := a.savedChannels[ch]
	if !saved {
		credentials.Mode = a.snapshot.Preferences.ChannelModes[ch]
	}
	credentials.Normalize(ch)
	*a.forms[ch] = credentials
	delete(a.pairedApps, ch)
	if credentials.RecipientID != "" {
		a.pairedApps[ch] = pairingApp(ch, credentials)
	}
	delete(a.editingChannels, ch)
}

func (a *app) editChannel() {
	if a.busy || !a.hasSavedChannel(a.channel) {
		return
	}
	a.cancelPair()
	a.resetFeedback()
	a.restoreChannelForm(a.channel)
	a.editingChannels[a.channel] = true
	a.channelOptions = a.forms[a.channel].Secret != ""
}

func (a *app) cancelChannelEdit() {
	if a.busy {
		return
	}
	a.cancelPair()
	a.resetFeedback()
	a.restoreChannelForm(a.channel)
	a.channelOptions = false
}

func (a *app) channelDraft(ch model.Channel) model.Credentials {
	credentials := *a.forms[ch]
	credentials.Normalize(ch)
	if !a.hasPairedAccount(ch) {
		credentials.RecipientID, credentials.RecipientType, credentials.RobotCode = "", "", ""
	}
	return credentials
}

type channelSaveResult struct {
	saved bool
	err   error
	local localChannels
}

func saveChannelSetup(ctx context.Context, e *engine.Engine, ch model.Channel, credentials model.Credentials, test bool) channelSaveResult {
	result := channelSaveResult{}
	result.err = e.SaveChannel(ch, credentials)
	result.saved = result.err == nil
	if result.saved && test {
		result.err = e.TestChannel(ctx, ch)
	}
	result.local = readLocalChannels(e, e.Snapshot().Preferences)
	return result
}

func (a *app) finishChannelSave(ch model.Channel, submitted model.Credentials, result channelSaveResult) {
	a.snapshot = a.engine.Snapshot()
	a.applyLocalChannels(result.local)
	// A test can fail after the setup was successfully saved. It is still
	// configured, and the error remains in the toast and notification history.
	current := *a.forms[ch]
	current.Normalize(ch)
	if result.saved && current == submitted {
		a.restoreChannelForm(ch)
	}
}

func (a *app) saveChannel(test bool) {
	if a.pairCancel != nil || a.busy {
		return
	}
	ch := a.channel
	credentials := a.channelDraft(ch)
	if err := channels.Validate(ch, credentials); err != nil {
		a.formError(err)
		return
	}
	submitted := *a.forms[ch]
	submitted.Normalize(ch)
	success := "Saved."
	if test {
		success = "Test sent."
	}
	a.fieldErrors = nil
	var result channelSaveResult
	a.workAfter(func() error {
		result = saveChannelSetup(a.ctx, a.engine, ch, credentials, test)
		return result.err
	}, success, func() { a.finishChannelSave(ch, submitted, result) })
}

func (a *app) testSavedChannel() {
	ch := a.channel
	if a.busy || !a.hasSavedChannel(ch) {
		return
	}
	var local localChannels
	a.workAfter(func() error {
		err := a.engine.TestChannel(a.ctx, ch)
		local = readLocalChannels(a.engine, a.engine.Snapshot().Preferences)
		return err
	}, "Test sent.", func() {
		a.snapshot = a.engine.Snapshot()
		a.applyLocalChannels(local)
	})
}

func (a *app) removeChannel() {
	if a.busy || a.pairCancel != nil {
		return
	}
	ch := a.channel
	var local localChannels
	a.workAfter(func() error {
		err := a.engine.RemoveChannel(ch)
		local = readLocalChannels(a.engine, a.engine.Snapshot().Preferences)
		return err
	}, "Channel removed.", func() {
		a.snapshot = a.engine.Snapshot()
		a.applyLocalChannels(local)
		if !a.snapshot.Preferences.Channels[ch] {
			a.restoreChannelForm(ch)
		}
	})
}
