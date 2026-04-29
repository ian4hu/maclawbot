package botmanager

import (
	"context"
	"testing"
	"time"

	"maclawbot/internal/event"
	"maclawbot/internal/router"
)

// TestNew tests that New returns a functional Manager.
func TestNew(t *testing.T) {
	state := router.NewState("/tmp/test_mgr_new.json")
	bus := event.NewBus()
	mgr := New(state, "http://localhost:9", 1*time.Second, bus)

	if mgr == nil {
		t.Fatal("Expected non-nil Manager")
	}
	if mgr.state != state {
		t.Error("Expected state to be set")
	}
	if mgr.bus != bus {
		t.Error("Expected bus to be set")
	}
	if mgr.cancels == nil {
		t.Error("Expected cancels map to be initialized")
	}
}

// TestStartBot_StopBot tests StartBot/StopBot lifecycle.
// We test with a non-routable address that fails immediately.
// The StartBot call returns after starting the goroutine; StopBot cancels its context.
func TestStartBot_StopBot(t *testing.T) {
	tmpFile := "/tmp/test_mgr_startstop_" + t.Name() + ".json"

	mgr := New(router.NewState(tmpFile), "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())
	// 10.255.255.1 is non-routable — connection will be refused immediately.
	// runPollLoop will exit on first error (connection refused is fatal after backoff),
	// but we verify StopBot cleans up the cancel map.

	bot := router.Bot{BotID: "testbot", Token: "tok", Enabled: true}

	mgr.StartBot(bot)
	mgr.StopBot("testbot")

	// ActiveBots should be empty after stop
	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected no active bots after start+stop, got %v", ids)
	}
}

// TestStartBot_Idempotent tests that calling StartBot twice restarts the goroutine.
func TestStartBot_Idempotent(t *testing.T) {
	tmpFile := "/tmp/test_mgr_idempotent_" + t.Name() + ".json"

	mgr := New(router.NewState(tmpFile), "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())

	bot := router.Bot{BotID: "idempotent", Token: "tok", Enabled: true}

	mgr.StartBot(bot)
	mgr.StartBot(bot) // Should cancel old and start new

	// Should only be one active bot
	if ids := mgr.ActiveBots(); len(ids) != 1 {
		t.Errorf("Expected 1 active bot, got %v", ids)
	}

	mgr.StopAll()
}

// TestStopBot_Unknown tests stopping a bot that was never started.
func TestStopBot_Unknown(t *testing.T) {
	tmpFile := "/tmp/test_mgr_stopunknown_" + t.Name() + ".json"

	mgr := New(router.NewState(tmpFile), "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())

	// Should not panic
	mgr.StopBot("never_started")

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected no active bots, got %v", ids)
	}
}

// TestStopAll tests stopping all poll loops.
func TestStopAll(t *testing.T) {
	tmpFile := "/tmp/test_mgr_stopall_" + t.Name() + ".json"

	mgr := New(router.NewState(tmpFile), "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())

	mgr.StartBot(router.Bot{BotID: "bot1", Token: "t1", Enabled: true})
	mgr.StartBot(router.Bot{BotID: "bot2", Token: "t2", Enabled: true})

	if ids := mgr.ActiveBots(); len(ids) != 2 {
		t.Fatalf("Expected 2 active bots before StopAll, got %v", ids)
	}

	mgr.StopAll()

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected no active bots after StopAll, got %v", ids)
	}
}

// TestStopAll_Empty tests StopAll with no running bots.
func TestStopAll_Empty(t *testing.T) {
	tmpFile := "/tmp/test_mgr_stopall_empty_" + t.Name() + ".json"

	mgr := New(router.NewState(tmpFile), "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())

	mgr.StopAll() // should not panic

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected empty active bots, got %v", ids)
	}
}

// TestStartAll tests that StartAll starts all enabled bots.
func TestStartAll(t *testing.T) {
	tmpFile := "/tmp/test_mgr_startall_" + t.Name() + ".json"

	state := router.NewState(tmpFile)
	bus := event.NewBus()
	mgr := New(state, "http://10.255.255.1:1", 10*time.Millisecond, bus)

	state.AddBot(router.Bot{BotID: "ena1", Token: "t1", Enabled: true})
	state.AddBot(router.Bot{BotID: "ena2", Token: "t2", Enabled: true})
	state.AddBot(router.Bot{BotID: "dis1", Token: "t3", Enabled: false}) // disabled

	mgr.StartAll()

	if ids := mgr.ActiveBots(); len(ids) != 2 {
		t.Errorf("Expected 2 active bots (enabled only), got %v", ids)
	}

	mgr.StopAll()
}

// TestStartAll_Empty tests StartAll with no bots.
func TestStartAll_Empty(t *testing.T) {
	tmpFile := "/tmp/test_mgr_startall_empt_" + t.Name() + ".json"

	state := router.NewState(tmpFile)
	bus := event.NewBus()
	mgr := New(state, "http://10.255.255.1:1", 10*time.Millisecond, bus)

	mgr.StartAll()

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected 0 active bots, got %v", ids)
	}
}

// TestOnEvent_BotAddedEvent tests that BotAddedEvent starts a poll loop.
func TestOnEvent_BotAddedEvent(t *testing.T) {
	tmpFile := "/tmp/test_mgr_onadd_" + t.Name() + ".json"

	state := router.NewState(tmpFile)
	bus := event.NewBus()
	mgr := New(state, "http://10.255.255.1:1", 10*time.Millisecond, bus)

	bot := router.Bot{BotID: "added_bot", Token: "t", Enabled: true}
	mgr.OnEvent(event.BotAddedEvent{Bot: bot})

	if ids := mgr.ActiveBots(); len(ids) != 1 {
		t.Fatalf("Expected 1 active bot after BotAddedEvent, got %v", ids)
	}

	mgr.StopAll()
}

// TestOnEvent_BotRemovedEvent tests that BotRemovedEvent stops a poll loop.
func TestOnEvent_BotRemovedEvent(t *testing.T) {
	tmpFile := "/tmp/test_mgr_onremove_" + t.Name() + ".json"

	state := router.NewState(tmpFile)
	mgr := New(state, "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())

	bot := router.Bot{BotID: "to_remove", Token: "t", Enabled: true}

	mgr.StartBot(bot)
	if ids := mgr.ActiveBots(); len(ids) != 1 {
		t.Fatalf("Expected 1 active bot, got %v", ids)
	}

	mgr.OnEvent(event.BotRemovedEvent{BotID: "to_remove"})

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected 0 active bots after BotRemovedEvent, got %v", ids)
	}
}

// TestOnEvent_UnknownEvent tests that unknown event types are silently ignored.
func TestOnEvent_UnknownEvent(t *testing.T) {
	tmpFile := "/tmp/test_mgr_unknown_event_" + t.Name() + ".json"

	state := router.NewState(tmpFile)
	bus := event.NewBus()
	mgr := New(state, "http://10.255.255.1:1", 10*time.Millisecond, bus)

	mgr.OnEvent("not_an_event_struct")
	mgr.OnEvent(42)
	mgr.OnEvent(struct{ Foo string }{Foo: "bar"})

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected no active bots for unknown events, got %v", ids)
	}
}

// TestActiveBots tests that ActiveBots returns running bot IDs correctly.
func TestActiveBots(t *testing.T) {
	tmpFile := "/tmp/test_mgr_activebots_" + t.Name() + ".json"

	mgr := New(router.NewState(tmpFile), "http://10.255.255.1:1", 10*time.Millisecond, event.NewBus())

	if ids := mgr.ActiveBots(); len(ids) != 0 {
		t.Errorf("Expected empty list initially, got %v", ids)
	}

	mgr.StartBot(router.Bot{BotID: "first", Token: "t1", Enabled: true})
	mgr.StartBot(router.Bot{BotID: "second", Token: "t2", Enabled: true})

	ids := mgr.ActiveBots()
	if len(ids) != 2 {
		t.Errorf("Expected 2 bots, got %v", ids)
	}
	found := map[string]bool{"first": false, "second": false}
	for _, id := range ids {
		found[id] = true
	}
	for id, ok := range found {
		if !ok {
			t.Errorf("Expected bot %q in ActiveBots", id)
		}
	}

	mgr.StopAll()
}

// TestIsTimeout tests isTimeout for various error types.
func TestIsTimeout(t *testing.T) {
	// context.DeadlineExceeded is a timeout
	if !isTimeout(context.DeadlineExceeded) {
		t.Error("Expected DeadlineExceeded to be timeout")
	}

	// context.Canceled is not a timeout
	if isTimeout(context.Canceled) {
		t.Error("Expected Canceled to not be timeout")
	}
}

// TestHandleFailure tests backoff logic (does not start any goroutines).
func TestHandleFailure(t *testing.T) {
	backoff := 10 * time.Millisecond

	// fails=0 (below max) -> returns 0
	got := handleFailure(0, 3, backoff)
	if got != 0 {
		t.Errorf("handleFailure(0) — expected 0, got %d", got)
	}

	// fails=2 (below max) -> returns 2 (slept 10ms)
	got = handleFailure(2, 3, backoff)
	if got != 2 {
		t.Errorf("handleFailure(2) — expected 2, got %d", got)
	}

	// fails=3 (>= max) -> returns 0 (reset, slept backoff ms)
	got = handleFailure(3, 3, backoff)
	if got != 0 {
		t.Errorf("handleFailure(3,3) — expected 0 (reset), got %d", got)
	}

	// fails=5 (> max) -> returns 0 (reset)
	got = handleFailure(5, 3, backoff)
	if got != 0 {
		t.Errorf("handleFailure(5,3) — expected 0 (reset), got %d", got)
	}

	// fails=0 with different maxFails
	got = handleFailure(0, 1, backoff)
	if got != 0 {
		t.Errorf("handleFailure(0,1) — expected 0, got %d", got)
	}

	// fails=1 with maxFails=1 -> reset (slept backoff ms)
	got = handleFailure(1, 1, backoff)
	if got != 0 {
		t.Errorf("handleFailure(1,1) — expected 0 (reset), got %d", got)
	}
}