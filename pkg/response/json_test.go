package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON_SetsContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	JSON(rr, http.StatusOK, map[string]string{"ok": "true"})

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func TestJSON_SetsStatusCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"NotFound", http.StatusNotFound},
		{"InternalServerError", http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			JSON(rr, tt.status, map[string]string{"x": "y"})
			if rr.Code != tt.status {
				t.Errorf("expected %d, got %d", tt.status, rr.Code)
			}
		})
	}
}

func TestJSON_EncodesBody(t *testing.T) {
	rr := httptest.NewRecorder()
	JSON(rr, http.StatusOK, map[string]string{"hello": "world"})

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("expected 'world', got %q", body["hello"])
	}
}

func TestErr_ReturnsErrorStructure(t *testing.T) {
	rr := httptest.NewRecorder()
	Err(rr, http.StatusUnauthorized, "auth_failed", "Not authorized")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}

	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Error.Code != "auth_failed" {
		t.Errorf("expected code 'auth_failed', got %q", body.Error.Code)
	}
	if body.Error.Message != "Not authorized" {
		t.Errorf("expected message 'Not authorized', got %q", body.Error.Message)
	}
}

func TestErr_ContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	Err(rr, http.StatusBadRequest, "bad", "bad request")

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}
