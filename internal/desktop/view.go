package desktop

import (
	"runtime"
	"slices"
	"time"

	"callbell/internal/buildinfo"
	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

var pages = []struct {
	id, title string
	icon      *ui.SVG
}{
	{"overview", "Overview", iconOverview}, {"channels", "Channels", iconChannels},
	{"history", "Activity", iconActivity}, {"settings", "Settings", iconSettings},
}

func (a *app) view(c *ui.Context) {
	a.syncPage()
	p := a.palette(c)
	applyPalette(c, p)
	c.Root().Background(p.canvas)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		top := float32(24)
		if runtime.GOOS == "darwin" {
			top = 58
		}
		ui.Column(c).Width(192).Shrink(0).Padding(top, 12, 18, 12).Gap(22).Children(func() {
			ui.Row(c.Key("brand-drag")).Padding(0, 10).Gap(7).AlignItems(ui.Center).DragWindow().Children(func() {
				if a.logo != nil {
					ui.Image(c, a.logo).Size(38, 38).Fit(ui.Contain)
				}
				ui.Text(c, "Nudilo").FontSize(fontSize(c, 21)).FontWeight(700)
			})
			ui.Sidebar(c.Key("navigation"), &a.page, func() {
				for _, page := range pages {
					item := ui.SidebarItem(c, page.id, page.icon, a.tr(page.title)).Padding(11, 10).Radius(9)
					if a.page == page.id {
						item.Background(p.selected).TextColor(p.accentInk)
					}
				}
			}).Background(ui.Transparent).Padding(0).FillWidth().Grow(1)
			a.updateIndicator(c)
			ui.Row(c).Padding(10).Gap(8).AlignItems(ui.Center).Children(func() {
				key, state := a.overallStatus()
				color := p.muted
				if state == "success" {
					color = c.Theme().Success
				}
				if state == "warning" {
					color = c.Theme().Warning
				}
				ui.Box(c).Size(6, 6).Radius(3).Background(color)
				ui.Text(c, a.tr(key)).FontSize(fontSize(c, 13)).TextColor(p.muted).Grow(1).MinWidth(0).Wrap().Role(ui.RoleStatus)
			})
		})
		ui.Column(c).Grow(1).MinWidth(0).Padding(12, 12, 12, 0).Children(func() {
			ui.Column(c).Fill().MinHeight(0).Background(p.card).Radius(16).Border(1, p.border.Alpha(.6)).Shadow(0, 3, 18, -6, p.shadow).Clip().Children(func() {
				ui.Scroll(c.Key("page-" + a.page)).Grow(1).MinHeight(0).Padding(24).Children(func() {
					gap := float32(20)
					if a.page == "overview" {
						gap = 16
					}
					ui.Column(c).Gap(gap).FillWidth().Children(func() {
						a.pageHeading(c)
						if a.fatal != nil {
							ui.Text(c, a.tr("Unable to start")).FontSize(fontSize(c, 22)).Bold()
							ui.Text(c, locale.FromError(a.fatal).Text(a.locale())).Wrap()
							return
						}
						switch a.page {
						case "channels":
							a.channelsView(c)
						case "history":
							a.historyView(c)
						case "settings":
							a.settingsView(c)
						default:
							a.overview(c)
						}
					})
				})
			})
		})
	})
	// The inset title bar is drawn by this view. Empty space beside the native
	// traffic lights must explicitly participate in window dragging too.
	if runtime.GOOS == "darwin" {
		ui.Box(c.Key("titlebar-drag")).Absolute().Top(0).Left(0).Width(192).Height(48).DragWindow()
		// This empty inset remains available after the page heading scrolls away.
		ui.Box(c.Key("content-inset-drag")).Absolute().Top(0).Left(192).Right(0).Height(24).DragWindow()
	}
	a.feedbackView(c)
	if ui.AlertDialog(c.Key("remove-channel-dialog"), &a.removeChannelOpen, a.tr("Remove this channel?"), a.tr("Its setup will be removed. Project rules will not switch to another channel."), a.tr("Cancel"), a.tr("Remove channel")) == 1 {
		a.removeChannel()
	}
	if ui.AlertDialog(c.Key("remove-route-dialog"), &a.removeRouteOpen, a.tr("Remove this project rule?"), a.tr("New notifications for this project will use the remaining rules or all configured channels."), a.tr("Cancel"), a.tr("Remove rule")) == 1 {
		a.removeRoute()
	}

	if ui.AlertDialog(c.Key("restart-update-dialog"), &a.updateRestartOpen, a.tr("Restart and discard changes?"),
		a.tr("Unsaved channel, notification and project rule edits will be lost."), a.tr("Cancel"), a.tr("Restart to update")) == 1 {
		restartForUpdate()
	}
	if ui.AlertDialog(c.Key("clear-history-dialog"), &a.clearHistoryOpen, a.tr("Clear all activity?"),
		a.tr("Notification records will be removed and pending retries canceled. This cannot be undone."), a.tr("Cancel"), a.tr("Clear all")) == 1 {
		a.work(a.engine.ClearHistory, "Activity cleared.")
	}
}
func (a *app) pageHeading(c *ui.Context) {
	title := "Overview"
	for _, p := range pages {
		if p.id == a.page {
			title = p.title
		}
	}
	ui.Row(c.Key("page-heading")).MinHeight(28).FillWidth().Gap(12).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Text(c, a.tr(title)).FontSize(fontSize(c, 20)).FontWeight(650)
		ui.Spacer(c)
		if a.page == "history" {
			ui.Button(c.Key("clear-history"), "").Label(a.tr("Clear all")).TextColor(c.Theme().Danger).
				Disabled(a.busy || len(a.snapshot.Records) == 0).OnClick(func() { a.clearHistoryOpen = true }).Children(func() {
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconTrash).Size(16, 16)
					ui.Text(c, a.tr("Clear all"))
				})
			})
		}
		if a.busy {
			ui.Spinner(c).Size(16, 16)
		}
	})
}
func section(c *ui.Context, title string, body func()) ui.Element {
	return ui.Column(c).Gap(12).FillWidth().Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(fontSize(c, 14)).FontWeight(650)
		}
		body()
	})
}
func card(c *ui.Context, body func()) ui.Element {
	return ui.Column(c).FillWidth().Padding(18).Gap(14).Background(c.Theme().Surface.Alpha(.55)).Radius(12).Children(body)
}
func caption(c *ui.Context, text string) {
	if text != "" {
		ui.Text(c, text).FontSize(fontSize(c, 13)).TextColor(c.Theme().TextMuted).Wrap()
	}
}
func (a *app) connectionStatus() (string, string) {
	if a.connecting {
		return a.tr("Connecting to Codex…"), ""
	}
	if a.connectionError != nil {
		return a.tr("Codex needs attention"), locale.FromError(a.connectionError).Text(a.locale())
	}
	if !a.snapshot.Preferences.AutoConnect || !a.connection.Ready {
		return a.tr("Codex disconnected"), ""
	}
	if a.monitorError != nil {
		return a.tr("Codex needs attention"), locale.FromError(a.monitorError).Text(a.locale())
	}
	last := a.lastAgentEvent(model.Codex)
	if !last.IsZero() && !last.Before(a.monitorSince) {
		return a.tr("Codex connected"), a.tr("Last event: {time}", a.date(last))
	}
	return a.tr("Listening for Codex"), a.tr("Waiting for the first event. Existing chats can send turn-ending reminders.")
}
func (a *app) date(t time.Time) string {
	return i18n.FormatDate(a.locale(), t.Local(), i18n.DateOptions{Skeleton: "MMMdHm"})
}
func (a *app) overview(c *ui.Context) {
	a.notificationControl(c)
	a.onboardingView(c)
	a.assistantsOverview(c)
	a.diagnosticsView(c)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.tr("Recent activity")).FontWeight(600).Grow(1)
		ui.Button(c, a.tr("View all")).OnClick(func() { a.page = "history" })
	})
	if a.snapshot.Error != "" {
		ui.Text(c, a.tr(a.snapshot.Error)).TextColor(c.Theme().Danger).Wrap().Role(ui.RoleStatus)
	}
	shown := 0
	for _, r := range a.snapshot.Records {
		if r.Event.Kind == model.Test {
			continue
		}
		a.record(c, r, false)
		shown++
		if shown == 2 {
			break
		}
	}
	if shown == 0 {
		caption(c, a.tr("No notifications yet"))
	}
	section(c, a.tr("Channels"), func() {
		ui.Row(c).Gap(8).FillWidth().Wrap().AlignItems(ui.Stretch).Children(func() {
			for _, ch := range model.Channels {
				a.channelCard(c, ch)
			}
		})
	})
}
func (a *app) kindRow(c *ui.Context, kind model.Kind) {
	ui.Row(c.Key("kind-" + string(kind))).FillWidth().MinHeight(28).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, kind.Name(a.locale())).Grow(1)
		value := a.snapshot.Preferences.Kinds[kind]
		if ui.Switch(c, &value).Label(kind.Name(a.locale())).Disabled(a.busy).Changed() {
			a.setPreference(func(p *model.Preferences) { p.Kinds[kind] = value })
		}
	})
}
func (a *app) channelsView(c *ui.Context) {
	p := a.palette(c)
	ui.Row(c).Gap(6).Wrap().Children(func() {
		for _, ch := range model.Channels {
			button := ui.Button(c.Key("channel-"+string(ch)), ch.Name(a.locale())).Padding(9, 14).Disabled(a.busy)
			if ch == a.channel {
				button.Background(p.selected).TextColor(p.accentInk)
			}
			button.OnClick(func() { a.chooseChannel(ch) })
		}
	})
	ch := a.channel
	form := a.forms[ch]
	form.Normalize(ch)
	if a.hasSavedChannel(ch) && !a.editingChannels[ch] {
		a.savedChannelView(c, ch, a.savedChannels[ch])
		return
	}
	card(c, func() {
		a.channelHeading(c, ch, a.editingChannels[ch])
		mode := 1
		if form.Mode == model.Direct {
			mode = 0
		}
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Send to")).Grow(1)
			if ui.Segmented(c.Key("destination-"+string(ch)), &mode, a.tr("Personal DM"), a.tr("Group / channel")).Label(a.tr("Send to")).Disabled(a.busy).Changed() {
				a.cancelPair()
				form.Mode = model.Group
				if mode == 0 {
					form.Mode = model.Direct
				}
				a.resetFeedback()
			}
		})
		if ch == model.Telegram {
			a.telegramFields(c, form)
		} else if form.Mode == model.Direct {
			a.directFields(c, ch, form)
		} else {
			a.secretField(c, "webhook", "Webhook URL", &form.Webhook, webhookPlaceholder(ch))
			if ch != model.Slack {
				ui.Collapsible(c.Key("channel-options-"+string(ch)), a.tr("Signing (optional)"), &a.channelOptions, func() {
					a.secretField(c, "secret", "Signing secret (optional)", &form.Secret, a.tr("Required only when signing is enabled"))
				}).FillWidth()
			}
		}
		ui.Divider(c)
		ui.Row(c).Gap(10).Wrap().Children(func() {
			if a.editingChannels[ch] {
				ui.Button(c.Key("cancel-edit"), a.tr("Cancel editing")).Disabled(a.busy).OnClick(a.cancelChannelEdit)
			} else if a.snapshot.Preferences.Channels[ch] {
				ui.Button(c, a.tr("Remove channel")).TextColor(c.Theme().Danger).Disabled(a.busy || a.pairCancel != nil).OnClick(func() { a.removeChannelOpen = true })
			}
			ui.Spacer(c)
			ui.Button(c.Key("save"), a.tr("Save")).Disabled(a.busy || a.pairCancel != nil).OnClick(func() { a.saveChannel(false) })
			label := "Save & test"
			if a.busy {
				label = "Working…"
			}
			ui.PrimaryButton(c.Key("save-test"), a.tr(label)).Disabled(a.busy || a.pairCancel != nil).OnClick(func() { a.saveChannel(true) })
		})
	})
	ui.Collapsible(c.Key("setup-help"), a.tr("Setup help"), &a.help, func() {
		ui.Column(c).Gap(10).Padding(12, 4).Children(func() {
			steps, url := guide(ch, form.Mode)
			for _, step := range steps {
				ui.Text(c, a.tr(step)).TextColor(p.muted).Wrap()
			}
			if ch == model.Slack && form.Mode == model.Direct {
				ui.Button(c, a.tr("Copy app manifest")).OnClick(func() { mygo.Clipboard.WriteText(slackManifest) })
			}
			ui.Link(c, a.tr("Official instructions"), url).TextColor(p.accentInk)
		})
	}).FillWidth()
}

func (a *app) textField(c *ui.Context, key, label string, value *string, placeholder string) {
	ui.Field(c, a.tr(label), func() {
		input := ui.TextInput(c.Key("field-"+key), value).Placeholder(placeholder).Disabled(a.busy || a.pairCancel != nil)
		a.fieldInput(input, key)
	}).FillWidth().Error(a.fieldError(key))
}

func (a *app) directFields(c *ui.Context, ch model.Channel, form *model.Credentials) {
	switch ch {
	case model.Slack:
		a.secretField(c, "token", "Bot User OAuth Token", &form.Token, "xoxb-…")
	case model.Feishu, model.Lark:
		a.textField(c, "app-id", "App ID", &form.AppID, "cli_…")
		a.secretField(c, "app-secret", "App Secret", &form.AppSecret, a.tr("From your app's credentials page"))
	case model.DingTalk:
		a.textField(c, "app-id", "Client ID (AppKey)", &form.AppID, "ding…")
		a.secretField(c, "app-secret", "Client Secret (AppSecret)", &form.AppSecret, a.tr("From your app's credentials page"))
	}
	a.pairingFields(c, ch, form)
}

func (a *app) telegramFields(c *ui.Context, form *model.Credentials) {
	a.secretField(c, "token", "Bot Token", &form.Token, a.tr("From @BotFather"))
	if form.Mode == model.Group {
		a.textField(c, "group-chat", "Group / channel ID", &form.ChatID, "-100… / @channel")
		return
	}
	a.pairingFields(c, model.Telegram, form)
}
func (a *app) secretField(c *ui.Context, key, label string, value *string, placeholder string) {
	ui.Field(c, a.tr(label), func() {
		input := ui.TextInput(c.Key("field-"+key), value).Placeholder(placeholder).Disabled(a.busy || a.pairCancel != nil)
		input.Password()
		a.fieldInput(input, key)
	}).FillWidth().Error(a.fieldError(key))
}
func webhookPlaceholder(ch model.Channel) string {
	switch ch {
	case model.Feishu:
		return "https://open.feishu.cn/open-apis/bot/v2/hook/…"
	case model.Lark:
		return "https://open.larksuite.com/open-apis/bot/v2/hook/…"
	case model.DingTalk:
		return "https://oapi.dingtalk.com/robot/send?access_token=…"
	default:
		return "https://hooks.slack.com/services/…"
	}
}
func guide(ch model.Channel, mode model.DeliveryMode) ([]string, string) {
	if mode == model.Direct {
		switch ch {
		case model.Feishu, model.Lark:
			host := "https://open.feishu.cn"
			if ch == model.Lark {
				host = "https://open.larksuite.com"
			}
			return []string{"Create a custom app with a bot. Add im:message:send_as_bot and im:message.p2p_msg:readonly, include yourself in its availability and publish it.", "Use a long connection for events and subscribe to im.message.receive_v1. Copy App ID and App Secret into Nudilo.", "If the console cannot detect a connection, click Pair account here, then save the long-connection setting while Nudilo is waiting.", "Choose Pair account and send the code privately to your bot. Once identified, save and test."}, host + "/document/server-docs/im-v1/message/events/receive"
		case model.DingTalk:
			return []string{"Create an internal DingTalk app with a robot. Enable private-message sending, include yourself in its availability and publish it.", "Set the robot's message receiving mode to Stream. Copy Client ID and Client Secret into Nudilo.", "Choose Pair account and send the code privately to your bot. Once identified, save and test."}, "https://github.com/open-dingtalk/dingtalk-stream-sdk-go"
		case model.Slack:
			return []string{"For a new Slack app, copy the app manifest below and choose From a manifest in Slack.", "Create a Slack app with chat:write, im:read and im:history bot scopes. Enable App Home's Messages tab and allow users to send messages. Install or reinstall the app.", "Copy the Bot User OAuth Token into Nudilo. No app-level token or Socket Mode is needed.", "Choose Pair account and send the code privately to your bot. Once identified, save and test."}, "https://docs.slack.dev/reference/methods/conversations.history/"
		}
	}
	switch ch {
	case model.Feishu:
		return []string{"Add a custom bot in your Feishu group settings.", "Copy the Webhook and the signing secret, if enabled.", "Save and test, then check the group on your phone."}, "https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot"
	case model.Lark:
		return []string{"Add a Custom Bot in your Lark group settings.", "Copy the Webhook and the signing secret, if enabled.", "Save and test, then check the group on your phone."}, "https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/bot-v3/use-custom-bots-in-a-group"
	case model.DingTalk:
		return []string{"Add a custom bot in your DingTalk group settings.", "Use a signing secret, or allow both Codex and Claude Code as security keywords.", "Save and test, then check the group on your phone."}, "https://open.dingtalk.com/document/robots/custom-robot-access"
	case model.Telegram:
		if mode == model.Group {
			return []string{"Create a bot with @BotFather and add it to your group or channel with permission to send messages.", "Enter the group's negative Chat ID or the channel's @username, then save and test."}, "https://core.telegram.org/bots/api#sendmessage"
		}
		return []string{"Create a bot with /newbot in @BotFather and copy its token.", "Choose Pair account, then Open bot and press Start, or send the code privately to your bot.", "Once your account is identified, save and test."}, "https://core.telegram.org/bots/tutorial#obtain-your-bot-token"
	default:
		return []string{"Create a Slack App and enable Incoming Webhooks.", "Add a Webhook to your workspace and select a channel.", "Save and test, then check the group on your phone."}, "https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/"
	}
}
func (a *app) settingsView(c *ui.Context) {
	a.settingsSection(c, "general", a.tr("General"), func() {
		card(c, func() {
			a.settingRow(c, "Launch at login", "", func() {
				value := a.login
				if ui.Switch(c.Key("login"), &value).Label(a.tr("Launch at login")).Disabled(a.busy).Changed() {
					a.work(func() error {
						err := loginError(mygo.App.SetOpenAtLogin(value))
						a.window.Update(func() { a.login = mygo.App.OpenAtLogin() })
						return err
					}, "")
				}
			})
			a.settingRow(c, "Include a summary", "Sends up to 500 characters to your selected channels.", func() {
				value := a.snapshot.Preferences.IncludeSummary
				if ui.Switch(c.Key("summary"), &value).Label(a.tr("Include a summary")).Disabled(a.busy).Changed() {
					a.setPreference(func(p *model.Preferences) { p.IncludeSummary = value })
				}
			})
		})
	})
	a.settingsSection(c, "types", a.tr("Notification types"), func() {
		card(c, func() {
			for _, kind := range model.Kinds {
				a.kindRow(c, kind)
			}
		})
	})
	a.settingsBlock(c, "content", func() { a.notificationContentView(c) })
	a.settingsBlock(c, "routing", func() { a.routingView(c) })
	a.settingsSection(c, "assistants", a.tr("Codex"), func() {
		card(c, func() {
			title, detail := a.connectionStatus()
			ui.Text(c, title).FontWeight(600)
			caption(c, detail)
			if !a.connectionChecked.IsZero() {
				caption(c, a.tr("Checked {time}", a.date(a.connectionChecked)))
			}
			if a.connection.Ready {
				caption(c, a.tr("Reopen existing chats for answer and approval reminders."))
			}
			ui.Row(c).Gap(8).Children(func() {
				a.refreshButton(c, "refresh-connection-settings")
				ui.Button(c.Key("reconnect-settings"), a.tr("Reconnect")).Disabled(a.connecting || a.refreshing || a.busy || a.diagnosing).OnClick(a.connect)
				ui.Button(c.Key("disconnect"), a.tr("Disconnect")).Disabled(a.connecting || a.refreshing || a.busy || a.diagnosing || !a.snapshot.Preferences.AutoConnect).OnClick(a.disconnect)
				ui.Spacer(c)
				ui.Button(c, a.tr("Open data folder")).OnClick(a.openData)
			})
			ui.Collapsible(c.Key("advanced"), a.tr("Advanced"), &a.advanced, func() {
				ui.Column(c).Gap(12).Padding(12, 0).Children(func() {
					ui.Field(c, a.tr("Configuration folder"), func() {
						ui.TextInput(c.Key("codex-home"), &a.codexHome).Disabled(a.connecting).Placeholder(a.tr("Auto-detect"))
					}).FillWidth()
					ui.Field(c, a.tr("Codex executable"), func() {
						ui.TextInput(c.Key("codex-binary"), &a.codexBinary).Disabled(a.connecting).Placeholder(a.tr("Auto-detect"))
					}).FillWidth()
					caption(c, a.connection.Version)
				})
			})
		})
	})
	a.settingsBlock(c, "claude", func() { a.claudeSettingsView(c) })
	a.settingsSection(c, "appearance", a.tr("Appearance"), func() {
		card(c, func() {
			a.settingRow(c, "Theme", "", func() {
				options := []string{"system", "light", "dark"}
				labels := []string{a.tr("Follow system"), a.tr("Light"), a.tr("Dark")}
				index := max(0, slices.Index(options, a.snapshot.Preferences.Theme))
				if ui.Segmented(c.Key("theme"), &index, labels...).Label(a.tr("Theme")).Disabled(a.busy).Changed() {
					a.setPreference(func(p *model.Preferences) { p.Theme = options[index] })
				}
			})
			a.settingRow(c, "Language", "", func() { a.languagePicker(c) })
		})
	})
	ui.Button(c, a.tr("Show setup guide")).Disabled(a.busy).OnClick(func() {
		a.setPreference(func(p *model.Preferences) { p.SetupDismissed = false })
		a.setupRequested = true
		a.page = "overview"
	})
	a.updatesView(c)
	ui.Text(c, "Nudilo · "+a.tr("Version {version}", buildinfo.Version)).TextColor(c.Theme().TextMuted)
}
func (a *app) settingRow(c *ui.Context, key, detail string, control func()) {
	ui.Row(c.Key("setting-" + key)).FillWidth().MinHeight(34).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, a.tr(key)).Wrap()
			if detail != "" {
				caption(c, a.tr(detail))
			}
		})
		control()
	})
}

// Values stay as locale tags; explicit roles expose popup choices to native
// accessibility as well as keyboard and pointer input.
func (a *app) languagePicker(c *ui.Context) {
	if a.localeChoice == "" {
		a.localeChoice = a.snapshot.Preferences.Language
	}
	tags := append([]string{"system"}, locale.Catalog.Languages()...)
	label := func(tag string) string {
		if tag == "system" {
			return a.tr("Follow system")
		}
		return i18n.Autonym(tag)
	}
	picker := ui.SelectBase(c.Key("language"), &a.localeChoice)
	picker.Trigger.Label(a.tr("Language")).Value(label(a.localeChoice)).Width(190).Padding(7, 10).Gap(10).Radius(8).Background(c.Theme().Surface).Border(1, c.Theme().Border).Disabled(a.busy).Children(func() {
		ui.Text(c, label(a.localeChoice)).Grow(1)
		ui.Icon(c, iconDown).Size(12, 12).TextColor(c.Theme().TextMuted)
	})
	picker.Popup(func(panel ui.Element) {
		panel.Padding(4).Radius(8).Background(c.Theme().Background).Border(1, c.Theme().Border)
		for _, tag := range tags {
			option := picker.Item(tag).Label(label(tag)).Role(ui.RoleMenuItemRadio).Focusable().Padding(8, 10).Radius(6)
			if option.Highlighted() {
				option.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
			}
			option.Children(func() { ui.Text(c, label(tag)) })
		}
	})
	if picker.Trigger.Changed() {
		value := a.localeChoice
		a.setPreference(func(p *model.Preferences) { p.Language = value })
	}
}
