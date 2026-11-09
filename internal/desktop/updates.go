package desktop

import (
	"sync/atomic"

	"callbell/internal/buildinfo"
	"callbell/internal/model"
	"callbell/internal/updates"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var restartForUpdate = mygo.App.Relaunch

func (a *app) startUpdates() {
	var queued atomic.Bool
	a.updates = updates.New(a.ctx, updates.Configure(), a.snapshot.Preferences.AutomaticUpdates, func() {
		if queued.CompareAndSwap(false, true) {
			a.window.Update(func() {
				queued.Store(false)
				if a.ctx.Err() != nil {
					return
				}
				before := a.updateState.Phase
				a.updateState = a.updates.Snapshot()
				if before != a.updateState.Phase {
					a.refreshMenus()
				}
			})
		}
	})
	go a.updates.Run()
}
func (a *app) checkUpdates() {
	if a.updates != nil {
		a.updates.Check(true)
	}
}
func (a *app) openUpdates() { a.page = "settings"; a.updatesOpen = true; a.show(); a.checkUpdates() }
func (a *app) updateStatus() string {
	s := a.updateState
	switch s.Phase {
	case updates.Checking:
		return a.tr("Checking for updates…")
	case updates.Current:
		return a.tr("You're up to date.")
	case updates.Available:
		if !s.CanInstall {
			return a.tr("Version {version} is available. Update this installation manually.", s.Version)
		}
		return a.tr("Version {version} is available.", s.Version)
	case updates.Downloading:
		if s.Total > 0 {
			return a.tr("Downloading update… {percent}%", min(s.Bytes, s.Total)*100/s.Total)
		}
		return a.tr("Downloading update…")
	case updates.Ready:
		return a.tr("Version {version} is ready. Restart when you're ready.", s.Version)
	case updates.CheckFailed:
		return a.tr("Couldn't check for updates. Check your connection and try again.")
	case updates.DownloadFailed:
		return a.tr("Update download or verification failed. Try again.")
	case updates.NotPublished:
		return a.tr("No public update is available yet.")
	}
	return ""
}
func (a *app) updateBusy() bool {
	return a.updateState.Phase == updates.Checking || a.updateState.Phase == updates.Downloading
}
func (a *app) updatesView(c *ui.Context) {
	if a.updatesOpen {
		a.settingsTarget = "updates"
		a.updatesOpen = false
	}
	a.settingsSection(c, "updates", a.tr("Updates"), func() {
		card(c, func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Text(c, "Nudilo "+buildinfo.Version).FontWeight(600).Grow(1)
				label := "Check for updates"
				if a.updateState.Phase == updates.Checking {
					label = "Checking for updates…"
				}
				ui.Button(c.Key("check-updates"), a.tr(label)).Disabled(a.updateBusy() || a.updateState.Phase == updates.Ready).OnClick(a.checkUpdates)
			})
			a.settingRow(c, "Automatic updates", "Download in the background. Restart when you're ready.", func() {
				on := a.snapshot.Preferences.AutomaticUpdates
				if ui.Switch(c.Key("automatic-updates"), &on).Label(a.tr("Automatic updates")).Disabled(a.busy).Changed() {
					a.setPreference(func(p *model.Preferences) { p.AutomaticUpdates = on })
					if a.engine == nil && a.updates != nil {
						a.updates.SetAutomatic(on)
					}
				}
			})
			if text := a.updateStatus(); text != "" {
				ui.Text(c, text).FontSize(fontSize(c, 13)).TextColor(c.Theme().TextMuted).Wrap()
			}
			if a.updateState.Phase == updates.Downloading {
				progress := float64(-1)
				if a.updateState.Total > 0 {
					progress = float64(min(a.updateState.Bytes, a.updateState.Total)) / float64(a.updateState.Total)
				}
				ui.Progress(c, progress).FillWidth()
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if !a.updateState.Checked.IsZero() {
					caption(c, a.tr("Last checked {time}", a.date(a.updateState.Checked)))
				}
				ui.Spacer(c)
				switch a.updateState.Phase {
				case updates.Ready:
					ui.Button(c.Key("restart-update"), a.tr("Restart to update")).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).Disabled(a.busy || a.connecting || a.claudeWorking || a.diagnosing || a.pairCancel != nil).OnClick(a.restartUpdate)
				case updates.Available, updates.DownloadFailed:
					if a.updateState.CanInstall {
						label := "Download update"
						if a.updateState.Phase == updates.DownloadFailed {
							label = "Retry download"
						}
						ui.Button(c.Key("download-update"), a.tr(label)).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).OnClick(func() {
							if a.updates != nil {
								a.updates.Download()
							}
						})
					} else {
						ui.Button(c.Key("download-installer"), a.tr("Download latest version")).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).OnClick(func() { c.OpenURL("https://github.com/" + buildinfo.Repository + "/releases/latest") })
					}
				}
			})
		})
	})
}
func (a *app) restartUpdate() {
	if a.updateState.Phase != updates.Ready || a.busy || a.connecting || a.claudeWorking || a.diagnosing || a.pairCancel != nil {
		return
	}
	if a.routeDraft != nil {
		a.updateRestartOpen = true
		return
	}
	for ch, editing := range a.editingChannels {
		if !editing || a.forms[ch] == nil {
			continue
		}
		saved := a.savedChannels[ch]
		saved.Normalize(ch)
		if *a.forms[ch] != saved {
			a.updateRestartOpen = true
			return
		}
	}
	for language, draft := range a.notificationDrafts {
		if draft.content != a.snapshot.Preferences.MessageContent[language].Resolved(language) {
			a.updateRestartOpen = true
			return
		}
	}
	restartForUpdate()
}
func (a *app) updateIndicator(c *ui.Context) {
	label := ""
	switch a.updateState.Phase {
	case updates.Available:
		label = "Update available"
	case updates.Downloading:
		label = "Downloading update…"
	case updates.Ready:
		label = "Update ready"
	case updates.DownloadFailed:
		label = "Retry update"
	}
	if label == "" {
		return
	}
	ui.Button(c.Key("sidebar-update"), a.tr(label)).FillWidth().OnClick(func() { a.page = "settings"; a.updatesOpen = true })
}
