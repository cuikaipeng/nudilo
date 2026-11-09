package model

import "time"

type ChannelHealth struct {
	State           State
	LastSuccess     time.Time
	LastTestSuccess time.Time
	LastRealSuccess time.Time
	Delivery        Delivery
}

// Health is scoped to the currently saved destination. Clearing history also
// clears delivery evidence; saved credentials alone never imply a successful send.
func Health(records []Record, ch Channel, since time.Time) ChannelHealth {
	h := ChannelHealth{}
	var latest time.Time
	for _, r := range records {
		d, ok := r.Deliveries[ch]
		if !ok || r.Cleared || r.Event.CreatedAt.Before(since) || d.State == Skipped {
			continue
		}
		at := d.LastAttempt
		if at.IsZero() {
			at = r.Event.CreatedAt
		}
		if latest.IsZero() || at.After(latest) {
			latest = at
			h.State = d.State
			h.Delivery = d
		}
		if d.State == Sent {
			if at.After(h.LastSuccess) {
				h.LastSuccess = at
			}
			if r.Event.Kind == Test && at.After(h.LastTestSuccess) {
				h.LastTestSuccess = at
			}
			if r.Event.Kind != Test && at.After(h.LastRealSuccess) {
				h.LastRealSuccess = at
			}
		}
	}
	return h
}
