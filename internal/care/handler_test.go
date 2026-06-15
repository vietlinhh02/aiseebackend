package care

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
)

type fakeFamilyChecker struct {
	linked bool
	err    error
}

func (f *fakeFamilyChecker) CheckFamilyLink(
	_ context.Context,
	_ string,
	_ string,
) (bool, error) {
	return f.linked, f.err
}

type fakeHandlerService struct {
	timelineResp TimelineResponse
	timelineErr  error
	dailyResp    DailyBriefResponse
	dailyErr     error
	weeklyResp   WeeklyReportResponse
	weeklyErr    error
	incidentResp IncidentReportResponse
	incidentErr  error
	exportResp   ExportResponse
	exportErr    error
}

func (f *fakeHandlerService) Timeline(
	_ context.Context,
	_ string,
	_ string,
	_ TimelineOptions,
) (TimelineResponse, error) {
	return f.timelineResp, f.timelineErr
}

func (f *fakeHandlerService) DailyBrief(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
) (DailyBriefResponse, error) {
	return f.dailyResp, f.dailyErr
}

func (f *fakeHandlerService) WeeklyReport(
	_ context.Context,
	_ string,
	_ string,
	_ time.Time,
) (WeeklyReportResponse, error) {
	return f.weeklyResp, f.weeklyErr
}

func (f *fakeHandlerService) IncidentReport(
	_ context.Context,
	_ string,
	_ string,
) (IncidentReportResponse, error) {
	return f.incidentResp, f.incidentErr
}

func (f *fakeHandlerService) ExportPDF(
	_ context.Context,
	_ string,
	_ string,
	_ ExportPDFRequest,
) (ExportResponse, error) {
	return f.exportResp, f.exportErr
}

func (f *fakeHandlerService) NavigationHistory(
	_ context.Context,
	_ string,
	_ string,
	_ int,
	_ int,
) (NavigationHistoryResponse, error) {
	return NavigationHistoryResponse{Trips: []NavigationTrip{}, WindowDays: 30}, nil
}

func TestHandlerTimelineReturnsForbiddenWithoutFamilyAccess(t *testing.T) {
	h := NewHandler(&fakeHandlerService{}, &fakeFamilyChecker{}, nil)
	req := withCareRouteContext(
		httptest.NewRequest(http.MethodGet, "/timeline", nil),
		"user-1",
	)
	req = req.WithContext(middleware.WithUserID(req.Context(), "family-1"))
	req = req.WithContext(middleware.WithRole(req.Context(), "family"))
	rr := httptest.NewRecorder()

	h.Timeline(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandlerDailyBriefMapsEntitlementErrors(t *testing.T) {
	h := NewHandler(&fakeHandlerService{
		dailyErr: &APIError{
			Status:  402,
			Code:    "entitlement_required",
			Message: "upgrade required",
		},
	}, &fakeFamilyChecker{linked: true}, nil)
	req := withCareRouteContext(
		httptest.NewRequest(http.MethodGet, "/daily-brief", nil),
		"user-1",
	)
	req = req.WithContext(middleware.WithUserID(req.Context(), "family-1"))
	req = req.WithContext(middleware.WithRole(req.Context(), "family"))
	rr := httptest.NewRecorder()

	h.DailyBrief(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandlerExportPDFRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(&fakeHandlerService{}, &fakeFamilyChecker{linked: true}, nil)
	req := withCareRouteContext(
		httptest.NewRequest(
			http.MethodPost,
			"/export/pdf",
			bytes.NewBufferString("{"),
		),
		"user-1",
	)
	req = req.WithContext(middleware.WithUserID(req.Context(), "family-1"))
	req = req.WithContext(middleware.WithRole(req.Context(), "family"))
	rr := httptest.NewRecorder()

	h.ExportPDF(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandlerExportPDFReturnsStructuredPlaceholder(t *testing.T) {
	h := NewHandler(&fakeHandlerService{
		exportResp: ExportResponse{
			ExportID:      "export-1",
			ReportType:    "weekly_report",
			Format:        "pdf_placeholder",
			UsageRecordID: "usage-1",
		},
	}, &fakeFamilyChecker{linked: true}, nil)
	body, _ := json.Marshal(ExportPDFRequest{ReportType: "weekly_report"})
	req := withCareRouteContext(
		httptest.NewRequest(
			http.MethodPost,
			"/export/pdf",
			bytes.NewReader(body),
		),
		"user-1",
	)
	req = req.WithContext(middleware.WithUserID(req.Context(), "family-1"))
	req = req.WithContext(middleware.WithRole(req.Context(), "family"))
	rr := httptest.NewRecorder()

	h.ExportPDF(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d", rr.Code)
	}
	var resp ExportResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ExportID != "export-1" || resp.Format != "pdf_placeholder" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandlerTimelineMapsUnexpectedErrors(t *testing.T) {
	h := NewHandler(&fakeHandlerService{
		timelineErr: errors.New("boom"),
	}, &fakeFamilyChecker{linked: true}, nil)
	req := withCareRouteContext(
		httptest.NewRequest(http.MethodGet, "/timeline", nil),
		"user-1",
	)
	req = req.WithContext(middleware.WithUserID(req.Context(), "family-1"))
	req = req.WithContext(middleware.WithRole(req.Context(), "family"))
	rr := httptest.NewRecorder()

	h.Timeline(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func withCareRouteContext(req *http.Request, userID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", userID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
