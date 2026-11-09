// Package updates coordinates checks and signed update preparation without
// stopping the notification worker. The user chooses when to restart.
package updates

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Phase string

const (
	Idle           Phase = "idle"
	Checking       Phase = "checking"
	Current        Phase = "current"
	Available      Phase = "available"
	Downloading    Phase = "downloading"
	Ready          Phase = "ready"
	CheckFailed    Phase = "check_failed"
	DownloadFailed Phase = "download_failed"
	NotPublished   Phase = "not_published"
)

var ErrNotPublished = errors.New("no public update is available")

type Release struct {
	Version string
	Install func(context.Context, func(int64, int64)) error
}
type Checker func(context.Context) (*Release, error)
type State struct {
	Phase        Phase
	Version      string
	CanInstall   bool
	Bytes, Total int64
	Checked      time.Time
}

// All state is protected by mu; UI callbacks run outside the lock. Only one
// check or installation may be in flight, including overlapping manual checks.
type Controller struct {
	mu                   sync.Mutex
	ctx                  context.Context
	check                Checker
	changed              func()
	automatic            bool
	state                State
	release              *Release
	startDelay, interval time.Duration
}

func New(ctx context.Context, check Checker, automatic bool, changed func()) *Controller {
	return &Controller{ctx: ctx, check: check, automatic: automatic, changed: changed,
		state: State{Phase: Idle}, startDelay: 30 * time.Second, interval: 6 * time.Hour}
}
func (c *Controller) Snapshot() State { c.mu.Lock(); defer c.mu.Unlock(); return c.state }
func (c *Controller) notify() {
	if c.changed != nil && c.ctx.Err() == nil {
		c.changed()
	}
}
func (c *Controller) SetAutomatic(on bool) {
	c.mu.Lock()
	changed := c.automatic != on
	c.automatic = on
	available := c.state.Phase == Available
	c.mu.Unlock()
	if changed && on {
		if available {
			c.Download()
		} else {
			c.Check(false)
		}
	}
}

// Turning automatic updates off prevents new checks/downloads. A preparation
// already in progress completes; a staged update is never silently discarded.
func (c *Controller) Run() {
	timer := time.NewTimer(c.startDelay)
	defer timer.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-timer.C:
			c.Check(false)
			timer.Reset(c.interval)
		}
	}
}
func (c *Controller) Check(manual bool) {
	c.mu.Lock()
	if c.ctx.Err() != nil || (!manual && !c.automatic) || c.state.Phase == Checking || c.state.Phase == Downloading || c.state.Phase == Ready {
		c.mu.Unlock()
		return
	}
	c.state.Phase = Checking
	c.mu.Unlock()
	c.notify()
	go func() {
		ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
		defer cancel()
		up, err := c.check(ctx)
		c.mu.Lock()
		if c.ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
		c.state = State{Phase: Current, Checked: time.Now()}
		c.release = nil
		switch {
		case errors.Is(err, ErrNotPublished):
			c.state.Phase = NotPublished
		case err != nil:
			c.state.Phase = CheckFailed
		case up != nil:
			c.release = up
			c.state.Phase, c.state.Version, c.state.CanInstall = Available, up.Version, up.Install != nil
		}
		autoDownload := c.automatic && c.state.Phase == Available && c.state.CanInstall
		c.mu.Unlock()
		c.notify()
		if autoDownload {
			c.download(true)
		}
	}()
}
func (c *Controller) Download() { c.download(false) }
func (c *Controller) download(automatic bool) {
	c.mu.Lock()
	if c.ctx.Err() != nil || (automatic && !c.automatic) || c.release == nil || c.release.Install == nil || (c.state.Phase != Available && c.state.Phase != DownloadFailed) {
		c.mu.Unlock()
		return
	}
	up := c.release
	c.state.Phase, c.state.Bytes, c.state.Total = Downloading, 0, 0
	c.mu.Unlock()
	c.notify()
	go func() {
		ctx, cancel := context.WithTimeout(c.ctx, 10*time.Minute)
		defer cancel()
		var last time.Time
		err := up.Install(ctx, func(n, size int64) {
			c.mu.Lock()
			c.state.Bytes, c.state.Total = max(n, 0), max(size, 0)
			notify := time.Since(last) >= 100*time.Millisecond || n == size
			if notify {
				last = time.Now()
			}
			c.mu.Unlock()
			if notify {
				c.notify()
			}
		})
		c.mu.Lock()
		if c.ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
		if err != nil {
			c.state.Phase = DownloadFailed
		} else {
			c.state.Phase = Ready
		}
		c.mu.Unlock()
		c.notify()
	}()
}
