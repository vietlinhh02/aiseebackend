package toolcall

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleToolCallRejectsMissingTool(t *testing.T) {
	t.Parallel()

	h := NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/internal/tool-call",
		strings.NewReader(`{"user_id":"user-1","params":{"contact_name":"Lan"}}`),
	)
	rr := httptest.NewRecorder()

	h.HandleToolCall(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "tool is required") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}
