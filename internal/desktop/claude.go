package desktop

import (
	"errors"
	"time"

	"callbell/internal/claude"
	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func (a *app) lastAgentEvent(agent model.Agent) time.Time {
	if a.snapshot.LastEvents != nil {
		return a.snapshot.LastEvents[agent]
	}
	// Older snapshots and headless fixtures predate the per-assistant map.
	if agent == model.Codex {
		return a.snapshot.LastEvent
	}
	return time.Time{}
}

func (a *app) connectClaude() {
	if a.claudeWorking || a.diagnosing || a.busy || a.engine == nil {
		return
	}
	a.claudeWorking = true
	a.claudeError = nil
	a.claudeConnection.Ready = false
	home, binary := a.claudeHome, a.claudeBinary
	go func() {
		err := a.engine.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = true; p.ClaudeHome = home; p.ClaudeBinary = binary })
		var result claude.Connection
		if err == nil {
			result, err = a.claudeInstaller.Connect(a.ctx, home, binary)
		}
		a.window.Update(func() {
			a.claudeWorking = false
			a.snapshot = a.engine.Snapshot()
			a.claudeConnection, a.claudeError = result, err
			a.claudeChecked = time.Now()
			if err == nil {
				a.claudeSince = a.claudeChecked
			}
		})
	}()
}

func (a *app) refreshClaude() {
	if a.claudeWorking || a.diagnosing || a.busy || a.engine == nil || !a.snapshot.Preferences.ClaudeAutoConnect {
		return
	}
	a.claudeWorking = true
	home := a.claudeConnection.Home
	binary := a.claudeConnection.Binary
	if binary == "" {
		binary = a.snapshot.Preferences.ClaudeBinary
	}
	if home == "" {
		home = a.snapshot.Preferences.ClaudeHome
	}
	go func() {
		result, err := a.claudeInstaller.Check(a.ctx, home, binary)
		a.window.Update(func() {
			a.claudeWorking = false
			a.claudeConnection, a.claudeError = result, err
			a.claudeChecked = time.Now()
		})
	}()
}

func (a *app) disconnectClaude() {
	if a.claudeWorking || a.diagnosing || a.busy || a.engine == nil {
		return
	}
	a.claudeWorking = true
	a.workAfter(func() error {
		// Disable collection first, including helpers already cached by a session.
		if err := a.engine.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = false }); err != nil {
			return err
		}
		return a.claudeInstaller.Disconnect()
	}, "Claude Code disconnected.", func() {
		a.claudeWorking = false
		a.snapshot = a.engine.Snapshot()
		if !a.snapshot.Preferences.ClaudeAutoConnect {
			a.claudeConnection = claude.Connection{}
			a.claudeError = nil
		}
	})
}

func (a *app) claudeStatus() (string, string) {
	if a.claudeWorking {
		return a.tr("Connecting to Claude Code…"), ""
	}
	if !a.snapshot.Preferences.ClaudeAutoConnect {
		return a.tr("Claude Code disconnected"), ""
	}
	if errors.Is(a.claudeError, claude.ErrNotFound) {
		return a.tr("Claude Code not found"), a.tr("Install and open Claude Code to connect it.")
	}
	if a.claudeError != nil {
		return a.tr("Claude Code needs attention"), locale.FromError(a.claudeError).Text(a.locale())
	}
	if !a.claudeConnection.Ready {
		return a.tr("Claude Code disconnected"), ""
	}
	last := a.lastAgentEvent(model.ClaudeCode)
	if !last.IsZero() && !last.Before(a.claudeSince) {
		return a.tr("Claude Code connected"), a.tr("Last event: {time}", a.date(last))
	}
	return a.tr("Listening for Claude Code"), a.tr("Waiting for the first Claude Code event.")
}

func (a *app) claudeRefreshButton(c *ui.Context, key string) {
	label := "Refresh status"
	if a.claudeWorking {
		label = "Checking…"
	}
	button := ui.Button(c.Key(key), "").Label(a.tr("Refresh {agent}", "Claude Code")).Disabled(a.claudeWorking || a.diagnosing || a.busy || !a.snapshot.Preferences.ClaudeAutoConnect).OnClick(a.refreshClaude).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			if a.claudeWorking {
				ui.Spinner(c).Size(14, 14)
			} else {
				ui.Icon(c, iconRefresh).Size(14, 14)
			}
			ui.Text(c, a.tr(label))
		})
	})
	if !a.claudeChecked.IsZero() {
		button.Tooltip(a.tr("Checked {time}", a.date(a.claudeChecked)))
	}
}

func (a *app) assistantsOverview(c *ui.Context) {
	p := a.palette(c)
	card(c, func() {
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("AI assistants")).FontWeight(600).Grow(1)
			assistantAction := "Add assistant"
			if a.showAssistants {
				assistantAction = "Done"
			}
			ui.Button(c, a.tr(assistantAction)).OnClick(func() { a.showAssistants = !a.showAssistants })
			ui.Button(c.Key("diagnose"), a.tr("Diagnose connection")).Disabled(a.busy || a.diagnosing || a.connecting || a.refreshing || a.claudeWorking).OnClick(a.runDiagnostics)
		})
		for _, agent := range model.Agents {
			if !a.assistantVisible(agent) {
				continue
			}
			ui.Row(c.Key("assistant-" + string(agent))).Gap(12).AlignItems(ui.Center).Children(func() {
				title, detail := a.connectionStatus()
				ready := a.connection.Ready && a.connectionError == nil && a.monitorError == nil && a.snapshot.Preferences.AutoConnect
				if agent == model.ClaudeCode {
					title, detail = a.claudeStatus()
					ready = a.claudeConnection.Ready && a.claudeError == nil && a.snapshot.Preferences.ClaudeAutoConnect
				}
				ui.Box(c).Size(34, 34).Shrink(0).Radius(10).Background(p.selected).Center().Children(func() {
					icon := iconChannels
					since := a.monitorSince
					if agent == model.ClaudeCode {
						since = a.claudeSince
					}
					if last := a.lastAgentEvent(agent); ready && !last.IsZero() && !last.Before(since) {
						icon = iconCheck
					}
					ui.Icon(c, icon).Size(18, 18).TextColor(p.accentInk)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() { ui.Text(c, title).FontWeight(600); caption(c, detail) })
				if agent == model.Codex {
					if !ready {
						ui.Button(c.Key("reconnect"), a.tr("Connect")).Disabled(a.connecting || a.refreshing || a.busy || a.diagnosing).OnClick(a.connect)
					}
					a.refreshButton(c, "refresh-connection")
				} else {
					if !ready {
						ui.Button(c.Key("connect-claude"), a.tr("Connect")).Disabled(a.claudeWorking || a.diagnosing || a.busy).OnClick(a.connectClaude)
					}
					a.claudeRefreshButton(c, "refresh-claude-overview")
				}
			})
		}
	})
}

func (a *app) claudeSettingsView(c *ui.Context) {
	section(c, "Claude Code", func() {
		card(c, func() {
			title, detail := a.claudeStatus()
			ui.Text(c, title).FontWeight(600)
			caption(c, detail)
			if !a.claudeChecked.IsZero() {
				caption(c, a.tr("Checked {time}", a.date(a.claudeChecked)))
			}
			if a.claudeConnection.Ready {
				caption(c, a.tr("If existing sessions stay silent, reopen them to load the listeners."))
			}
			ui.Row(c).Gap(8).Children(func() {
				a.claudeRefreshButton(c, "refresh-claude-settings")
				ui.Button(c.Key("reconnect-claude"), a.tr("Reconnect")).Disabled(a.claudeWorking || a.diagnosing || a.busy).OnClick(a.connectClaude)
				ui.Button(c.Key("disconnect-claude"), a.tr("Disconnect")).Disabled(a.claudeWorking || a.diagnosing || a.busy || !a.snapshot.Preferences.ClaudeAutoConnect).OnClick(a.disconnectClaude)
			})
			ui.Collapsible(c.Key("claude-advanced"), a.tr("Advanced"), &a.claudeAdvanced, func() {
				ui.Column(c).Gap(12).Padding(12, 0).Children(func() {
					ui.Field(c, a.tr("Claude Code configuration folder"), func() {
						ui.TextInput(c.Key("claude-home"), &a.claudeHome).Placeholder(a.tr("Auto-detect")).Disabled(a.claudeWorking || a.diagnosing || a.busy)
					}).FillWidth()
					ui.Field(c, a.tr("Claude Code executable"), func() {
						ui.TextInput(c.Key("claude-binary"), &a.claudeBinary).Placeholder(a.tr("Auto-detect")).Disabled(a.claudeWorking || a.diagnosing || a.busy)
					}).FillWidth()
					caption(c, a.claudeConnection.Version)
				})
			})
			for _, ch := range []model.Channel{model.Feishu, model.Lark, model.DingTalk} {
				if a.snapshot.Preferences.Channels[ch] && a.snapshot.Preferences.ChannelModes[ch] != model.Direct {
					caption(c, a.tr("For group bot keyword checks, allow both Codex and Claude Code."))
					break
				}
			}
		})
	})
}
