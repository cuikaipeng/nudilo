package desktop

import (
	"time"
	"unicode/utf8"

	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

type notificationDraft struct {
	content     model.NotificationContent
	kind        int
	agent       int
	previewTime time.Time
	caret       *int
}

func (a *app) notificationDraft(language string) *notificationDraft {
	if a.notificationDrafts == nil {
		a.notificationDrafts = map[string]*notificationDraft{}
	}
	if a.notificationDrafts[language] == nil {
		a.notificationDrafts[language] = &notificationDraft{
			content:     a.snapshot.Preferences.MessageContent[language].Resolved(language),
			previewTime: time.Now(),
		}
	}
	return a.notificationDrafts[language]
}

func (a *app) notificationContentView(c *ui.Context) {
	section(c, a.tr("Notification content"), func() {
		card(c, func() {
			caption(c, a.tr("Matches app language: {language}", i18n.Autonym(a.locale())))
			ui.Collapsible(c.Key("notification-content"), a.tr("Customize"), &a.notificationOpen, func() {
				ui.Column(c).FillWidth().Gap(14).Padding(12, 0, 0, 0).Children(func() {
					a.notificationEditor(c)
				})
			})
		})
	})
}

func (a *app) notificationEditor(c *ui.Context) {
	language := a.locale()
	draft := a.notificationDraft(language)
	key := c.Key("notification-" + language)
	message := func(err error) string {
		if err == nil {
			return ""
		}
		return locale.FromError(err).Text(language)
	}
	titleError := model.ValidateNotificationTitle(draft.content.Title)
	ui.Field(key, a.tr("Notification title"), func() {
		ui.TextInput(key.Key("title"), &draft.content.Title).Label(a.tr("Notification title")).FillWidth().Disabled(a.busy)
	}).FillWidth().Error(message(titleError))
	labels := make([]string, len(model.Kinds))
	for i, kind := range model.Kinds {
		labels[i] = kind.Name(language)
	}
	ui.Segmented(key.Key("kind"), &draft.kind, labels...).Label(a.tr("Notification type")).Disabled(a.busy)
	kind := model.Kinds[draft.kind]
	body := draft.content.Body(kind)
	bodyError := model.ValidateNotificationBody(*body)
	ui.Row(key).FillWidth().Gap(16).AlignItems(ui.Stretch).Children(func() {
		ui.Column(key).Grow(1).MinWidth(0).Gap(10).Children(func() {
			var input ui.Element
			ui.Field(key, a.tr("Message text"), func() {
				input = ui.TextArea(key.Key(string(kind)), body).Label(a.tr("Message text")).Height(170).FillWidth().Disabled(a.busy)
				if draft.caret != nil {
					input.SetTextSelection(*draft.caret, *draft.caret).Focus()
					draft.caret = nil
				}
			}).FillWidth().Error(message(bodyError))
			for _, variables := range [][]struct{ label, token string }{{{"Project", "{project}"}, {"Time", "{time}"}}, {{"Summary", "{summary}"}, {"Assistant", "{agent}"}}} {
				ui.Row(key).Gap(6).AlignItems(ui.Center).Children(func() {
					for _, variable := range variables {
						button := ui.Button(key.Key(variable.token), a.tr(variable.label)).Disabled(a.busy || input.Composing() || (variable.token == "{summary}" && !a.snapshot.Preferences.IncludeSummary))
						if variable.token == "{summary}" && !a.snapshot.Preferences.IncludeSummary {
							button.Tooltip(a.tr("Enable Include a summary in General first."))
						}
						button.OnClick(func() {
							start, end := input.TextSelection()
							value, caret := insertNotificationVariable(*body, variable.token, start, end)
							*body, draft.caret = value, &caret
						})
					}
				})
			}
			caption(key, a.tr("{count} / {limit} characters", utf8.RuneCountInString(*body), model.NotificationBodyLimit))
		})
		ui.Column(key).Width(260).Shrink(0).Gap(10).Children(func() {
			ui.Text(key, a.tr("Preview")).FontWeight(600)
			ui.Segmented(key.Key("preview-agent"), &draft.agent, "Codex", "Claude Code").Label(a.tr("Preview assistant"))
			event := model.Event{Agent: model.Agents[draft.agent], Kind: kind, Language: language, Project: "nudilo", Summary: a.tr("Your changes are ready to review."), CreatedAt: draft.previewTime}
			preview, truncated := model.RenderNotification(event, draft.content, a.snapshot.Preferences.IncludeSummary)
			ui.Scroll(key.Key("preview")).Height(230).FillWidth().Padding(14).Radius(10).Background(c.Theme().Background).Border(1, c.Theme().Border).Children(func() {
				ui.Text(key, preview).Wrap().FontSize(fontSize(c, 13))
			})
			if truncated {
				caption(key, a.tr("Long messages are shortened for delivery."))
			}
		})
	})
	caption(key, a.tr("The assistant and event type are always included."))
	valid := titleError == nil && bodyError == nil
	for _, other := range model.Kinds {
		if other != kind {
			if err := model.ValidateNotificationBody(*draft.content.Body(other)); err != nil {
				ui.Text(key, other.Name(language)+": "+message(err)).Wrap().TextColor(c.Theme().Danger).FontSize(fontSize(c, 12))
				valid = false
			}
		}
	}
	dirty := draft.content != a.snapshot.Preferences.MessageContent[language].Resolved(language)
	ui.Row(key).FillWidth().Gap(8).Children(func() {
		ui.Button(key.Key("reset"), a.tr("Restore defaults")).Disabled(a.busy || draft.content == model.DefaultNotificationContent(language)).OnClick(func() {
			draft.content = model.DefaultNotificationContent(language)
			draft.caret = nil
		})
		ui.Spacer(key)
		ui.Button(key.Key("cancel"), a.tr("Cancel")).Disabled(a.busy || !dirty).OnClick(func() {
			draft.content = a.snapshot.Preferences.MessageContent[language].Resolved(language)
			draft.caret = nil
		})
		ui.Button(key.Key("save"), a.tr("Save")).Background(c.Theme().Accent).TextColor(c.Theme().AccentText).Disabled(a.busy || !dirty || !valid).OnClick(func() { a.saveNotificationContent(language) })
	})
}

func insertNotificationVariable(value, token string, start, end int) (string, int) {
	runes := []rune(value)
	start = max(0, min(start, len(runes)))
	end = max(start, min(end, len(runes)))
	return string(runes[:start]) + token + string(runes[end:]), start + utf8.RuneCountInString(token)
}

func (a *app) saveNotificationContent(language string) {
	draft := a.notificationDraft(language)
	submitted := draft.content
	validate := model.Preferences{MessageContent: map[string]model.NotificationContent{language: submitted}}
	if validate.ValidateNotificationContent() != nil || a.busy {
		return
	}
	edit := func(p *model.Preferences) { p.SetNotificationContent(language, submitted) }
	if a.engine == nil {
		edit(&a.snapshot.Preferences)
		draft.content = a.snapshot.Preferences.MessageContent[language].Resolved(language)
		return
	}
	var saved bool
	a.workAfter(func() error {
		err := a.engine.UpdatePreferences(edit)
		saved = err == nil
		return err
	}, "Saved.", func() {
		if saved && draft.content == submitted {
			// Capture language when Save is clicked; switching language while a
			// write completes must never replace the other language's draft.
			a.snapshot = a.engine.Snapshot()
			draft.content = a.snapshot.Preferences.MessageContent[language].Resolved(language)
		}
	})
}
