package user

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJoinFamilyRejectsMissingInviteCode(t *testing.T) {
	t.Parallel()

	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/family/join",
		strings.NewReader(`{}`),
	)
	rec := httptest.NewRecorder()

	h.JoinFamily(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invite_code is required") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestTransferGroupOwnershipRejectsMissingTargetUser(t *testing.T) {
	t.Parallel()

	h := NewHandler(nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/family/group/transfer",
		strings.NewReader(`{}`),
	)
	rec := httptest.NewRecorder()

	h.TransferGroupOwnership(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "target_user_id is required") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
