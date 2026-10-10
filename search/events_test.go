package search

import (
	"testing"

	"github.com/ZanzyTHEbar/virtual-vectorfs/internal/fffbridge"
)

func TestWatchSubscriptionGapDoesNotSuppressOverflowRescan(t *testing.T) {
	engine := &Engine{rescan: make(chan struct{}, 1)}
	engine.handleEvent(fffbridge.Event{
		Kind:           "rescan",
		Reason:         "watch_subscription_gap",
		RescanRequired: true,
	})
	if len(engine.rescan) != 0 {
		t.Fatal("watch subscription gap scheduled a redundant rescan")
	}

	engine.handleEvent(fffbridge.Event{
		Kind:           "rescan",
		Reason:         "bridge_queue_overflow",
		RescanRequired: true,
	})
	if len(engine.rescan) != 1 {
		t.Fatal("bridge queue overflow did not schedule a rescan")
	}
}
