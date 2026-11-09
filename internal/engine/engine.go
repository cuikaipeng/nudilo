package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"callbell/internal/channels"
	"callbell/internal/locale"
	"callbell/internal/model"
	"callbell/internal/storage"
)

type Vault interface {
	Get(model.Channel) (model.Credentials, error)
	Set(model.Channel, model.Credentials) error
	Delete(model.Channel) error
}
type Sender interface {
	Send(context.Context, model.Channel, model.Credentials, model.Event) error
}
type Snapshot struct {
	Preferences model.Preferences
	Records     []model.Record
	Error       string
	LastEvent   time.Time
	LastEvents  map[model.Agent]time.Time
}

const skippedBeforeStartup = "This notification is from before Nudilo started and was skipped."

type Engine struct {
	mu            sync.Mutex
	paths         storage.Paths
	startedAt     time.Time
	pref          model.Preferences
	records       []model.Record
	vault         Vault
	sender        Sender
	lastError     string
	language      string
	onChange      func()
	wake          chan struct{}
	stepMu        sync.Mutex
	clearRequests atomic.Int32
}

func New(paths storage.Paths, vault Vault, sender Sender) (*Engine, error) {
	startedAt := time.Now()
	if err := paths.Prepare(); err != nil {
		return nil, err
	}
	pref, err := paths.Preferences()
	if err != nil {
		return nil, err
	}
	e := &Engine{paths: paths, startedAt: startedAt, pref: pref, vault: vault, sender: sender, wake: make(chan struct{}, 1)}
	if err = storage.ReadJSON(paths.History(), &e.records); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, locale.M("Cannot read history. The original file was preserved: {detail}", err)
	}
	// Retries belong to the process that received the event. Persist their
	// cancellation before returning an engine that can start sending.
	changed := false
	for i := range e.records {
		for ch, d := range e.records[i].Deliveries {
			if d.State == model.Pending {
				d.State, d.Error, d.ErrorMessage = model.Skipped, skippedBeforeStartup, nil
				d.NextAttempt = time.Time{}
				e.records[i].Deliveries[ch] = d
				changed = true
			}
		}
	}
	if changed {
		if err := e.persistLocked(); err != nil {
			return nil, locale.M("Cannot save notification history. Check disk space.")
		}
	}
	return e, nil
}
func (e *Engine) OnChange(fn func()) { e.mu.Lock(); e.onChange = fn; e.mu.Unlock() }
func (e *Engine) changed() {
	e.mu.Lock()
	fn := e.onChange
	e.mu.Unlock()
	if fn != nil {
		fn()
	}
}
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Clone through JSON: the UI never shares mutable maps with the worker.
	b, _ := json.Marshal(struct {
		Preferences model.Preferences
		Records     []model.Record
	}{e.pref, e.records})
	var s Snapshot
	_ = json.Unmarshal(b, &s)
	s.Error = e.lastError
	s.LastEvents = map[model.Agent]time.Time{}
	for _, r := range s.Records {
		agent := r.Event.Agent.Normalized()
		if r.Event.Kind != model.Test && r.Event.CreatedAt.After(s.LastEvents[agent]) {
			s.LastEvents[agent] = r.Event.CreatedAt
		}
		if r.Event.Kind != model.Test && r.Event.CreatedAt.After(s.LastEvent) {
			s.LastEvent = r.Event.CreatedAt
		}
	}
	visible := s.Records[:0]
	for _, r := range s.Records {
		if !r.Cleared {
			visible = append(visible, r)
		}
	}
	s.Records = visible
	return s
}
func (e *Engine) UpdatePreferences(edit func(*model.Preferences)) error {
	e.mu.Lock()
	b, _ := json.Marshal(e.pref)
	var updated model.Preferences
	_ = json.Unmarshal(b, &updated)
	updated.Normalize()
	edit(&updated)
	updated.Normalize()
	err := updated.ValidateNotificationContent()
	if err == nil {
		err = updated.ValidateRoutes()
	}
	if err == nil {
		err = storage.WriteJSON(e.paths.Settings(), updated)
	}
	if err == nil {
		e.pref = updated
	}
	e.mu.Unlock()
	if err == nil {
		select {
		case e.wake <- struct{}{}:
		default:
		}
		e.changed()
	}
	return err
}
func (e *Engine) SaveChannel(ch model.Channel, c model.Credentials) error {
	e.stepMu.Lock()
	defer e.stepMu.Unlock()
	c.Normalize(ch)
	if err := channels.Validate(ch, c); err != nil {
		return err
	}
	// Retain the other mode when it was not included in the submitted setup.
	e.mu.Lock()
	exists := e.pref.Channels[ch]
	e.mu.Unlock()
	if exists {
		old, err := e.vault.Get(ch)
		if err != nil && !errors.Is(err, storage.ErrCredentialsMissing) {
			return errors.New("Cannot read channel credentials. Save your configuration again.")
		}
		old.Normalize(ch)
		if !sameDestination(ch, old, c) {
			e.mu.Lock()
			for i := range e.records {
				d, ok := e.records[i].Deliveries[ch]
				if ok && d.State == model.Pending {
					d.State, d.Error, d.ErrorMessage = model.Skipped, "Destination changed. This pending notification was not forwarded.", nil
					e.records[i].Deliveries[ch] = d
				}
			}
			err := e.persistLocked()
			e.mu.Unlock()
			e.changed()
			if err != nil {
				return err
			}
		}
		if c.Mode == model.Direct {
			if c.Webhook == "" {
				c.Webhook = old.Webhook
			}
			if c.Secret == "" {
				c.Secret = old.Secret
			}
			if c.ChatID == "" {
				c.ChatID = old.ChatID
			}
		} else {
			if c.AppID == "" {
				c.AppID = old.AppID
			}
			if c.AppSecret == "" {
				c.AppSecret = old.AppSecret
			}
			if ch == model.Slack && c.Token == "" {
				c.Token = old.Token
			}
			// A recipient from another app/bot must not be restored after the
			// editor deliberately cleared an outdated account binding.
			sameApp := c.AppID == old.AppID && c.AppSecret == old.AppSecret
			if ch == model.Slack || ch == model.Telegram {
				sameApp = c.Token == old.Token
			}
			if sameApp {
				if c.RobotCode == "" {
					c.RobotCode = old.RobotCode
				}
				if c.RecipientID == "" {
					c.RecipientID, c.RecipientType = old.RecipientID, old.RecipientType
				}
			}
		}
	}
	if err := e.vault.Set(ch, c); err != nil {
		return locale.M("Cannot save local credentials: {detail}", err)
	}
	return e.UpdatePreferences(func(p *model.Preferences) {
		p.Channels[ch] = true
		p.ChannelSince[ch] = time.Now()
		if p.ChannelModes == nil {
			p.ChannelModes = map[model.Channel]model.DeliveryMode{}
		}
		p.ChannelModes[ch] = c.Mode
	})
}

func sameDestination(ch model.Channel, a, b model.Credentials) bool {
	if a.Mode != b.Mode {
		return false
	}
	if ch == model.Telegram {
		if a.Mode == model.Direct {
			return a.Token == b.Token && a.RecipientID == b.RecipientID
		}
		return a.Token == b.Token && a.ChatID == b.ChatID
	}
	if a.Mode == model.Group {
		return a.Webhook == b.Webhook
	}
	if a.RecipientID != b.RecipientID {
		return false
	}
	switch ch {
	case model.Slack:
		return a.Token == b.Token
	case model.DingTalk:
		return a.AppID == b.AppID && a.RobotCode == b.RobotCode
	default:
		return a.AppID == b.AppID && a.RecipientType == b.RecipientType
	}
}
func (e *Engine) RemoveChannel(ch model.Channel) error {
	if err := e.UpdatePreferences(func(p *model.Preferences) {
		delete(p.Channels, ch)
		delete(p.ChannelModes, ch)
		delete(p.ChannelSince, ch)
	}); err != nil {
		return err
	}
	return e.vault.Delete(ch)
}
func (e *Engine) Credentials(ch model.Channel) (model.Credentials, error) { return e.vault.Get(ch) }
func (e *Engine) TestChannel(ctx context.Context, ch model.Channel) error {
	e.stepMu.Lock()
	defer e.stepMu.Unlock()
	c, err := e.vault.Get(ch)
	if err != nil {
		return errors.New("Cannot read channel credentials. Save your configuration again.")
	}
	now := time.Now()
	event := model.Event{ID: fmt.Sprintf("test-%d", now.UnixNano()), Kind: model.Test, Project: "Nudilo", CreatedAt: now}
	e.mu.Lock()
	event = e.pref.PrepareNotification(event, e.language)
	e.mu.Unlock()
	err = e.sender.Send(ctx, ch, c, event)
	d := model.Delivery{State: model.Sent, Attempts: 1, LastAttempt: time.Now()}
	if err != nil {
		d.State = model.Failed
		d.Error = err.Error()
		message := locale.FromError(err)
		d.ErrorMessage = &message
	}
	e.mu.Lock()
	e.records = append([]model.Record{{Event: event, Deliveries: map[model.Channel]model.Delivery{ch: d}}}, e.records...)
	persistErr := e.persistLocked()
	e.mu.Unlock()
	e.changed()
	if persistErr != nil {
		return persistErr
	}
	return err
}
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		e.Step(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-e.wake:
		}
	}
}
func (e *Engine) Step(ctx context.Context, now time.Time) {
	e.stepMu.Lock()
	defer e.stepMu.Unlock()
	if e.clearRequests.Load() > 0 {
		return
	}
	if err := e.resumeIfDue(now); err != nil {
		return
	}
	if err := e.ingest(now); err != nil {
		e.changed()
		return
	}
	for {
		if ctx.Err() != nil || e.clearRequests.Load() > 0 {
			return
		}
		e.mu.Lock()
		if e.pref.Paused {
			e.mu.Unlock()
			break
		}
		index := -1
		var channel model.Channel
		for i := range e.records {
			for ch, d := range e.records[i].Deliveries {
				if d.State == model.Pending && !now.Before(d.NextAttempt) {
					index = i
					channel = ch
					break
				}
			}
			if index >= 0 {
				break
			}
		}
		if index < 0 {
			e.mu.Unlock()
			break
		}
		r := e.records[index]
		d := r.Deliveries[channel]
		if now.Sub(r.Event.CreatedAt) > 24*time.Hour {
			d.State = model.Expired
			d.Error = "Older than 24 hours. No further attempts."
		} else if !e.pref.AgentEnabled(r.Event.Agent) {
			d.State = model.Skipped
			d.Error = "This assistant is disconnected."
		} else if !e.pref.Channels[channel] || !e.pref.Kinds[r.Event.Kind] {
			d.State = model.Skipped
			d.Error = "This channel or notification type is disabled."
		} else if !slices.Contains(e.pref.Destinations(r.Event), channel) {
			d.State = model.Skipped
			d.Error = "Project routing changed. This notification was not forwarded."
		}
		if d.State != model.Pending {
			d.ErrorMessage = nil
			e.records[index].Deliveries[channel] = d
			err := e.persistLocked()
			e.mu.Unlock()
			if err != nil {
				break
			}
			e.changed()
			continue
		}
		event := e.pref.PrepareNotification(r.Event, e.language)
		e.mu.Unlock()
		credentials, err := e.vault.Get(channel)
		if err != nil {
			err = errors.New("Channel credentials are unavailable. Save the configuration again.")
		} else {
			err = e.sender.Send(ctx, channel, credentials, event)
		}
		if ctx.Err() != nil {
			return
		}
		e.mu.Lock()
		// Indices may change while the user sends a test notification.
		for i := range e.records {
			if e.records[i].Event.ID == event.ID && e.records[i].Event.Agent.Normalized() == event.Agent.Normalized() {
				index = i
				break
			}
		}
		d.Attempts++
		d.LastAttempt = time.Now()
		d.Error = ""
		d.ErrorMessage = nil
		d.State = model.Sent
		if err != nil {
			d.Error = err.Error()
			message := locale.FromError(err)
			d.ErrorMessage = &message
			d.State = model.Failed
			if delay, retry := channels.Retry(err, d.Attempts); retry {
				d.State = model.Pending
				d.NextAttempt = time.Now().Add(delay)
			}
		}
		e.records[index].Deliveries[channel] = d
		persistErr := e.persistLocked()
		e.mu.Unlock()
		e.changed()
		if persistErr != nil {
			return
		}
	}
}
func (e *Engine) ingest(now time.Time) error {
	files, err := os.ReadDir(e.paths.Inbox())
	if err != nil {
		e.setError("Cannot read the local event queue.")
		return err
	}
	e.clearError("Cannot read the local event queue.")
	count := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		count++
		if count > 100 {
			break
		}
		path := filepath.Join(e.paths.Inbox(), file.Name())
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.Size() > 32768 {
			e.quarantine(path)
			continue
		}
		var event model.Event
		if storage.ReadJSON(path, &event) != nil || event.ID == "" || event.CreatedAt.IsZero() {
			e.quarantine(path)
			continue
		}
		if (event.Agent.Normalized() != model.Codex && event.Agent != model.ClaudeCode) || (event.Kind != model.TurnEnded && event.Kind != model.InputRequired && event.Kind != model.ApprovalRequired && event.Kind != model.TurnFailed) {
			e.quarantine(path)
			continue
		}
		e.mu.Lock()
		exists := false
		for _, r := range e.records {
			if r.Event.Agent.Normalized() == event.Agent.Normalized() && (r.Event.ID == event.ID || (event.Kind == model.TurnEnded && event.TurnID != "" && r.Event.Kind == model.TurnEnded && r.Event.TurnID == event.TurnID)) {
				exists = true
				break
			}
		}
		if !exists {
			if !e.pref.IncludeSummary {
				event.Summary = ""
			}
			r := model.Record{Event: event, Deliveries: map[model.Channel]model.Delivery{}}
			// Keep the launch cutoff for every batch, including old events
			// written by a delayed helper after the worker has already started.
			if event.CreatedAt.Before(e.startedAt) {
				r.Note = skippedBeforeStartup
			} else if !e.pref.AgentEnabled(event.Agent) {
				r.Note = "This assistant was disconnected when the event arrived."
			} else if e.pref.PausedFor(event.CreatedAt) {
				r.Note = "Notifications were paused when this event arrived."
			} else if !e.pref.Kinds[event.Kind] {
				r.Note = "This notification type is disabled."
			} else {
				for _, ch := range e.pref.Destinations(event) {
					state := model.Pending
					if now.Sub(event.CreatedAt) > 24*time.Hour {
						state = model.Expired
					}
					r.Deliveries[ch] = model.Delivery{State: state}
				}
				if len(r.Deliveries) == 0 {
					r.Note = "No enabled channel matched this notification. Check project rules and channel setup."
				}
			}
			e.records = append([]model.Record{r}, e.records...)
		}
		err = e.persistLocked()
		e.mu.Unlock()
		if err != nil {
			return err
		}
		_ = os.Remove(path)
		e.changed()
	}
	return nil
}

func (e *Engine) quarantine(path string) {
	dir := filepath.Join(e.paths.Root, "rejected")
	err := os.MkdirAll(dir, 0700)
	if err == nil {
		err = os.Rename(path, filepath.Join(dir, fmt.Sprintf("%s-%d.bad", filepath.Base(path), time.Now().UnixNano())))
	}
	if err != nil {
		e.setError("An invalid event could not be moved out of the queue. Check folder permissions and disk space.")
		return
	}
	e.setError("An invalid event was moved to the rejected folder. Other notifications continue.")
}
func (e *Engine) Retry(id string) error {
	e.mu.Lock()
	for i := range e.records {
		if e.records[i].Event.ID == id {
			for ch, d := range e.records[i].Deliveries {
				if d.State == model.Failed {
					d.State = model.Pending
					d.Attempts = 0
					d.NextAttempt = time.Time{}
					e.records[i].Deliveries[ch] = d
				}
			}
		}
	}
	err := e.persistLocked()
	e.mu.Unlock()
	e.changed()
	select {
	case e.wake <- struct{}{}:
	default:
	}
	return err
}

// ClearHistory cancels remaining retries and removes notification content.
// The worker cannot restore an in-flight record after the clear. Minimal
// tombstones remain within the existing retention bound so a delayed second
// source for the same turn does not send the notification again.
func (e *Engine) ClearHistory() error {
	// Let the current send finish, then yield the worker before it starts
	// another queued delivery. Waiting for its whole batch could take minutes.
	e.clearRequests.Add(1)
	defer e.clearRequests.Add(-1)
	e.stepMu.Lock()
	defer e.stepMu.Unlock()
	e.mu.Lock()
	cleared := make([]model.Record, 0, len(e.records))
	for _, r := range e.records {
		if r.Event.Kind == model.Test {
			continue
		}
		cleared = append(cleared, model.Record{Cleared: true, Event: model.Event{
			ID: r.Event.ID, Agent: r.Event.Agent, Kind: r.Event.Kind, TurnID: r.Event.TurnID, CreatedAt: r.Event.CreatedAt,
		}})
		if len(cleared) == 500 {
			break
		}
	}
	// Commit before replacing memory: a failed write must preserve the list.
	err := storage.WriteJSON(e.paths.History(), cleared)
	if err == nil {
		e.records = cleared
		if e.lastError == "Cannot save notification history. Check disk space." {
			e.lastError = ""
		}
	}
	e.mu.Unlock()
	if err != nil {
		return locale.M("Could not clear activity. Check folder permissions and disk space.")
	}
	e.changed()
	return nil
}

func (e *Engine) persistLocked() error {
	sort.SliceStable(e.records, func(i, j int) bool { return e.records[i].Event.CreatedAt.After(e.records[j].Event.CreatedAt) })
	if len(e.records) > 500 {
		kept := make([]model.Record, 0, 500)
		for i, r := range e.records {
			if i < 500 || r.State() == model.Pending {
				kept = append(kept, r)
			}
		}
		e.records = kept
	}
	if err := storage.WriteJSON(e.paths.History(), e.records); err != nil {
		e.lastError = "Cannot save notification history. Check disk space."
		return err
	}
	if e.lastError == "Cannot save notification history. Check disk space." {
		e.lastError = ""
	}
	return nil
}
func (e *Engine) setError(message string) {
	e.mu.Lock()
	e.lastError = message
	e.mu.Unlock()
	e.changed()
}

func (e *Engine) clearError(message string) {
	e.mu.Lock()
	changed := e.lastError == message
	if changed {
		e.lastError = ""
	}
	e.mu.Unlock()
	if changed {
		e.changed()
	}
}

// SetLocale receives the resolved system language from the native runtime.
func (e *Engine) SetLocale(language string) { e.mu.Lock(); e.language = language; e.mu.Unlock() }
