package desktop

import (
	"time"

	"callbell/internal/locale"
	"github.com/egoist/mygo/ui"
)

func (a *app) refreshConnection() {
	if a.connecting || a.refreshing || a.busy || a.diagnosing || a.engine == nil || !a.snapshot.Preferences.AutoConnect {
		return
	}
	a.refreshing = true
	generation := a.monitorGeneration
	// Check the active connection, not unsaved Advanced settings drafts.
	home, binary := a.connection.Home, a.connection.Binary
	if home == "" {
		home = a.snapshot.Preferences.CodexHome
	}
	if binary == "" {
		binary = a.snapshot.Preferences.CodexBinary
	}
	go func() {
		result, err := a.installer.Check(a.ctx, home, binary)
		a.window.Update(func() {
			a.refreshing = false
			if a.monitorGeneration != generation {
				return
			}
			a.snapshot = a.engine.Snapshot()
			a.connectionChecked = time.Now()
			if err == nil && a.monitorCancel == nil {
				err = locale.M("Reconnect to resume monitoring Codex.")
				result.Ready = false
			}
			a.connection, a.connectionError = result, err
		})
	}()
}

func (a *app) refreshButton(c *ui.Context, key string) {
	label := "Refresh status"
	if a.refreshing {
		label = "Checking…"
	}
	button := ui.Button(c.Key(key), "").Label(a.tr(label)).Disabled(a.connecting || a.refreshing || a.busy || a.diagnosing || !a.snapshot.Preferences.AutoConnect).
		OnClick(a.refreshConnection).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			if a.refreshing {
				ui.Spinner(c).Size(14, 14)
			} else {
				ui.Icon(c, iconRefresh).Size(14, 14)
			}
			ui.Text(c, a.tr(label))
		})
	})
	if !a.connectionChecked.IsZero() {
		button.Tooltip(a.tr("Checked {time}", a.date(a.connectionChecked)))
	}
}
