package chat

import (
	"testing"
)

func TestHub_AddRemove(t *testing.T) {
	hub := NewHub()

	if hub.IsOnline("user-1") {
		t.Error("expected user-1 offline before add")
	}

	// We can't easily test with real websocket.Conn in unit tests,
	// so we test the map operations directly
	hub.mu.Lock()
	hub.conns["user-1"] = nil
	hub.mu.Unlock()

	if !hub.IsOnline("user-1") {
		t.Error("expected user-1 online after add")
	}

	hub.Remove("user-1")

	if hub.IsOnline("user-1") {
		t.Error("expected user-1 offline after remove")
	}
}

func TestHub_RemoveNonexistent(t *testing.T) {
	hub := NewHub()
	hub.Remove("nonexistent") // should not panic
}
