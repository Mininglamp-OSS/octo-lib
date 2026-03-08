package config

import (
	"sync"
	"testing"
)

// TestOnlineStatusListenerConcurrent tests concurrent access to online status listeners
func TestOnlineStatusListenerConcurrent(t *testing.T) {
	ctx := &Context{}

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ctx.AddOnlineStatusListener(func(onlineStatusList []OnlineStatus) {})
		}()
	}

	// Concurrent reads while writing
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = ctx.GetAllOnlineStatusListeners()
		}()
	}

	wg.Wait()

	// Verify all listeners were added
	listeners := ctx.GetAllOnlineStatusListeners()
	if len(listeners) != numGoroutines {
		t.Errorf("expected %d listeners, got %d", numGoroutines, len(listeners))
	}
}

// TestEventListenerConcurrent tests concurrent access to event listeners
func TestEventListenerConcurrent(t *testing.T) {
	ctx := &Context{}

	var wg sync.WaitGroup
	const numGoroutines = 100
	const event = "test_event"

	// Concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ctx.AddEventListener(event, func(data []byte, commit EventCommit) {})
		}()
	}

	// Concurrent reads while writing
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = ctx.GetEventListeners(event)
		}()
	}

	wg.Wait()

	// Verify all listeners were added
	listeners := ctx.GetEventListeners(event)
	if len(listeners) != numGoroutines {
		t.Errorf("expected %d listeners, got %d", numGoroutines, len(listeners))
	}
}

// TestMessagesListenerConcurrent tests concurrent access to messages listeners
func TestMessagesListenerConcurrent(t *testing.T) {
	ctx := &Context{}

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ctx.AddMessagesListener(func(messages []*MessageResp) {})
		}()
	}

	// Concurrent reads (notify) while writing
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ctx.NotifyMessagesListeners([]*MessageResp{})
		}()
	}

	wg.Wait()
}

// TestEventListenerMultipleEvents tests concurrent access to multiple events
func TestEventListenerMultipleEvents(t *testing.T) {
	ctx := &Context{}

	var wg sync.WaitGroup
	const numGoroutines = 50
	events := []string{"event1", "event2", "event3", "event4"}

	// Concurrent writes to different events
	for _, event := range events {
		event := event
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				ctx.AddEventListener(event, func(data []byte, commit EventCommit) {})
			}()
		}
	}

	// Concurrent reads from different events
	for _, event := range events {
		event := event
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				_ = ctx.GetEventListeners(event)
			}()
		}
	}

	wg.Wait()

	// Verify all listeners were added to each event
	for _, event := range events {
		listeners := ctx.GetEventListeners(event)
		if len(listeners) != numGoroutines {
			t.Errorf("event %s: expected %d listeners, got %d", event, numGoroutines, len(listeners))
		}
	}
}
