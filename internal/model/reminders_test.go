package model

import (
	"reflect"
	"testing"
	"time"
)

func TestPauseIntervalSurvivesResumeAndRestart(t *testing.T) {
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	p := Defaults()
	p.Pause(start, end)
	if p.PausedFor(start.Add(-time.Second)) || !p.PausedFor(start) || !p.PausedFor(end.Add(-time.Second)) || p.PausedFor(end) {
		t.Fatal("pause boundary incorrect")
	}
	p.Pause(start.Add(time.Minute), end)
	if !p.PausedFor(start) {
		t.Fatal("changing pause duration lost earlier paused events")
	}
	p.Resume(end.Add(time.Hour))
	if p.Paused || !p.PauseUntil.Equal(end) || !p.PausedFor(start.Add(time.Minute)) || p.PausedFor(end) {
		t.Fatal("resume lost paused-event evidence")
	}
	p.Pause(start, time.Time{})
	p.Resume(start.Add(5 * time.Minute))
	if p.PausedFor(start.Add(6*time.Minute)) || !p.PausedFor(start) {
		t.Fatal("manual resume did not close pause interval")
	}
}
func TestProjectRoutesAreExactAndNeverFallbackFromUnavailableDestination(t *testing.T) {
	p := Defaults()
	p.Channels[Feishu] = true
	p.Channels[Slack] = true
	p.Routes = []ProjectRoute{{Project: "work", Channels: []Channel{Feishu}}, {Project: "work", Agent: ClaudeCode, Channels: []Channel{Slack}}}
	for _, tc := range []struct {
		project string
		agent   Agent
		want    []Channel
	}{
		{"work", Codex, []Channel{Feishu}}, {"work", ClaudeCode, []Channel{Slack}}, {"Work", Codex, []Channel{Feishu, Slack}}, {"other", ClaudeCode, []Channel{Feishu, Slack}},
	} {
		if got := p.Destinations(Event{Project: tc.project, Agent: tc.agent}); !reflect.DeepEqual(got, tc.want) {
			t.Fatal(tc, got)
		}
	}
	delete(p.Channels, Slack)
	if got := p.Destinations(Event{Project: "work", Agent: ClaudeCode}); len(got) != 0 {
		t.Fatal("routed private event fell back to another channel", got)
	}
	p.Routes = append(p.Routes, p.Routes[0])
	if p.ValidateRoutes() == nil {
		t.Fatal("duplicate rule accepted")
	}
}
func TestHealthDistinguishesTestFromRealDeliveryAndNewSetup(t *testing.T) {
	now := time.Now()
	records := []Record{
		{Event: Event{Kind: Test, CreatedAt: now}, Deliveries: map[Channel]Delivery{Feishu: {State: Sent, Attempts: 1, LastAttempt: now.Add(time.Second)}}},
	}
	h := Health(records, Feishu, time.Time{})
	if h.LastTestSuccess.IsZero() || !h.LastRealSuccess.IsZero() {
		t.Fatal("test claimed end-to-end verification")
	}
	records = append(records, Record{Event: Event{Kind: TurnEnded, CreatedAt: now.Add(time.Minute)}, Deliveries: map[Channel]Delivery{Feishu: {State: Pending, Attempts: 1, LastAttempt: now.Add(time.Minute), Error: "offline"}}})
	h = Health(records, Feishu, time.Time{})
	if h.State != Pending || h.LastSuccess.IsZero() {
		t.Fatal("retry hid delivery failure/history", h)
	}
	if h = Health(records, Feishu, now.Add(time.Hour)); h.State != "" || !h.LastTestSuccess.IsZero() {
		t.Fatal("new setup inherited old verification")
	}
}
