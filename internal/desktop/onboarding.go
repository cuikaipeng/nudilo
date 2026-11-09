package desktop

import (
	"time"

	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func (a *app) setupSteps() (bool, bool, bool) {
	assistant := a.readyAssistant()
	channel := a.healthyChannels() > 0
	tested := false
	for _, ch := range model.Channels {
		if a.hasSavedChannel(ch) && !a.channelHealth(ch).LastSuccess.IsZero() {
			tested = true
		}
	}
	return assistant, channel, tested
}
func (a *app) onboardingView(c *ui.Context) {
	assistant, channel, tested := a.setupSteps()
	if !a.setupRequested {
		if a.snapshot.Preferences.SetupDismissed {
			return
		}
		if assistant && channel && tested {
			// Remember completion so clearing activity does not restart onboarding.
			// Try only once per run if saving fails; avoid a render/retry loop.
			if !a.busy && !a.setupAutoDismissTried {
				a.setupAutoDismissTried = true
				a.setPreference(func(p *model.Preferences) { p.SetupDismissed = true })
			}
			return
		}
	}
	card(c, func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr("Get ready in three steps")).FontWeight(600).Grow(1)
			ui.Button(c, a.tr("Later")).OnClick(func() {
				a.setupRequested = false
				a.setPreference(func(p *model.Preferences) { p.SetupDismissed = true })
			})
		})
		steps := []struct {
			title  string
			done   bool
			action string
			run    func()
		}{
			{"Detect your AI assistant", assistant, "Add assistant", func() { a.showAssistants = true }},
			{"Connect a notification channel", channel, "Set up", func() { a.chooseChannel(a.firstConfiguredChannel()) }},
			{"Receive a test message", tested, "Send test", func() { a.chooseChannel(a.firstConfiguredChannel()) }},
		}
		for i, s := range steps {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				label := a.tr("Step {number}", i+1)
				if s.done {
					label = a.tr("Done")
				}
				ui.Text(c, label).Width(62).TextColor(c.Theme().TextMuted)
				ui.Text(c, a.tr(s.title)).Grow(1).MinWidth(0).Wrap()
				if s.done {
					ui.Icon(c, iconCheck).Size(18, 18).TextColor(c.Theme().Success)
				} else {
					ui.Button(c, a.tr(s.action)).OnClick(s.run)
				}
			})
		}
	})
}
func (a *app) pauseFor(option int) {
	now := time.Now()
	until := time.Time{}
	switch option {
	case 0:
		until = now.Add(15 * time.Minute)
	case 1:
		until = now.Add(time.Hour)
	case 2:
		until = time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
	}
	a.setPreference(func(p *model.Preferences) { p.Pause(now, until) })
}
func (a *app) togglePause() {
	paused := a.snapshot.Preferences.Paused
	now := time.Now()
	a.setPreference(func(p *model.Preferences) {
		if paused {
			p.Resume(now)
		} else {
			p.Pause(now, time.Time{})
		}
	})
}

var pauseOptions = []string{"For 15 minutes", "For 1 hour", "Until tomorrow at 09:00", "Until I resume"}

func (a *app) pauseMenu(c *ui.Context) {
	ui.PrimaryButton(c.Key("pause-duration"), "").
		Label(a.tr("Pause duration")).Tooltip(a.tr("Pause duration")).
		Width(40).MinHeight(40).Padding(0).Radius(0, 24, 24, 0).Disabled(a.busy).
		PopoverMenuButton(func(m *ui.Menu) {
			for i, label := range pauseOptions {
				if m.Item(a.tr(label)).Disabled(a.busy).Chosen() {
					a.pauseFor(i)
				}
			}
		}).Children(func() { ui.Icon(c, iconDown).Size(14, 14) })
}
