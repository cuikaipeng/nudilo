package desktop

import (
	"context"
	"time"

	"callbell/internal/channels"
	"callbell/internal/claude"
	"callbell/internal/codex"
	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

type diagnosticResult struct {
	title, detail locale.Message
	ok            bool
}

func (a *app) channelHealth(ch model.Channel) model.ChannelHealth {
	return model.Health(a.snapshot.Records, ch, a.snapshot.Preferences.ChannelSince[ch])
}
func (a *app) assistantReady(agent model.Agent) bool {
	if !a.snapshot.Preferences.AgentEnabled(agent) {
		return false
	}
	if agent == model.ClaudeCode {
		return a.claudeConnection.Ready && a.claudeError == nil
	}
	return a.connection.Ready && a.connectionError == nil && a.monitorError == nil
}
func (a *app) assistantVisible(agent model.Agent) bool {
	if a.showAssistants || a.assistantReady(agent) || !a.lastAgentEvent(agent).IsZero() {
		return true
	}
	if agent == model.ClaudeCode {
		return a.claudeConnection.Binary != "" || a.claudeBinary != "" || a.claudeHome != ""
	}
	return a.connection.Binary != "" || a.codexBinary != "" || a.codexHome != ""
}
func (a *app) readyAssistant() bool {
	for _, agent := range model.Agents {
		if a.assistantReady(agent) {
			return true
		}
	}
	return false
}
func (a *app) healthyChannels() int {
	count := 0
	for _, ch := range model.Channels {
		if a.hasSavedChannel(ch) {
			count++
		}
	}
	return count
}

// The overall state uses usable channels. An unused assistant or a secondary
// channel needing setup must not imply that every notification has stopped.
func (a *app) overallStatus() (string, string) {
	if a.snapshot.Preferences.Paused {
		return "Paused", "muted"
	}
	if !a.readyAssistant() {
		return "Connect an AI assistant", "warning"
	}
	if a.healthyChannels() == 0 {
		return "Add a channel to get started.", "warning"
	}
	if a.snapshot.Error != "" {
		return "Local data needs attention", "warning"
	}
	for _, result := range a.diagnosis {
		if result.title.Key == "Local event queue" && !result.ok {
			return "Local data needs attention", "warning"
		}
	}
	for _, ch := range model.Channels {
		if !a.snapshot.Preferences.Channels[ch] {
			continue
		}
		h := a.channelHealth(ch)
		if a.localMissing[ch] || h.State == model.Failed || (h.State == model.Pending && h.Delivery.Error != "") {
			return "Running · some channels need attention", "warning"
		}
	}
	for _, rule := range a.snapshot.Preferences.Routes {
		for _, ch := range rule.Channels {
			if !a.hasSavedChannel(ch) {
				return "Running · project rules need attention", "warning"
			}
		}
	}
	return "Ready for your next reminder", "success"
}
func (a *app) deliveryStatus(ch model.Channel) string {
	h := a.channelHealth(ch)
	switch h.State {
	case model.Sent:
		return a.tr("Last accepted: {time}", a.date(h.LastSuccess))
	case model.Failed:
		return a.tr("Last send failed")
	case model.Pending:
		if h.Delivery.Error != "" && !h.Delivery.NextAttempt.IsZero() {
			return a.tr("Retry scheduled: {time}", a.date(h.Delivery.NextAttempt))
		}
		return a.tr("Waiting to send")
	case model.Expired:
		return a.tr("Last notification expired")
	}
	return a.tr("Not tested yet")
}
func (a *app) channelDeliveryView(c *ui.Context, ch model.Channel) {
	h := a.channelHealth(ch)
	color := c.Theme().TextMuted
	if h.State == model.Sent {
		color = c.Theme().Success
	}
	if h.State == model.Failed || h.State == model.Pending && h.Delivery.Error != "" {
		color = c.Theme().Warning
	}
	ui.Text(c, a.deliveryStatus(ch)).TextColor(color).Wrap().Role(ui.RoleStatus)
	if h.State == model.Failed || h.State == model.Pending && h.Delivery.Error != "" {
		detail := a.tr(h.Delivery.Error)
		if h.Delivery.ErrorMessage != nil {
			detail = h.Delivery.ErrorMessage.Text(a.locale())
		}
		caption(c, detail)
	}
	if !h.LastRealSuccess.IsZero() {
		caption(c, a.tr("AI event accepted: {time}", a.date(h.LastRealSuccess)))
	} else if !h.LastTestSuccess.IsZero() {
		caption(c, a.tr("Channel test passed. Waiting for a real AI event."))
	}
}
func (a *app) runDiagnostics() {
	if a.diagnosing || a.busy || a.engine == nil || a.connecting || a.refreshing || a.claudeWorking {
		return
	}
	a.diagnosisOpen, a.diagnosing, a.diagnosisReveal = true, true, true
	a.diagnosis = nil
	// All inputs are copied on the UI thread before the worker reads them.
	e, ci, cl := a.engine, a.installer, a.claudeInstaller
	pref := a.snapshot.Preferences
	codexHome, codexBinary := a.connection.Home, a.connection.Binary
	claudeHome, claudeBinary := a.claudeConnection.Home, a.claudeConnection.Binary
	checkCodex := a.assistantVisible(model.Codex) && pref.AutoConnect
	checkClaude := a.assistantVisible(model.ClaudeCode) && pref.ClaudeAutoConnect
	monitorErr := a.monitorError
	monitorRunning := a.monitorCancel != nil
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		var codexResult codex.Connection
		var claudeResult claude.Connection
		var codexErr, claudeErr error
		var codexCheck, claudeCheck func() error
		if checkCodex {
			codexCheck = func() error {
				codexResult, codexErr = ci.Check(ctx, codexHome, codexBinary)
				if codexErr == nil {
					codexErr = monitorErr
				}
				if codexErr == nil && !monitorRunning {
					codexErr = locale.M("Reconnect to resume monitoring Codex.")
				}
				return codexErr
			}
		}
		if checkClaude {
			claudeCheck = func() error { claudeResult, claudeErr = cl.Check(ctx, claudeHome, claudeBinary); return claudeErr }
		}
		results := collectDiagnostics(pref, codexCheck, claudeCheck, e.CheckStorage, e.Credentials)
		a.window.Update(func() {
			a.diagnosis, a.diagnosing, a.diagnosisAt = results, false, time.Now()
			a.diagnosisReveal = true
			if checkCodex {
				a.connection, a.connectionError, a.connectionChecked = codexResult, codexErr, a.diagnosisAt
			}
			if checkClaude {
				a.claudeConnection, a.claudeError, a.claudeChecked = claudeResult, claudeErr, a.diagnosisAt
			}
		})
	}()
}
func (a *app) diagnosticsView(c *ui.Context) {
	if !a.diagnosisOpen {
		return
	}
	panel := card(c, func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Connection diagnostics")).FontWeight(600).Grow(1)
			ui.Button(c, a.tr("Close")).OnClick(func() { a.diagnosisOpen = false })
		})
		if a.diagnosing {
			ui.Text(c, a.tr("Checking…")).Role(ui.RoleStatus)
			ui.Spinner(c).Size(18, 18)
		}
		for _, r := range a.diagnosis {
			color := c.Theme().Success
			if !r.ok {
				color = c.Theme().Warning
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, r.title.Text(a.locale())).FontWeight(600)
				ui.Text(c, r.detail.Text(a.locale())).TextColor(color).Wrap().Role(ui.RoleStatus)
			})
		}
		if !a.diagnosisAt.IsZero() {
			caption(c, a.tr("Checked {time}", a.date(a.diagnosisAt)))
		}
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Button(c, a.tr("AI assistant settings")).OnClick(func() {
				a.page = "settings"
				a.settingsTarget = "assistants"
				if !a.assistantVisible(model.Codex) && a.assistantVisible(model.ClaudeCode) {
					a.settingsTarget = "claude"
				}
			})
			ui.Button(c, a.tr("Test a channel")).OnClick(func() { a.chooseChannel(a.firstConfiguredChannel()) })
		})
	})
	if a.diagnosisReveal {
		panel.ScrollIntoView()
		a.diagnosisReveal = false
	}
}
func (a *app) firstConfiguredChannel() model.Channel {
	for _, ch := range model.Channels {
		if a.hasSavedChannel(ch) {
			return ch
		}
	}
	return model.Feishu
}

// These checks have no sender: diagnosing configuration never emits a message.
func collectDiagnostics(pref model.Preferences, codexCheck, claudeCheck, storageCheck func() error, credentials func(model.Channel) (model.Credentials, error)) []diagnosticResult {
	results := []diagnosticResult{}
	add := func(title string, err error, success string) {
		r := diagnosticResult{title: locale.M(title), detail: locale.M(success), ok: err == nil}
		if err != nil {
			r.detail = locale.FromError(err)
		}
		results = append(results, r)
	}
	if codexCheck != nil {
		add("Codex", codexCheck(), "Event listeners are ready.")
	}
	if claudeCheck != nil {
		add("Claude Code", claudeCheck(), "Event listeners are ready.")
	}
	if codexCheck == nil && claudeCheck == nil {
		add("AI assistants", locale.M("Connect an AI assistant"), "")
	}
	add("Local event queue", storageCheck(), "Local storage is readable and writable.")
	for _, ch := range model.Channels {
		if !pref.Channels[ch] {
			continue
		}
		value, err := credentials(ch)
		if err == nil {
			err = channels.Validate(ch, value)
		}
		add(ch.Name(), err, "Saved setup is valid. Use Send test to check delivery.")
	}
	return results
}
