package desktop

import (
	"slices"
	"strings"

	"callbell/internal/locale"
	"callbell/internal/model"
	"github.com/egoist/mygo/ui"
)

func (a *app) editRoute(index int) {
	a.routeIndex = index
	a.routeError = locale.Message{}
	a.routeDraft = &model.ProjectRoute{}
	if index >= 0 && index < len(a.snapshot.Preferences.Routes) {
		r := a.snapshot.Preferences.Routes[index]
		r.Channels = slices.Clone(r.Channels)
		a.routeDraft = &r
	}
}
func (a *app) saveRoute() {
	if a.routeDraft == nil || a.busy {
		return
	}
	draft := *a.routeDraft
	draft.Project = strings.TrimSpace(draft.Project)
	draft.Channels = slices.Clone(draft.Channels)
	routes := slices.Clone(a.snapshot.Preferences.Routes)
	if a.routeIndex >= 0 && a.routeIndex < len(routes) {
		routes[a.routeIndex] = draft
	} else {
		routes = append(routes, draft)
	}
	validation := a.snapshot.Preferences
	validation.Routes = routes
	err := validation.ValidateRoutes()
	if err == nil {
		for _, ch := range draft.Channels {
			if !a.hasSavedChannel(ch) {
				err = locale.M("Set up the selected channels before saving this rule.")
				break
			}
		}
	}
	if err != nil {
		a.routeError = locale.FromError(err)
		return
	}
	a.commitRoutes(routes)
}
func (a *app) commitRoutes(routes []model.ProjectRoute) {
	edit := func(p *model.Preferences) { p.Routes = routes }
	if a.engine == nil {
		edit(&a.snapshot.Preferences)
		a.routeDraft = nil
		a.routeError = locale.Message{}
		return
	}
	saved := false
	a.workAfter(func() error { err := a.engine.UpdatePreferences(edit); saved = err == nil; return err }, "Saved.", func() {
		if saved {
			a.routeDraft = nil
			a.routeError = locale.Message{}
		}
	})
}
func (a *app) removeRoute() {
	index := a.routeIndex
	routes := slices.Clone(a.snapshot.Preferences.Routes)
	if index < 0 || index >= len(routes) {
		return
	}
	routes = append(routes[:index], routes[index+1:]...)
	a.commitRoutes(routes)
}
func (a *app) routingView(c *ui.Context) {
	section(c, a.tr("Project routing"), func() {
		card(c, func() {
			ui.Collapsible(c.Key("project-routing"), a.tr("Choose channels by project"), &a.routesOpen, func() {
				ui.Column(c).Gap(14).Padding(12, 0, 0, 0).Children(func() {
					caption(c, a.tr("Projects without a rule use all configured channels."))
					for i, r := range a.snapshot.Preferences.Routes {
						names := []string{}
						missing := []string{}
						for _, ch := range r.Channels {
							names = append(names, ch.Name(a.locale()))
							if !a.hasSavedChannel(ch) {
								missing = append(missing, ch.Name(a.locale()))
							}
						}
						ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
							ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
								ui.Text(c, r.Project).FontWeight(600).Wrap()
								assistant := a.tr("All assistants")
								if r.Agent != "" {
									assistant = r.Agent.Name()
								}
								caption(c, assistant+" → "+strings.Join(names, " · "))
								if len(missing) > 0 {
									ui.Text(c, a.tr("Needs setup: {channels}", strings.Join(missing, " · "))).TextColor(c.Theme().Warning).Wrap()
								}
							})
							ui.Button(c.Key(r.Project+string(r.Agent)), a.tr("Edit rule")).Disabled(a.busy).OnClick(func() { a.editRoute(i) })
						})
					}
					if a.routeDraft == nil {
						ui.Button(c.Key("add-project-rule"), a.tr("Add rule")).OnClick(func() { a.editRoute(-1) })
					} else {
						a.routeEditor(c)
					}
				})
			})
		})
	})
}
func (a *app) routeEditor(c *ui.Context) {
	draft := a.routeDraft
	ui.Column(c.Key("project-rule-editor")).Gap(12).Padding(14).Radius(10).Background(c.Theme().Background).Children(func() {
		ui.Field(c, a.tr("Project name"), func() {
			ui.TextInput(c.Key("route-project"), &draft.Project).Label(a.tr("Project name")).FillWidth().Disabled(a.busy)
		}).FillWidth()
		caption(c, a.tr("Match the project name shown in notifications, exactly. Same-name projects share this rule."))
		projects := []string{}
		for _, r := range a.snapshot.Records {
			if r.Event.Kind != model.Test && r.Event.Project != "" && !slices.Contains(projects, r.Event.Project) {
				projects = append(projects, r.Event.Project)
			}
		}
		if len(projects) > 0 {
			ui.PopoverMenuButton(c, a.tr("Use a recent project"), func(m *ui.Menu) {
				for _, p := range projects {
					if m.Item(p).Chosen() {
						draft.Project = p
					}
				}
			}).Disabled(a.busy)
		}
		agent := string(draft.Agent)
		if selectControl(c.Key("route-agent"), a.tr("Assistant"), &agent, []selectOption{{"", a.tr("All assistants")}, {"codex", "Codex"}, {"claude_code", "Claude Code"}}).Disabled(a.busy).Changed() {
			draft.Agent = model.Agent(agent)
		}
		ui.CheckboxGroup(c, a.tr("Send to"), func() {
			ui.Row(c).Gap(12).Wrap().Children(func() {
				for _, ch := range model.Channels {
					on := slices.Contains(draft.Channels, ch)
					label := ch.Name(a.locale())
					if !a.hasSavedChannel(ch) {
						label += " · " + a.tr("Not configured")
					}
					if ui.Checkbox(c.Key("route-"+string(ch)), &on, label).Disabled(a.busy || (!a.hasSavedChannel(ch) && !on)).Changed() {
						if on {
							draft.Channels = append(draft.Channels, ch)
						} else {
							draft.Channels = slices.DeleteFunc(draft.Channels, func(v model.Channel) bool { return v == ch })
						}
					}
				}
			})
		})
		if a.routeError.Key != "" {
			ui.Text(c, a.routeError.Text(a.locale())).TextColor(c.Theme().Danger).Wrap().Role(ui.RoleStatus)
		}
		ui.Row(c).Gap(10).Wrap().Children(func() {
			if a.routeIndex >= 0 {
				ui.Button(c, a.tr("Remove rule")).TextColor(c.Theme().Danger).Disabled(a.busy).OnClick(func() { a.removeRouteOpen = true })
			}
			ui.Spacer(c)
			ui.Button(c.Key("cancel-project-rule"), a.tr("Cancel")).Disabled(a.busy).OnClick(func() { a.routeDraft = nil; a.routeError = locale.Message{} })
			ui.PrimaryButton(c.Key("save-project-rule"), a.tr("Save rule")).Disabled(a.busy).OnClick(a.saveRoute)
		})
	})
}
