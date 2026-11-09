package updates

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func awaitState(t *testing.T, c *Controller, want Phase) State {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("wanted %s, got %+v", want, c.Snapshot())
		case <-tick.C:
			if s := c.Snapshot(); s.Phase == want {
				return s
			}
		}
	}
}
func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
func TestAutomaticPreparationAndManualOptOut(t *testing.T) {
	for _, automatic := range []bool{true, false} {
		var installs atomic.Int32
		c := New(testContext(t), func(context.Context) (*Release, error) {
			return &Release{Version: "1.2.0", Install: func(ctx context.Context, progress func(int64, int64)) error {
				installs.Add(1)
				progress(12, 12)
				return nil
			}}, nil
		}, automatic, nil)
		c.Check(true)
		if !automatic {
			awaitState(t, c, Available)
			if installs.Load() != 0 {
				t.Fatal("manual mode installed without a request")
			}
			c.Download()
		}
		s := awaitState(t, c, Ready)
		if installs.Load() != 1 || s.Version != "1.2.0" || s.Bytes != 12 {
			t.Fatal(s, installs.Load())
		}
		c.Check(true)
		c.Download()
		if installs.Load() != 1 {
			t.Fatal("prepared update was overwritten")
		}
	}
}
func TestOneCheckInFlightAndDisableBeforeDownload(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	var checks, installs atomic.Int32
	c := New(testContext(t), func(context.Context) (*Release, error) {
		checks.Add(1)
		close(started)
		<-finish
		return &Release{Version: "1.0.0", Install: func(context.Context, func(int64, int64)) error { installs.Add(1); return nil }}, nil
	}, true, nil)
	c.Check(true)
	<-started
	for range 10 {
		c.Check(true)
	}
	c.SetAutomatic(false)
	close(finish)
	awaitState(t, c, Available)
	if checks.Load() != 1 || installs.Load() != 0 {
		t.Fatal("duplicate check or unwanted automatic install")
	}
	c.SetAutomatic(true)
	awaitState(t, c, Ready)
}
func TestFailureCanRetryAndNeverBecomesReady(t *testing.T) {
	var tries atomic.Int32
	c := New(testContext(t), func(context.Context) (*Release, error) {
		return &Release{Version: "1.0.0", Install: func(context.Context, func(int64, int64)) error {
			if tries.Add(1) == 1 {
				return errors.New("invalid signature")
			}
			return nil
		}}, nil
	}, false, nil)
	c.Check(true)
	awaitState(t, c, Available)
	c.Download()
	awaitState(t, c, DownloadFailed)
	c.Download()
	awaitState(t, c, Ready)
	if tries.Load() != 2 {
		t.Fatal(tries.Load())
	}
}
func TestNoReleaseAndNetworkErrorsAreNotUpToDate(t *testing.T) {
	for _, tc := range []struct {
		err   error
		phase Phase
	}{{ErrNotPublished, NotPublished}, {errors.New("offline"), CheckFailed}, {nil, Current}} {
		c := New(testContext(t), func(context.Context) (*Release, error) { return nil, tc.err }, false, nil)
		c.Check(true)
		awaitState(t, c, tc.phase)
	}
}
func TestReadOnlyUpdateAndCanceledCheckNeverInstall(t *testing.T) {
	c := New(testContext(t), func(context.Context) (*Release, error) { return &Release{Version: "1.0.0"}, nil }, true, nil)
	c.Check(true)
	s := awaitState(t, c, Available)
	if s.CanInstall {
		t.Fatal("read-only installation offered self-update")
	}
	c.Download()
	if c.Snapshot().Phase != Available {
		t.Fatal("attempted installation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, stopped := make(chan struct{}), make(chan struct{})
	var installs atomic.Int32
	c = New(ctx, func(ctx context.Context) (*Release, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return &Release{Version: "1.0.0", Install: func(context.Context, func(int64, int64)) error { installs.Add(1); return nil }}, nil
	}, true, nil)
	c.Check(true)
	<-started
	cancel()
	<-stopped
	if installs.Load() != 0 {
		t.Fatal("installed after shutdown")
	}
}
func TestPeriodicChecksRespectPreference(t *testing.T) {
	var checks atomic.Int32
	called := make(chan struct{}, 1)
	c := New(testContext(t), func(context.Context) (*Release, error) {
		checks.Add(1)
		select {
		case called <- struct{}{}:
		default:
		}
		return nil, nil
	}, false, nil)
	c.startDelay = time.Millisecond
	c.interval = 10 * time.Millisecond
	go c.Run()
	time.Sleep(30 * time.Millisecond)
	if checks.Load() != 0 {
		t.Fatal("checked with automatic updates disabled")
	}
	c.SetAutomatic(true)
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("enabling automatic checks did not check")
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("periodic check missing")
	}
}
