package fffbridge

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReadCallsShareHandleAndCloseWaits(t *testing.T) {
	bridge, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bridge.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	for range 32 {
		events, err := bridge.PollEvents(0, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
	}

	pollDone := make(chan error, 1)
	go func() {
		_, err := bridge.PollEvents(time.Second, 100)
		pollDone <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for bridge.mu.TryLock() {
		bridge.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("PollEvents did not enter its blocking read")
		}
		runtime.Gosched()
	}
	started := time.Now()
	if _, err := bridge.Status(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 300*time.Millisecond {
		t.Fatalf("status waited behind the blocking event poll: %s", elapsed)
	}
	select {
	case err := <-pollDone:
		t.Fatalf("event poll returned before its timeout: %v", err)
	default:
	}

	closeStarted := make(chan struct{})
	closeDone := make(chan struct{})
	go func() {
		close(closeStarted)
		bridge.Close()
		close(closeDone)
	}()
	<-closeStarted
	select {
	case <-closeDone:
		t.Fatal("Close returned while PollEvents still held a read lock")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case err := <-pollDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PollEvents did not finish")
	}
	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not finish after active calls returned")
	}
	if _, err := bridge.Status(); err == nil || !errors.Is(err, errHandleClosed) {
		t.Fatalf("Status after Close error = %v, want closed handle", err)
	}
	if err := bridge.WaitReady(context.Background()); err == nil || !errors.Is(err, errHandleClosed) {
		t.Fatalf("WaitReady after Close error = %v, want closed handle", err)
	}
	bridge.Close()
}

func TestFFFCStringInputsRejectNUL(t *testing.T) {
	if _, err := Open("root\x00suffix", Options{}); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("Open NUL error = %v", err)
	}
	bridge := &Bridge{}
	if _, err := bridge.SearchPaths("query\x00suffix", 0, 1); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("path query NUL error = %v", err)
	}
	if _, err := bridge.Grep("query\x00suffix", "plain", 0, 1, 0, 0); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("grep query NUL error = %v", err)
	}
	if _, err := bridge.Grep("query", "plain\x00suffix", 0, 1, 0, 0); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("grep mode NUL error = %v", err)
	}
}

func TestPollEventsPreservesRescanReason(t *testing.T) {
	bridge, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bridge.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}

	events, err := bridge.PollEvents(0, 128)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.RescanRequired && event.Reason == "watch_subscription_gap" {
			return
		}
	}
	t.Fatalf("PollEvents did not preserve the watch subscription gap reason: %+v", events)
}
