package toolcall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleToolCall_ValidRequest(t *testing.T) {
	h := NewHandler()
	body := `{"tool":"navigate","user_id":"u1","params":{"dest":"home"}}`

	req := httptest.NewRequest("POST", "/tool-call", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleToolCall(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["tool"] != "navigate" {
		t.Errorf("expected tool 'navigate', got %v", resp["tool"])
	}
	if resp["status"] != "not_implemented" {
		t.Errorf("expected status 'not_implemented', got %v", resp["status"])
	}
}

func TestHandleToolCall_InvalidJSON(t *testing.T) {
	h := NewHandler()

	req := httptest.NewRequest("POST", "/tool-call", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleToolCall(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "invalid_body" {
		t.Errorf("expected error code 'invalid_body', got %v", errObj["code"])
	}
}

func TestHandleToolCall_MissingToolField(t *testing.T) {
	h := NewHandler()
	body := `{"user_id":"u1","params":{}}`

	req := httptest.NewRequest("POST", "/tool-call", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleToolCall(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["tool"] != "" {
		t.Errorf("expected empty tool name, got %v", resp["tool"])
	}
}

func TestHandleToolCall_EmptyBody(t *testing.T) {
	h := NewHandler()

	req := httptest.NewRequest("POST", "/tool-call", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleToolCall(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
