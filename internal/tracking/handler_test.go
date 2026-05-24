package tracking

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecordEventRejectsMissingUserID(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&fakeStore{}))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/internal/tracking/events",
		strings.NewReader(`{
			"event_type":"ai_session_started",
			"category":"ai_ops",
			"source":"agent_worker",
			"source_id":"session-1"
		}`),
	)
	rr := httptest.NewRecorder()

	h.RecordEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "user_id is required") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}
