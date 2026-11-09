package desktop

import (
	"errors"
	"time"

	"callbell/internal/channels"
	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

type feedbackScope struct {
	epoch   uint64
	page    string
	channel model.Channel
	mode    model.DeliveryMode
}

func (a *app) scope() feedbackScope {
	return feedbackScope{a.feedbackEpoch, a.page, a.channel, a.forms[a.channel].Mode}
}

func (a *app) resetFeedback() {
	a.feedbackEpoch++
	a.showFeedback(locale.Message{}, false)
	a.fieldErrors = nil
	a.focusField = ""
}

func (a *app) syncPage() {
	if a.feedbackPage != a.page {
		if a.feedbackPage != "" {
			a.resetFeedback()
		}
		a.feedbackPage = a.page
		a.clearHistoryOpen = false
		if a.page != "channels" {
			a.cancelPair()
		}
	}
}

func (a *app) showFeedback(message locale.Message, isError bool) {
	a.message, a.messageError = message, isError
	a.feedbackVersion++
}

func (a *app) finishWork(scope feedbackScope, err error, success string) {
	a.busy = false
	// Navigation (including away and back) invalidates async feedback.
	if a.scope() != scope {
		return
	}
	if err != nil {
		a.showFeedback(locale.FromError(err), true)
		a.localeChoice = a.snapshot.Preferences.Language
	} else {
		a.showFeedback(locale.M(success), false)
	}
}

func (a *app) formError(err error) {
	a.fieldErrors = nil
	var failure *channels.Failure
	if errors.As(err, &failure) && failure.Field != "" {
		field, message := failure.Field, locale.FromError(err)
		switch field {
		case "recipient", "private-chat", "robot-code":
			field, message = "pairing", locale.M("Pair your account again to update the recipient.")
		}
		a.fieldErrors = map[string]locale.Message{field: message}
		a.focusField = field
		a.showFeedback(locale.Message{}, false)
		return
	}
	a.showFeedback(locale.FromError(err), true)
}

func (a *app) fieldError(key string) string {
	return a.fieldErrors[key].Text(a.locale())
}

func (a *app) fieldInput(input ui.Element, key string) {
	if input.Changed() {
		delete(a.fieldErrors, key)
	}
	if a.focusField == key {
		input.Focus()
		a.focusField = ""
	}
}

func (a *app) feedbackView(c *ui.Context) {
	const id = "nudilo-feedback"
	if a.displayedFeedback != a.feedbackVersion {
		a.displayedFeedback = a.feedbackVersion
		c.CloseToast(id)
		if a.message.Key != "" {
			kind, duration := "success", 3*time.Second
			if a.messageError {
				kind, duration = "error", 8*time.Second
			}
			version := a.feedbackVersion
			c.AddToast(ui.Toast{ID: id, Type: kind, Timeout: duration, Title: a.message.Text(a.locale()),
				OnClose: func() {
					if a.feedbackVersion == version {
						a.message = locale.Message{}
					}
				},
			})
		}
	}
	p := a.palette(c)
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Left(204).Right(24).Bottom(28).AlignItems(ui.Center)
		for _, toast := range toasts {
			parts := ui.ToastBase(c, toast)
			color, icon := c.Theme().Success, iconCheck
			if toast.Type == "error" {
				color, icon = c.Theme().Danger, iconAlert
			}
			parts.Root.Row().AlignItems(ui.Center).Gap(12).Padding(14, 16).MaxWidth(540).
				Background(p.card).Border(1, p.border).Radius(12).Shadow(0, 6, 24, -4, p.shadow).
				Transition(ui.ElementTransition{Enter: &ui.Motion{Y: 8}, Exit: &ui.Motion{Y: 6}, Duration: 180 * time.Millisecond}).Children(func() {
				ui.Icon(c, icon).Size(20, 20).Shrink(0).TextColor(color)
				ui.Text(c, a.message.Text(a.locale())).Grow(1).MinWidth(0).Wrap().Role(ui.RoleStatus)
				parts.CloseButton().Label(a.tr("Dismiss")).Padding(4).Children(func() { ui.Icon(c, iconClose).Size(16, 16).TextColor(p.muted) })
			})
			if left := parts.Left(); left > 0 {
				c.After(left)
			}
		}
	})
}
