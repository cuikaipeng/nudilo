package model

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"callbell/internal/locale"
)

// Keep the last pause interval after resuming: events queued while the desktop
// was asleep must not become new notifications when it wakes up.
func (p *Preferences) Pause(now, until time.Time) {
	if !p.Paused {
		p.PauseStarted = now
	}
	p.Paused, p.PauseUntil = true, until
}
func (p *Preferences) Resume(now time.Time) {
	if p.Paused && (p.PauseUntil.IsZero() || now.Before(p.PauseUntil)) {
		p.PauseUntil = now
	}
	p.Paused = false
}
func (p Preferences) PausedFor(eventTime time.Time) bool {
	if !p.PauseStarted.IsZero() && eventTime.Before(p.PauseStarted) {
		return false
	}
	if !p.PauseUntil.IsZero() {
		return eventTime.Before(p.PauseUntil)
	}
	return p.Paused
}

// Project is the exact name already included in notification events. No full
// working directory or additional conversation content is collected for rules.
type ProjectRoute struct {
	Project  string    `json:"project"`
	Agent    Agent     `json:"agent,omitempty"`
	Channels []Channel `json:"channels"`
}

func (p Preferences) ValidateRoutes() error {
	if len(p.Routes) > 100 {
		return locale.M("Use up to 100 project rules.")
	}
	seen := map[string]bool{}
	for _, r := range p.Routes {
		if strings.TrimSpace(r.Project) == "" || !utf8.ValidString(r.Project) || utf8.RuneCountInString(r.Project) > 100 || strings.ContainsAny(r.Project, "\n\r") {
			return locale.M("Enter a project name of up to 100 characters.")
		}
		if r.Agent != "" && !slices.Contains(Agents, r.Agent) {
			return locale.M("Choose a supported assistant.")
		}
		key := r.Project + "\x00" + string(r.Agent)
		if seen[key] {
			return locale.M("A rule already exists for this project and assistant.")
		}
		seen[key] = true
		if len(r.Channels) == 0 {
			return locale.M("Choose at least one notification channel.")
		}
		for _, ch := range r.Channels {
			if !slices.Contains(Channels, ch) {
				return locale.M("This channel is not supported.")
			}
		}
	}
	return nil
}

// An assistant-specific rule takes precedence over the project's general rule.
// A matched rule with unavailable channels deliberately has no fallback.
func (p Preferences) Destinations(event Event) []Channel {
	selected := Channels
	match := -1
	for i, r := range p.Routes {
		if r.Project != event.Project {
			continue
		}
		if r.Agent == event.Agent.Normalized() {
			match = i
			break
		}
		if r.Agent == "" {
			match = i
		}
	}
	if match >= 0 {
		selected = p.Routes[match].Channels
	}
	result := []Channel{}
	for _, ch := range Channels {
		if p.Channels[ch] && slices.Contains(selected, ch) {
			result = append(result, ch)
		}
	}
	return result
}
