package engine

import (
	"bytes"
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestClearHistoryPersistsRedactsAndPreservesDeduplication(t *testing.T) {
	e, paths, vault, sender := harness(t)
	sender.failOnce = true
	if err := e.UpdatePreferences(func(p *model.Preferences) {
		p.IncludeSummary = true
		p.Channels[model.Feishu], p.Channels[model.Slack] = true, true
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event := model.Event{ID: "hook-event", TurnID: "turn-1", Kind: model.TurnEnded, Project: "private-project", Summary: "private-summary", CreatedAt: now}
	if err := paths.QueueEvent(&event); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now)
	if e.Snapshot().Records[0].State() != model.Pending {
		t.Fatal("fixture must include a pending retry")
	}
	if err := e.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if s := e.Snapshot(); len(s.Records) != 0 || !s.LastEvent.Equal(now) {
		t.Fatal("clearing changed connection evidence or left visible history", s)
	}
	data, err := os.ReadFile(paths.History())
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private-project", "private-summary", "offline", "attempts"} {
		if bytes.Contains(data, []byte(private)) {
			t.Fatal("notification content retained", private)
		}
	}
	e, err = New(paths, vault, sender)
	if err != nil {
		t.Fatal(err)
	}
	// Both a repeated hook ID and the monitor's different ID for the same turn
	// remain deduplicated after restarting with a cleared activity list.
	if err := paths.QueueEvent(&event); err != nil {
		t.Fatal(err)
	}
	event.ID = "monitor-event"
	if err := paths.QueueEvent(&event); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(time.Minute))
	if len(e.Snapshot().Records) != 0 || sender.sent[model.Feishu] != 1 || sender.sent[model.Slack] != 1 {
		t.Fatal("cleared history replayed or resumed pending retries")
	}
	event.ID, event.TurnID, event.CreatedAt = "new-event", "turn-2", now.Add(2*time.Minute)
	if err := paths.QueueEvent(&event); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), event.CreatedAt)
	if s := e.Snapshot(); len(s.Records) != 1 || s.Records[0].State() != model.Sent {
		t.Fatal("new notifications stopped after clearing")
	}
}

func TestFailedClearPreservesHistory(t *testing.T) {
	e, paths, _, _ := harness(t)
	queue(t, paths, "keep-me", time.Now())
	e.Step(context.Background(), time.Now())
	// A directory at the destination makes the atomic rename fail on all OSes.
	if err := os.Rename(paths.History(), paths.History()+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.History(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := e.ClearHistory(); err == nil {
		t.Fatal("write failure swallowed")
	}
	if s := e.Snapshot(); len(s.Records) != 1 || s.Records[0].Event.ID != "keep-me" {
		t.Fatal("failed clear discarded in-memory history")
	}
}

type holdingSender struct {
	started, release chan struct{}
	calls            atomic.Int32
}

func (s *holdingSender) Send(context.Context, model.Channel, model.Credentials, model.Event) error {
	if s.calls.Add(1) == 1 {
		close(s.started)
		<-s.release
	}
	return nil
}

func TestClearHistoryWaitsForInFlightDelivery(t *testing.T) {
	e, paths, _, _ := harness(t)
	sender := &holdingSender{started: make(chan struct{}), release: make(chan struct{})}
	e.sender = sender
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, paths, "waiting", now)
	queue(t, paths, "in-flight", now.Add(time.Second))
	stepDone := make(chan struct{})
	go func() { e.Step(context.Background(), now.Add(2*time.Second)); close(stepDone) }()
	<-sender.started
	cleared := make(chan error, 1)
	go func() { cleared <- e.ClearHistory() }()
	select {
	case err := <-cleared:
		close(sender.release)
		<-stepDone
		t.Fatal("clear completed before an in-flight delivery", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(sender.release)
	<-stepDone
	if err := <-cleared; err != nil {
		t.Fatal(err)
	}
	if len(e.Snapshot().Records) != 0 {
		t.Fatal("in-flight delivery restored cleared history")
	}
	if sender.calls.Load() != 1 {
		t.Fatal("clear allowed another pending delivery to start")
	}
}
