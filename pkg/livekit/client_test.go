package livekit

import (
	"testing"
)

func TestNewClient_StoresParams(t *testing.T) {
	c := NewClient("wss://lk.example.com", "api-key-123", "api-secret-456")

	if c.host != "wss://lk.example.com" {
		t.Errorf("expected host 'wss://lk.example.com', got %q", c.host)
	}
	if c.apiKey != "api-key-123" {
		t.Errorf("expected apiKey 'api-key-123', got %q", c.apiKey)
	}
	if c.apiSecret != "api-secret-456" {
		t.Errorf("expected apiSecret 'api-secret-456', got %q", c.apiSecret)
	}
}

func TestCreateToken_ReturnsNonEmpty(t *testing.T) {
	c := NewClient("wss://lk.example.com", "test-api-key", "test-api-secret")

	token, err := c.CreateToken("room-1", "user-abc", "Eddie")
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestCreateToken_DifferentRoomsDifferentTokens(t *testing.T) {
	c := NewClient("wss://lk.example.com", "test-api-key", "test-api-secret")

	tok1, err := c.CreateToken("room-1", "user-abc", "Eddie")
	if err != nil {
		t.Fatalf("CreateToken room-1 failed: %v", err)
	}

	tok2, err := c.CreateToken("room-2", "user-abc", "Eddie")
	if err != nil {
		t.Fatalf("CreateToken room-2 failed: %v", err)
	}

	if tok1 == tok2 {
		t.Error("tokens for different rooms should differ")
	}
}
