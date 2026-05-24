package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/billing"
)

type fakeAdminHandlerService struct {
	listPage   int32
	listLimit  int32
	listResult []AdminBillingTransaction
	listErr    error
	userID     string
	userPage   int32
	userLimit  int32
	userResult []AdminBillingTransaction
	userErr    error
	days       int32
	summary    TrackingSummary
	timeline   []UserTimelineEvent
	tracked    AdminTrackingEventInput
}

func (f *fakeAdminHandlerService) SearchUsers(
	context.Context,
	string,
	int32,
	int32,
) ([]UserResult, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetUserProfile(
	context.Context,
	string,
) (*UserProfile, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetFamilyLinks(
	context.Context,
	string,
) ([]FamilyLinkResult, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetUserSubscription(
	context.Context,
	string,
) (*AdminSubscription, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetUserUsage(
	context.Context,
	string,
	int32,
	int32,
) ([]AdminUsageEntry, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetUserAICostEvents(
	context.Context,
	string,
	int32,
) ([]AICostEventResult, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetUserEntitlements(
	context.Context,
	string,
) ([]AdminEntitlementResult, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetSubscriptionEvents(
	context.Context,
	int32,
	int32,
) ([]AdminSubEvent, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetAuditLogs(
	context.Context,
	string,
	int32,
	int32,
) ([]map[string]any, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) GetAnalytics(
	context.Context,
) (AdminAnalytics, error) {
	return AdminAnalytics{}, nil
}

func (f *fakeAdminHandlerService) GetAICostSummary(
	context.Context,
	int32,
) (AICostSummary, error) {
	return AICostSummary{}, nil
}

func (f *fakeAdminHandlerService) GetTrackingSummary(
	_ context.Context,
	days int32,
) (TrackingSummary, error) {
	f.days = days
	return f.summary, nil
}

func (f *fakeAdminHandlerService) GetUserTimeline(
	_ context.Context,
	userID string,
	limit int32,
) ([]UserTimelineEvent, error) {
	f.userID = userID
	f.userLimit = limit
	return f.timeline, nil
}

func (f *fakeAdminHandlerService) RecordAdminTrackingEvent(
	_ context.Context,
	_ string,
	input AdminTrackingEventInput,
) (TrackingEvent, error) {
	f.tracked = input
	return TrackingEvent{ID: "track-1", EventType: input.EventType}, nil
}

func (f *fakeAdminHandlerService) ListBillingTransactions(
	_ context.Context,
	page,
	limit int32,
) ([]AdminBillingTransaction, error) {
	f.listPage = page
	f.listLimit = limit
	return f.listResult, f.listErr
}

func (f *fakeAdminHandlerService) ListUserBillingTransactions(
	_ context.Context,
	userID string,
	page,
	limit int32,
) ([]AdminBillingTransaction, error) {
	f.userID = userID
	f.userPage = page
	f.userLimit = limit
	return f.userResult, f.userErr
}

func (f *fakeAdminHandlerService) ListIncidentsForAdmin(context.Context, string, int32) ([]AdminIncidentListItem, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) ListCareAlertsForAdmin(context.Context, string, int32) ([]AdminCareAlertListItem, error) {
	return nil, nil
}

func (f *fakeAdminHandlerService) InsertBillingRefundNote(context.Context, string, string, string) error {
	return nil
}

func (f *fakeAdminHandlerService) ManualGrantUsage(
	_ context.Context,
	_, _ string,
	_ ManualGrantInput,
) (billing.UsageGrant, error) {
	return billing.UsageGrant{ID: "stub-grant"}, nil
}

func (f *fakeAdminHandlerService) IsAdmin(context.Context, string) (bool, error) {
	return true, nil
}

func TestListBillingTransactionsClampsLimitAndReturnsEnvelope(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{
		listResult: []AdminBillingTransaction{
			{
				ID:              "txn-1",
				UserID:          "user-1",
				UserEmail:       "care@example.com",
				UserDisplayName: "Care User",
				Provider:        "revenuecat",
				ProviderEventID: "evt-1",
				Status:          "paid",
				Amount:          149000,
				Currency:        "VND",
				OccurredAt:      "2026-04-29T09:30:00Z",
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/billing-transactions?page=2&limit=999", nil)
	rec := httptest.NewRecorder()

	NewHandler(svc).ListBillingTransactions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.listPage != 2 {
		t.Fatalf("page = %d, want 2", svc.listPage)
	}
	if svc.listLimit != 200 {
		t.Fatalf("limit = %d, want 200", svc.listLimit)
	}

	var body struct {
		Transactions []AdminBillingTransaction `json:"transactions"`
		Page         int32                     `json:"page"`
		Limit        int32                     `json:"limit"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Page != 2 || body.Limit != 200 {
		t.Fatalf("envelope = %+v, want page=2 limit=200", body)
	}
	if len(body.Transactions) != 1 || body.Transactions[0].ID != "txn-1" {
		t.Fatalf("transactions = %+v", body.Transactions)
	}
}

func TestListUserBillingTransactionsUsesRouteUserID(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{
		userResult: []AdminBillingTransaction{
			{
				ID:              "txn-2",
				UserID:          "user-9",
				UserEmail:       "family@example.com",
				UserDisplayName: "Family User",
				Provider:        "revenuecat",
				ProviderEventID: "evt-2",
				Status:          "refunded",
				Amount:          99000,
				Currency:        "VND",
				OccurredAt:      "2026-04-28T09:30:00Z",
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/users/user-9/billing-transactions?limit=250", nil)
	rec := httptest.NewRecorder()
	req = withURLParam(req, "user_id", "user-9")

	NewHandler(svc).ListUserBillingTransactions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.userID != "user-9" {
		t.Fatalf("userID = %q, want user-9", svc.userID)
	}
	if svc.userPage != 1 {
		t.Fatalf("page = %d, want 1", svc.userPage)
	}
	if svc.userLimit != 200 {
		t.Fatalf("limit = %d, want 200", svc.userLimit)
	}
}

func TestListBillingTransactionsReturnsInternalError(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{
		listErr: errors.New("db unavailable"),
	}
	req := httptest.NewRequest(http.MethodGet, "/billing-transactions", nil)
	rec := httptest.NewRecorder()

	NewHandler(svc).ListBillingTransactions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "billing_transactions_failed" {
		t.Fatalf("error code = %q, want billing_transactions_failed", body.Error.Code)
	}
}

func TestGetTrackingSummaryClampsDays(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{
		summary: TrackingSummary{AISessionsStarted: 5},
	}
	req := httptest.NewRequest(http.MethodGet, "/tracking?days=999", nil)
	rec := httptest.NewRecorder()

	NewHandler(svc).GetTrackingSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.days != 365 {
		t.Fatalf("days = %d, want 365", svc.days)
	}

	var body TrackingSummary
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.AISessionsStarted != 5 {
		t.Fatalf("summary = %+v", body)
	}
}

func TestGetUserTimelineUsesRouteUserIDAndLimit(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{
		timeline: []UserTimelineEvent{{ID: "track-1", Kind: "tracking"}},
	}
	req := httptest.NewRequest(http.MethodGet, "/users/user-3/timeline?limit=999", nil)
	req = withURLParam(req, "user_id", "user-3")
	rec := httptest.NewRecorder()

	NewHandler(svc).GetUserTimeline(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if svc.userID != "user-3" {
		t.Fatalf("userID = %q, want user-3", svc.userID)
	}
	if svc.userLimit != 300 {
		t.Fatalf("limit = %d, want 300", svc.userLimit)
	}

	var body struct {
		Events []UserTimelineEvent `json:"events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != "track-1" {
		t.Fatalf("events = %+v", body.Events)
	}
}

func TestManualGrantRejectsMissingFields(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{}
	req := httptest.NewRequest(
		http.MethodPost,
		"/users/user-1/manual-grant",
		strings.NewReader(`{"usage_type":"","quantity":0,"reason":""}`),
	)
	req = withURLParam(req, "user_id", "user-1")
	rec := httptest.NewRecorder()

	NewHandler(svc).ManualGrant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "usage_type, quantity") ||
		!strings.Contains(rec.Body.String(), "reason are required") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestRecordTrackingEventRejectsMissingEventType(t *testing.T) {
	t.Parallel()

	svc := &fakeAdminHandlerService{}
	req := httptest.NewRequest(
		http.MethodPost,
		"/tracking/events",
		strings.NewReader(`{"source":"admin","source_id":"evt-1"}`),
	)
	rec := httptest.NewRecorder()

	NewHandler(svc).RecordTrackingEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "event_type is required") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func withURLParam(r *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
}
