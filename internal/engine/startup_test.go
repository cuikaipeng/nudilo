package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"callbell/internal/locale"
	"callbell/internal/model"
	"callbell/internal/storage"
)

func TestStartupSkipsOldEventsButSendsNewEvents(t *testing.T) {
	for _, agent := range []model.Agent{"", model.Codex, model.ClaudeCode} {
		for _, kind := range []model.Kind{model.TurnEnded, model.InputRequired, model.ApprovalRequired, model.TurnFailed} {
			t.Run(fmt.Sprintf("%s/%s", agent, kind), func(t *testing.T) {
				e, paths, vault, sender := harness(t)
				if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true }); err != nil {
					t.Fatal(err)
				}
				old := model.Event{ID: "offline", Agent: agent, Kind: kind, CreatedAt: time.Now()}
				if err := paths.QueueEvent(&old); err != nil {
					t.Fatal(err)
				}
				e, err := New(paths, vault, sender)
				if err != nil {
					t.Fatal(err)
				}
				// Events between startup and the first worker pass must not be lost.
				for _, offset := range []time.Duration{0, time.Nanosecond} {
					fresh := model.Event{ID: fmt.Sprintf("fresh-%d", offset), Agent: agent, Kind: kind, CreatedAt: e.startedAt.Add(offset)}
					if err := paths.QueueEvent(&fresh); err != nil {
						t.Fatal(err)
					}
				}
				e.Step(context.Background(), time.Now())
				if len(sender.events) != 2 || len(e.Snapshot().Records) != 3 {
					t.Fatal("startup replayed old events or lost new ones", sender.events)
				}
				for _, r := range e.Snapshot().Records {
					if r.Event.ID == "offline" {
						if r.State() != model.Skipped || r.Note != skippedBeforeStartup || len(r.Deliveries) != 0 {
							t.Fatal("offline event did not retain its skip reason", r)
						}
					} else if r.State() != model.Sent {
						t.Fatal("fresh event was not delivered", r)
					}
				}
			})
		}
	}
}

func TestStartupCutoffSurvivesMultipleBatchesAndDelayedQueueWrites(t *testing.T) {
	e, paths, vault, sender := harness(t)
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true }); err != nil {
		t.Fatal(err)
	}
	old := time.Now()
	for i := 0; i < 201; i++ {
		queue(t, paths, fmt.Sprintf("offline-%03d", i), old)
	}
	e, err := New(paths, vault, sender)
	if err != nil {
		t.Fatal(err)
	}
	queue(t, paths, "z-fresh", time.Now())
	e.Step(context.Background(), time.Now())
	// A helper can finish writing after startup even though its event is older.
	queue(t, paths, "delayed-old", e.startedAt.Add(-time.Nanosecond))
	for i := 0; i < 3; i++ {
		e.Step(context.Background(), time.Now())
	}
	if len(sender.events) != 1 || sender.events[0].ID != "z-fresh" || len(e.Snapshot().Records) != 203 {
		t.Fatal("backlog escaped the startup cutoff or blocked fresh delivery", len(sender.events), len(e.Snapshot().Records))
	}
	for _, r := range e.Snapshot().Records {
		if r.Event.ID != "z-fresh" && (r.State() != model.Skipped || r.Note != skippedBeforeStartup) {
			t.Fatal("old queued event missing its skip reason", r)
		}
	}
	files, err := os.ReadDir(paths.Inbox())
	if err != nil || len(files) != 0 {
		t.Fatal("processed backlog left in inbox", err)
	}
}

func TestRestartCancelsPersistedPendingDeliveries(t *testing.T) {
	e, paths, vault, sender := harness(t)
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true; p.Channels[model.Slack] = true }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	message := locale.M("offline")
	pending := model.Delivery{State: model.Pending, Attempts: 1, LastAttempt: now.Add(-time.Minute), NextAttempt: now.Add(time.Hour), Error: "offline", ErrorMessage: &message}
	records := []model.Record{
		{Event: model.Event{ID: "mixed", Kind: model.TurnEnded, CreatedAt: now.Add(-time.Minute)}, Deliveries: map[model.Channel]model.Delivery{
			model.Slack:    pending,
			model.Feishu:   {State: model.Sent, Attempts: 1, LastAttempt: now.Add(-time.Minute)},
			model.Telegram: {State: model.Failed, Attempts: 3, Error: "failed"},
			model.DingTalk: {State: model.Expired},
			model.Lark:     {State: model.Skipped, Error: "disabled"},
		}},
		// Loaded retries are stale even if the system clock moved backwards.
		{Event: model.Event{ID: "future-timestamp", Agent: model.ClaudeCode, Kind: model.InputRequired, CreatedAt: now.Add(time.Hour)}, Deliveries: map[model.Channel]model.Delivery{model.Slack: {State: model.Pending}}},
	}
	if err := storage.WriteJSON(paths.History(), records); err != nil {
		t.Fatal(err)
	}
	for restart := 0; restart < 2; restart++ {
		e, err := New(paths, vault, sender)
		if err != nil {
			t.Fatal(err)
		}
		var saved []model.Record
		if err := storage.ReadJSON(paths.History(), &saved); err != nil {
			t.Fatal(err)
		}
		for _, r := range saved {
			d := r.Deliveries[model.Slack]
			if d.State != model.Skipped || d.Error != skippedBeforeStartup || d.ErrorMessage != nil || !d.NextAttempt.IsZero() {
				t.Fatal("startup did not persist cancellation", r)
			}
			if r.Event.ID == "mixed" {
				if d.Attempts != pending.Attempts || !d.LastAttempt.Equal(pending.LastAttempt) {
					t.Fatal("cancellation erased previous attempt evidence", d)
				}
				for ch, original := range records[0].Deliveries {
					if ch != model.Slack {
						got := r.Deliveries[ch]
						if got.State != original.State || got.Attempts != original.Attempts || got.Error != original.Error || got.ErrorMessage != nil || !got.LastAttempt.Equal(original.LastAttempt) || !got.NextAttempt.Equal(original.NextAttempt) {
							t.Fatal("startup changed a finished delivery", ch, got)
						}
					}
				}
			}
		}
		queue(t, paths, "mixed", time.Now())
		e.Step(context.Background(), now.Add(2*time.Hour))
		if len(sender.events) != 0 || len(e.Snapshot().Records) != 2 {
			t.Fatal("restart replayed or duplicated persisted events", sender.events)
		}
	}
}

func TestManualRetryOfOldFailureStillWorks(t *testing.T) {
	e, paths, vault, sender := harness(t)
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Slack] = true }); err != nil {
		t.Fatal(err)
	}
	record := model.Record{Event: model.Event{ID: "failed", Kind: model.TurnEnded, CreatedAt: time.Now().Add(-time.Minute)}, Deliveries: map[model.Channel]model.Delivery{model.Slack: {State: model.Failed, Attempts: 3}}}
	if err := storage.WriteJSON(paths.History(), []model.Record{record}); err != nil {
		t.Fatal(err)
	}
	e, err := New(paths, vault, sender)
	if err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), time.Now())
	if len(sender.events) != 0 {
		t.Fatal("old failure retried without user action")
	}
	if err := e.Retry("failed"); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), time.Now())
	if len(sender.events) != 1 || e.Snapshot().Records[0].State() != model.Sent {
		t.Fatal("manual retry of an old failure was blocked")
	}
}

func TestStartupSkipSurvivesHistoryWriteFailure(t *testing.T) {
	e, paths, _, sender := harness(t)
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true }); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.History(), 0700); err != nil {
		t.Fatal(err)
	}
	queue(t, paths, "a-old", e.startedAt.Add(-time.Second))
	queue(t, paths, "z-new", time.Now())
	e.Step(context.Background(), time.Now())
	if _, err := os.Stat(filepath.Join(paths.Inbox(), "a-old.json")); err != nil || len(sender.events) != 0 {
		t.Fatal("failed acknowledgment removed or sent the queued event", err)
	}
	if err := os.Remove(paths.History()); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), time.Now())
	if len(sender.events) != 1 || sender.events[0].ID != "z-new" || e.Snapshot().Error != "" {
		t.Fatal("storage recovery replayed old event or blocked fresh delivery", sender.events)
	}
	var saved []model.Record
	if err := storage.ReadJSON(paths.History(), &saved); err != nil || len(saved) != 2 || saved[1].State() != model.Skipped {
		t.Fatal("skip was not durably recorded", saved, err)
	}
}

func TestStartupCancellationMustPersistBeforeWorkerStarts(t *testing.T) {
	_, paths, vault, sender := harness(t)
	record := model.Record{Event: model.Event{ID: "pending", Kind: model.TurnEnded, CreatedAt: time.Now()}, Deliveries: map[model.Channel]model.Delivery{model.Slack: {State: model.Pending}}}
	if err := storage.WriteJSON(paths.History(), []model.Record{record}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(paths.History())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(paths.Root, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(paths.Root, 0700) })
	if probe, err := os.CreateTemp(paths.Root, "permission-probe"); err == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem permits writes despite read-only directory mode")
	}
	if e, err := New(paths, vault, sender); err == nil || e != nil {
		t.Fatal("worker was returned before cancellation could be saved")
	}
	after, err := os.ReadFile(paths.History())
	if err != nil || !bytes.Equal(before, after) || len(sender.events) != 0 {
		t.Fatal("failed startup changed history or sent a message", err)
	}
}
