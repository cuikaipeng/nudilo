package engine

import (
	"context"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestSourcesRemainIndependentThroughRetriesAndClearing(t *testing.T) {
	e, paths, vault, sender := harness(t)
	now := time.Now()
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Slack] = true })
	// A legacy Codex record and a Claude event may share external identifiers.
	codex := model.Event{ID: "same", TurnID: "same-turn", Kind: model.TurnEnded, CreatedAt: now}
	claude := codex
	claude.Agent = model.ClaudeCode
	claude.CreatedAt = now.Add(time.Second)
	sender.failOnce = true
	if err := paths.QueueEvent(&codex); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now)
	if err := paths.QueueEvent(&claude); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(time.Second))
	if len(e.Snapshot().Records) != 2 || len(sender.events) != 2 {
		t.Fatal("events from different assistants collapsed")
	}
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = false })
	e.Step(context.Background(), now.Add(time.Minute))
	if len(sender.events) != 3 || sender.events[2].AgentName() != "Codex" {
		t.Fatal("Claude disconnect stopped Codex retry")
	}
	s := e.Snapshot()
	if !s.LastEvents[model.Codex].Equal(now) || !s.LastEvents[model.ClaudeCode].Equal(now.Add(time.Second)) {
		t.Fatal("source connection timestamps mixed", s.LastEvents)
	}
	if err := e.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	e, err := New(paths, vault, sender)
	if err != nil {
		t.Fatal(err)
	}
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = true })
	if err := paths.QueueEvent(&claude); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now.Add(time.Minute))
	if len(sender.events) != 3 || len(e.Snapshot().Records) != 0 {
		t.Fatal("cleared Claude event sent twice after restart")
	}
	if !e.Snapshot().LastEvents[model.ClaudeCode].Equal(claude.CreatedAt) {
		t.Fatal("clear lost per-source connection evidence")
	}
}

func TestClaudePendingDeliveryStopsWhenOnlyClaudeIsDisconnected(t *testing.T) {
	e, paths, _, sender := harness(t)
	now := time.Now()
	sender.failOnce = true
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Slack] = true })
	event := model.Event{ID: "claude-pending", Agent: model.ClaudeCode, Kind: model.TurnFailed, CreatedAt: now}
	if err := paths.QueueEvent(&event); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now)
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.ClaudeAutoConnect = false })
	e.Step(context.Background(), now.Add(time.Minute))
	if len(sender.events) != 1 || e.Snapshot().Records[0].State() != model.Skipped {
		t.Fatal("Claude retry sent while disconnected")
	}
	queue(t, paths, "codex-after", now)
	e.Step(context.Background(), now)
	if len(sender.events) != 2 || sender.events[1].AgentName() != "Codex" {
		t.Fatal("Codex disconnected with Claude")
	}
}
