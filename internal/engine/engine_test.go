package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"callbell/internal/channels"
	"callbell/internal/model"
	"callbell/internal/storage"
)

type fakeVault struct {
	mu     sync.Mutex
	values map[model.Channel]model.Credentials
}

func (v *fakeVault) Get(ch model.Channel) (model.Credentials, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.values[ch]
	if !ok {
		return c, errors.New("missing")
	}
	return c, nil
}
func (v *fakeVault) Set(ch model.Channel, c model.Credentials) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[ch] = c
	return nil
}
func (v *fakeVault) Delete(ch model.Channel) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.values, ch)
	return nil
}

type fakeSender struct {
	sent     map[model.Channel]int
	failOnce bool
	events   []model.Event
}

func (s *fakeSender) Send(_ context.Context, ch model.Channel, _ model.Credentials, event model.Event) error {
	s.sent[ch]++
	s.events = append(s.events, event)
	if ch == model.Slack && s.failOnce {
		s.failOnce = false
		return &channels.Failure{Message: "offline", Retryable: true}
	}
	return nil
}
func harness(t *testing.T) (*Engine, storage.Paths, *fakeVault, *fakeSender) {
	t.Helper()
	p := storage.Paths{Root: t.TempDir()}
	v := &fakeVault{values: map[model.Channel]model.Credentials{model.Feishu: {Webhook: "sensitive-fixture"}, model.Slack: {Webhook: "sensitive-fixture"}}}
	s := &fakeSender{sent: map[model.Channel]int{}}
	e, err := New(p, v, s)
	if err != nil {
		t.Fatal(err)
	}
	return e, p, v, s
}
func queue(t *testing.T, p storage.Paths, id string, created time.Time) {
	t.Helper()
	if err := storage.WriteJSON(filepath.Join(p.Inbox(), id+".json"), model.Event{ID: id, Kind: model.TurnEnded, Project: "work", Summary: "private reply", CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
}

func TestPartialDeliveryRetryAndRestartDeduplication(t *testing.T) {
	e, p, v, s := harness(t)
	s.failOnce = true
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true; p.Channels[model.Slack] = true })
	now := time.Now()
	queue(t, p, "event1", now)
	e.Step(context.Background(), now)
	r := e.Snapshot().Records[0]
	if r.Deliveries[model.Feishu].State != model.Sent || r.Deliveries[model.Slack].State != model.Pending {
		t.Fatalf("%+v", r)
	}
	// A transient failure retries in the same run without duplicating success.
	e.Step(context.Background(), now.Add(time.Minute))
	if s.sent[model.Feishu] != 1 || s.sent[model.Slack] != 2 {
		t.Fatal("failed channel did not retry independently", s.sent)
	}
	e2, err := New(p, v, s)
	if err != nil {
		t.Fatal(err)
	}
	e2.Step(context.Background(), now.Add(time.Minute))
	if s.sent[model.Feishu] != 1 || s.sent[model.Slack] != 2 {
		t.Fatal("successful channel was sent twice", s.sent)
	}
	queue(t, p, "event1", now)
	e2.Step(context.Background(), now.Add(time.Minute))
	if len(e2.Snapshot().Records) != 1 || s.sent[model.Feishu] != 1 || s.sent[model.Slack] != 2 {
		t.Fatal("duplicate event was sent")
	}
	for _, event := range s.events {
		if event.Summary != "" {
			t.Fatal("summary sent without opt-in")
		}
	}
	b, _ := os.ReadFile(p.History())
	settings, _ := os.ReadFile(p.Settings())
	if strings.Contains(string(b)+string(settings), "sensitive-fixture") || strings.Contains(string(b), "private reply") {
		t.Fatal("private data leaked to disk")
	}
}
func TestUnconfiguredAndPausedEventsAreNotBackfilled(t *testing.T) {
	e, p, _, s := harness(t)
	now := time.Now()
	queue(t, p, "unconfigured", now)
	e.Step(context.Background(), now)
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true; p.Paused = true })
	queue(t, p, "paused", now)
	e.Step(context.Background(), now)
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Paused = false })
	e.Step(context.Background(), now)
	if len(s.events) != 0 {
		t.Fatal("skipped events were backfilled")
	}
	for _, r := range e.Snapshot().Records {
		if r.State() != model.Skipped || r.Note == "" {
			t.Fatal("missing skipped explanation")
		}
	}
}

func TestCompletionSourcesDeduplicateAcrossRestartAndDisconnect(t *testing.T) {
	e, p, v, s := harness(t)
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true })
	now := time.Now()
	first := &model.Event{ID: "hook", Kind: model.TurnEnded, TurnID: "unique-turn", SessionID: "root", CreatedAt: now}
	if err := p.QueueEvent(first); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now)
	e, err := New(p, v, s)
	if err != nil {
		t.Fatal(err)
	}
	// A fork's transcript ID can differ from the session ID supplied by Hooks.
	second := *first
	second.ID = "rollout"
	second.SessionID = "fork"
	if err = p.QueueEvent(&second); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now)
	if len(s.events) != 1 || len(e.Snapshot().Records) != 1 {
		t.Fatal("duplicate completion sent across sources")
	}
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.AutoConnect = false })
	second.ID = "disconnected"
	second.TurnID = "next-turn"
	if err = p.QueueEvent(&second); err != nil {
		t.Fatal(err)
	}
	e.Step(context.Background(), now)
	if len(s.events) != 1 {
		t.Fatal("cached hook sent after disconnect")
	}
}
func TestExpiredAndDisabledEventsAreNotSent(t *testing.T) {
	e, p, _, s := harness(t)
	now := time.Now()
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true })
	// Expiry still applies while the same process remains running.
	queue(t, p, "expired", now)
	e.Step(context.Background(), now.Add(25*time.Hour))
	if e.Snapshot().Records[0].State() != model.Expired || len(s.events) != 0 {
		t.Fatal("stale notification sent")
	}
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Kinds[model.TurnEnded] = false })
	queue(t, p, "disabled", now.Add(25*time.Hour))
	e.Step(context.Background(), now.Add(25*time.Hour))
	if len(s.events) != 0 {
		t.Fatal("disabled event sent")
	}
}
func TestCorruptSettingsAndHistoryArePreserved(t *testing.T) {
	p := storage.Paths{Root: t.TempDir()}
	_ = p.Prepare()
	_ = os.WriteFile(p.Settings(), []byte("broken"), 0600)
	if _, err := New(p, nil, nil); err == nil {
		t.Fatal("corrupt settings silently replaced")
	}
	b, _ := os.ReadFile(p.Settings())
	if string(b) != "broken" {
		t.Fatal("corrupt settings overwritten")
	}
	_ = os.Remove(p.Settings())
	_ = os.WriteFile(p.History(), []byte("broken"), 0600)
	if _, err := New(p, nil, nil); err == nil {
		t.Fatal("corrupt history silently replaced")
	}
}
func TestHistoryWriteFailureDoesNotLoseInbox(t *testing.T) {
	e, p, _, s := harness(t)
	_ = e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true })
	if err := os.Mkdir(p.History(), 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, p, "pending", now)
	e.Step(context.Background(), now)
	if _, err := os.Stat(filepath.Join(p.Inbox(), "pending.json")); err != nil {
		t.Fatal("uncommitted event removed")
	}
	if len(s.events) != 0 {
		t.Fatal("event sent before durable history")
	}
}

func TestNotificationLanguageFollowsSystemAndOverride(t *testing.T) {
	e, p, _, sender := harness(t)
	e.SetLocale("zh-Hans")
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Channels[model.Feishu] = true }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	queue(t, p, "chinese", now)
	e.Step(context.Background(), now)
	if len(sender.events) != 1 || !strings.Contains(sender.events[0].Message(), "本轮已结束") {
		t.Fatalf("wrong system language: %+v", sender.events)
	}
	if err := e.UpdatePreferences(func(p *model.Preferences) { p.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	queue(t, p, "english", now)
	e.Step(context.Background(), now)
	if len(sender.events) != 2 || !strings.Contains(sender.events[1].Message(), "Turn ended") {
		t.Fatalf("explicit language ignored: %+v", sender.events)
	}
	if err := e.TestChannel(context.Background(), model.Feishu); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sender.events[2].Message(), "Connection test") {
		t.Fatal("test notification language differs")
	}
}
