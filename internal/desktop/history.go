package desktop

import (
	"sort"
	"strings"
	"time"

	"callbell/internal/model"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

type selectOption struct{ value, label string }

func selectControl(c *ui.Context, label string, value *string, options []selectOption) ui.Element {
	text := *value
	for _, o := range options {
		if o.value == *value {
			text = o.label
			break
		}
	}
	s := ui.SelectBase(c, value)
	s.Trigger.Label(label).Value(text).MinWidth(120).Padding(8, 10).Gap(8).Radius(8).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() { ui.Text(c, text).Grow(1).MinWidth(0).Wrap(); ui.Icon(c, iconDown).Size(12, 12) })
	s.Popup(func(panel ui.Element) {
		panel.Padding(4).Radius(8).Background(c.Theme().Background).Border(1, c.Theme().Border).MaxHeight(280)
		for _, o := range options {
			item := s.Item(o.value).Label(o.label).Padding(9, 10).Radius(6)
			if item.Highlighted() {
				item.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
			}
			item.Children(func() { ui.Text(c, o.label).Wrap() })
		}
	})
	return s.Trigger
}
func recordKey(r model.Record) string { return string(r.Event.Agent.Normalized()) + ":" + r.Event.ID }
func (a *app) filteredHistory() []model.Record {
	query := strings.ToLower(strings.TrimSpace(a.historyQuery))
	records := []model.Record{}
	for _, r := range a.snapshot.Records {
		if a.historyAgent != "" && (r.Event.Kind == model.Test || r.Event.Agent.Normalized() != a.historyAgent) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(r.Event.Project), query) {
			continue
		}
		if a.historyState != "" {
			match := string(r.State()) == a.historyState
			for _, d := range r.Deliveries {
				if string(d.State) == a.historyState {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		records = append(records, r)
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].Event.CreatedAt.After(records[j].Event.CreatedAt) })
	return records
}
func (a *app) historyView(c *ui.Context) {
	if a.snapshot.Error != "" {
		ui.Text(c, a.tr(a.snapshot.Error)).TextColor(c.Theme().Danger).Wrap().Role(ui.RoleStatus)
	}
	ui.Row(c).Gap(10).Wrap().Children(func() {
		if ui.TextInput(c.Key("history-search"), &a.historyQuery).Label(a.tr("Search projects")).Placeholder(a.tr("Search projects")).MinWidth(180).Grow(1).Changed() {
			a.historyLimit = 100
		}
		agent := string(a.historyAgent)
		if selectControl(c.Key("history-agent"), a.tr("Assistant filter"), &agent, []selectOption{{"", a.tr("All assistants")}, {"codex", "Codex"}, {"claude_code", "Claude Code"}}).Changed() {
			a.historyAgent = model.Agent(agent)
			a.historyLimit = 100
		}
		states := []selectOption{{"", a.tr("All statuses")}}
		for _, state := range []model.State{model.Failed, model.Pending, model.Sent, model.Skipped, model.Expired} {
			states = append(states, selectOption{string(state), state.Name(a.locale())})
		}
		if selectControl(c.Key("history-state"), a.tr("Status filter"), &a.historyState, states).Changed() {
			a.historyLimit = 100
		}
	})
	records := a.filteredHistory()
	if len(records) == 0 {
		ui.Column(c).FillWidth().Padding(40, 24).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconBell).Size(32, 32).TextColor(c.Theme().TextMuted)
			if len(a.snapshot.Records) > 0 {
				ui.Text(c, a.tr("No matching notifications"))
				ui.Button(c, a.tr("Clear filters")).OnClick(func() { a.historyQuery, a.historyState = "", ""; a.historyAgent = "" })
			} else {
				ui.Text(c, a.tr("No notifications yet")).FontWeight(600)
				if a.healthyChannels() == 0 {
					ui.PrimaryButton(c, a.tr("Set up")).OnClick(func() { a.chooseChannel(a.firstConfiguredChannel()) })
				}
			}
		})
		return
	}
	caption(c, a.tr("{count, plural, one {# notification} other {# notifications}}", len(records)))
	limit := a.historyLimit
	if limit <= 0 {
		limit = 100
	}
	lastDay := ""
	for i, r := range records {
		if i >= limit {
			break
		}
		day := r.Event.CreatedAt.Local().Format("2006-01-02")
		if day != lastDay {
			lastDay = day
			ui.Text(c, a.historyDay(r.Event.CreatedAt)).FontWeight(600).Padding(10, 0, 2, 0)
		}
		a.record(c, r, true)
	}
	if len(records) > limit {
		ui.Button(c, a.tr("Show more")).OnClick(func() { a.historyLimit = limit + 100 })
	}
}
func (a *app) historyDay(at time.Time) string {
	now := time.Now()
	day := at.Local().Format("2006-01-02")
	if day == now.Format("2006-01-02") {
		return a.tr("Today")
	}
	if day == now.AddDate(0, 0, -1).Format("2006-01-02") {
		return a.tr("Yesterday")
	}
	return i18n.FormatDate(a.locale(), at.Local(), i18n.DateOptions{Skeleton: "yMMMd"})
}
func (a *app) record(c *ui.Context, r model.Record, detail bool) {
	key := recordKey(r)
	ui.Column(c.Key(key)).FillWidth().Padding(14, 16).Gap(10).Background(c.Theme().Surface.Alpha(.55)).Radius(10).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			icon := iconBell
			if r.State() == model.Failed {
				icon = iconAlert
			}
			ui.Icon(c, icon).Size(18, 18).Shrink(0).TextColor(c.Theme().TextMuted)
			ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
				ui.Text(c, r.Event.Kind.Name(a.locale())).FontWeight(600).Wrap()
				caption(c, r.Event.AgentName()+" · "+r.Event.Project)
			})
			ui.Column(c).MaxWidth(220).Gap(5).AlignItems(ui.End).Children(func() {
				color := c.Theme().TextMuted
				if r.State() == model.Failed {
					color = c.Theme().Danger
				}
				label := r.State().Name(a.locale())
				if len(r.Deliveries) == 1 {
					for ch := range r.Deliveries {
						label = ch.Name(a.locale()) + " · " + label
					}
				}
				ui.Text(c, label).TextColor(color).Wrap()
				caption(c, i18n.FormatDate(a.locale(), r.Event.CreatedAt.Local(), i18n.DateOptions{Skeleton: "Hm"}))
			})
		})
		if !detail {
			return
		}
		hasDetail := r.Note != "" || len(r.Deliveries) > 1
		for _, d := range r.Deliveries {
			hasDetail = hasDetail || d.Error != "" || d.ErrorMessage != nil
		}
		if !hasDetail {
			return
		}
		if a.historyExpanded == nil {
			a.historyExpanded = map[string]bool{}
		}
		open := a.historyExpanded[key]
		ui.Collapsible(c.Key("details-"+key), a.tr("Delivery details"), &open, func() {
			ui.Column(c).Gap(10).Padding(8, 0).Children(func() {
				if r.Note != "" {
					caption(c, a.tr(r.Note))
				}
				for _, ch := range model.Channels {
					if d, ok := r.Deliveries[ch]; ok {
						label := ch.Name(a.locale()) + " · " + d.State.Name(a.locale())
						if d.ErrorMessage != nil {
							label += " · " + d.ErrorMessage.Text(a.locale())
						} else if d.Error != "" {
							label += " · " + a.tr(d.Error)
						}
						ui.Text(c, label).Wrap()
						if d.State == model.Pending && !d.NextAttempt.IsZero() {
							caption(c, a.tr("Retry scheduled: {time}", a.date(d.NextAttempt)))
						}
					}
				}
				if r.State() == model.Failed && r.Event.Kind != model.Test {
					ui.Button(c, a.tr("Retry failed channels")).Disabled(a.busy || a.engine == nil).OnClick(func() { a.work(func() error { return a.engine.Retry(r.Event.ID) }, "Added to the retry queue.") })
				}
			})
		})
		a.historyExpanded[key] = open
	})
}
