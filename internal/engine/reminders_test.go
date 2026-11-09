package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestTimedPauseResumesAfterRestartWithoutReplayingPausedEvents(t *testing.T) {
	e, p, v, s := harness(t)
	start := time.Now().Add(-2 * time.Hour)
	until := start.Add(time.Hour)
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true; p.Pause(start, until) }); err != nil {
		t.Fatal(err)
	}
	queue(t, p, "during-pause", start.Add(time.Minute))
	queue(t, p, "offline-after-pause", until.Add(time.Minute))
	var err error
	e, err = New(p, v, s)
	if err != nil {
		t.Fatal(err)
	}
	queue(t, p, "after-startup", time.Now())
	e.Step(context.Background(), time.Now())
	snapshot := e.Snapshot()
	if snapshot.Preferences.Paused || s.sent[model.Feishu] != 1 || s.events[0].ID != "after-startup" {
		t.Fatal("pause replayed or failed to resume", s.events)
	}
	saved, err := p.Preferences()
	if err != nil || saved.Paused || !saved.PauseUntil.Equal(until) {
		t.Fatal("resume not persisted", saved, err)
	}
}

func TestTimedPauseRetainsPendingRetryInTheSameRun(t *testing.T) {
	e, paths, _, sender := harness(t)
	sender.failOnce = true
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Slack] = true }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, paths, "before-pause", now)
	e.Step(context.Background(), now)
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Pause(now, now.Add(time.Hour)) }); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(time.Minute))
	if len(sender.events) != 1 || e.Snapshot().Records[0].State() != model.Pending {
		t.Fatal("pause discarded or sent a pending retry")
	}
	e.Step(context.Background(), now.Add(time.Hour))
	if len(sender.events) != 2 || e.Snapshot().Records[0].State() != model.Sent || e.Snapshot().Preferences.Paused {
		t.Fatal("pending retry failed to resume in the same run")
	}
}

func TestRoutingChangeDoesNotForwardAnOldRetry(t *testing.T) {
	e, p, _, s := harness(t)
	s.failOnce = true
	if err := e.UpdatePreferences(func(p *model.Preferences) {
		p.Channels[model.Feishu] = true
		p.Channels[model.Slack] = true
		p.Routes = []model.ProjectRoute{{Project: "work", Channels: []model.Channel{model.Slack}}}
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, p, "old", now)
	e.Step(context.Background(), now)
	if e.Snapshot().Records[0].State() != model.Pending {
		t.Fatal("fixture not waiting for retry")
	}
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Routes[0].Channels = []model.Channel{model.Feishu} }); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(time.Hour))
	if s.sent[model.Slack] != 1 || s.sent[model.Feishu] != 0 || e.Snapshot().Records[0].State() != model.Skipped {
		t.Fatal("old event forwarded after rule edit", s.sent)
	}
	queue(t, p, "new", now.Add(2*time.Hour))
	e.Step(context.Background(), now.Add(2*time.Hour))
	if s.sent[model.Feishu] != 1 {
		t.Fatal("new event ignored updated routing")
	}
}
func TestDiagnosticProbeDoesNotSendOrConsumeEvents(t *testing.T) {
	e, p, _, s := harness(t)
	queue(t, p, "keep", time.Now())
	if err := e.CheckStorage(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(p.Inbox())
	if err != nil || len(entries) != 1 || len(s.events) != 0 {
		t.Fatal("diagnosis changed the queue or sent a message")
	}
	if err = os.RemoveAll(p.Inbox()); err != nil {
		t.Fatal(err)
	}
	if e.CheckStorage() == nil {
		t.Fatal("unreadable queue marked healthy")
	}
}

func TestTimedResumeRetriesPersistenceWithoutSendingEarly(t *testing.T) {
	e, paths, _, sender := harness(t)
	now := time.Now()
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true; p.Pause(now.Add(-time.Hour), now) }); err != nil {
		t.Fatal(err)
	}
	queue(t, paths, "after-deadline", now.Add(time.Minute))
	if err := os.Remove(paths.Settings()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.Settings(), 0700); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(2*time.Minute))
	if !e.Snapshot().Preferences.Paused || len(sender.events) != 0 || e.Snapshot().Error == "" {
		t.Fatal("failed resume pretended to succeed")
	}
	if err := os.Remove(paths.Settings()); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(3*time.Minute))
	if e.Snapshot().Preferences.Paused || len(sender.events) != 1 || e.Snapshot().Error != "" {
		t.Fatal("resume did not recover after storage returned", e.Snapshot())
	}
}
