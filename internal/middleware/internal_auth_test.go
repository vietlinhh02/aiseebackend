package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/team-dang/aiseebackend/internal/middleware"
)

func TestInternalAuth_ValidKey(t *testing.T) {
	handler := middleware.InternalAuth("test-key")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Internal-Key", "test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestInternalAuth_WrongKey(t *testing.T) {
	handler := middleware.InternalAuth("test-key")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not reach handler")
		},
	))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Internal-Key", "wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
