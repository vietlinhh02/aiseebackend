package care

import (
	"context"
	"testing"
	"time"

	"github.com/team-dang/aiseebackend/internal/billing"
)

type fakeStore struct {
	windowData WindowData
	start      time.Time
	end        time.Time
	userID     string

	denyFamilyLink bool
}

func (f *fakeStore) LoadWindow(
	_ context.Context,
	userID string,
	start time.Time,
	end time.Time,
) (WindowData, error) {
	f.userID = userID
	f.start = start
	f.end = end
	return f.windowData, nil
}

func (f *fakeStore) InsertCareEvent(context.Context, InsertCareEventInput) error { return nil }

func (f *fakeStore) ListCareEventsForCitation(context.Context, string, int) ([]CareEventRow, error) {
	return nil, nil
}

func (f *fakeStore) CreateIncidentSOS(context.Context, string, string, float64, float64, int) (string, error) {
	return "", nil
}

func (f *fakeStore) AppendIncidentEvent(context.Context, string, string, *string, map[string]any) error {
	return nil
}

func (f *fakeStore) LatestIncidentInRange(context.Context, string, time.Time, time.Time) (*IncidentRow, error) {
	return nil, nil
}

func (f *fakeStore) ListIncidentEvents(context.Context, string) ([]IncidentEventRow, error) {
	return nil, nil
}

func (f *fakeStore) GetLatestOpenIncidentID(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (f *fakeStore) HasFamilyLinkBetween(context.Context, string, string) (bool, error) {
	if f.denyFamilyLink {
		return false, nil
	}
	return true, nil
}

func (f *fakeStore) ListCareAlerts(context.Context, string, string, int) ([]CareAlertRow, error) {
	return nil, nil
}

func (f *fakeStore) InsertCareAlert(context.Context, string, string, string, string, *string, map[string]any) (string, error) {
	return "", nil
}

func (f *fakeStore) UpdateCareAlertAck(context.Context, string, string) error { return nil }

func (f *fakeStore) UpdateCareAlertResolve(context.Context, string, string) error { return nil }

func (f *fakeStore) UpdateCareAlertDismiss(context.Context, string, string) error { return nil }

func (f *fakeStore) GetCareAlertForUser(context.Context, string, string) (*CareAlertRow, error) {
	return nil, nil
}

func (f *fakeStore) GetCareAlertByID(context.Context, string) (*CareAlertRow, error) {
	return nil, nil
}

func (f *fakeStore) ListCareTasks(context.Context, string, int) ([]CareTaskRow, error) {
	return nil, nil
}

func (f *fakeStore) InsertCareTask(context.Context, string, string, string, *time.Time, map[string]any) (string, error) {
	return "", nil
}

func (f *fakeStore) UpdateCareTask(context.Context, string, string, string, *time.Time, string, map[string]any) error {
	return nil
}

func (f *fakeStore) CompleteCareTask(context.Context, string, string) error { return nil }

func (f *fakeStore) GetCareTask(context.Context, string, string) (*CareTaskRow, error) {
	return nil, nil
}

func (f *fakeStore) GetCareTaskByID(context.Context, string) (*CareTaskRow, error) { return nil, nil }

func (f *fakeStore) GetMemberPermissions(context.Context, string, string) (map[string]any, error) {
	return defaultPermissionScopes(), nil
}

func (f *fakeStore) UpsertMemberPermissions(context.Context, string, string, map[string]any) error {
	return nil
}

func (f *fakeStore) InsertAuditLog(context.Context, string, string, map[string]any, string) error {
	return nil
}

type fakeBilling struct {
	entitlementsResp  billing.EntitlementsResponse
	entitlementsUser  string
	entitlementsInput billing.BillingOverviewInput
	recordInput       billing.UsageRecordInput
	recordResp        billing.UsageRecord
	recordErr         error
}

func (f *fakeBilling) CurrentEntitlements(
	_ context.Context,
	userID string,
) (billing.EntitlementsResponse, error) {
	f.entitlementsUser = userID
	return f.entitlementsResp, nil
}

func (f *fakeBilling) CurrentEntitlementsForBilling(
	_ context.Context,
	input billing.BillingOverviewInput,
) (billing.EntitlementsResponse, error) {
	f.entitlementsInput = input
	f.entitlementsUser = input.UserID
	return f.entitlementsResp, nil
}

func (f *fakeBilling) RecordBillableUsage(
	_ context.Context,
	input billing.UsageRecordInput,
) (billing.UsageRecord, error) {
	f.recordInput = input
	if f.recordResp.ID == "" {
		f.recordResp = billing.UsageRecord{
			ID:        "usage-1",
			UserID:    input.UserID,
			UsageType: input.UsageType,
			Quantity:  input.Quantity,
			Unit:      input.Unit,
			Source:    input.Source,
			SourceID:  input.SourceID,
		}
	}
	return f.recordResp, f.recordErr
}

func fixedCareTime() time.Time {
	return time.Date(2026, 4, 29, 9, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
}

func TestServiceTimelineAggregatesEventsAndClampsWindow(t *testing.T) {
	store := &fakeStore{
		windowData: WindowData{
			Medications: []MedicationInfo{
				{ID: "med-1", Name: "Aspirin", Dosage: "1 pill", Active: true},
			},
			MedicationLogs: []MedicationLogEntry{
				{
					MedicationID: "med-1",
					TakenAt:      time.Date(2026, 4, 28, 8, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
					Skipped:      true,
				},
			},
			Symptoms: []SymptomEntry{
				{
					Symptom:  "dizziness",
					Severity: "moderate",
					LoggedAt: time.Date(2026, 4, 29, 7, 30, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
			Audits: []AuditEntry{
				{
					ToolName:  "emergency_alert",
					Params:    map[string]any{"lat": 10.5, "lng": 106.7},
					Result:    `{"status":"alert_sent","family_notified":2}`,
					CreatedAt: time.Date(2026, 4, 27, 10, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
			Calls: []CallUsageEntry{
				{
					UsageType:  "video_call_minutes",
					Quantity:   12,
					Unit:       "minutes",
					OccurredAt: time.Date(2026, 4, 26, 11, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
		},
	}
	billingSvc := &fakeBilling{
		entitlementsResp: billing.EntitlementsResponse{
			Entitlements: map[string]billing.EntitlementValue{
				"care_timeline_days": {LimitValue: float64(7)},
			},
		},
	}

	svc := NewService(store, billingSvc)
	svc.now = fixedCareTime

	resp, err := svc.Timeline(
		context.Background(),
		"family-1",
		"user-1",
		TimelineOptions{Days: 30, Limit: 10},
	)
	if err != nil {
		t.Fatalf("Timeline() error: %v", err)
	}
	if billingSvc.entitlementsUser != "family-1" {
		t.Fatalf("entitlements user = %q", billingSvc.entitlementsUser)
	}
	if billingSvc.entitlementsInput.CaredUserID != "user-1" {
		t.Fatalf("entitlements input = %+v", billingSvc.entitlementsInput)
	}
	if resp.EffectiveDays != 7 {
		t.Fatalf("effective days = %d", resp.EffectiveDays)
	}
	if want := time.Date(2026, 4, 23, 0, 0, 0, 0, time.FixedZone("ICT", 7*60*60)); !store.start.Equal(want) {
		t.Fatalf("start = %s, want %s", store.start, want)
	}
	if len(resp.Events) != 4 {
		t.Fatalf("events = %d, want 4", len(resp.Events))
	}
	if resp.Events[0].Kind != "symptom_logged" {
		t.Fatalf("first event kind = %q", resp.Events[0].Kind)
	}
	if resp.Events[1].Title != "Medication skipped: Aspirin" {
		t.Fatalf("second event title = %q", resp.Events[1].Title)
	}
	if resp.Events[2].Kind != "sos_alert" {
		t.Fatalf("third event kind = %q", resp.Events[2].Kind)
	}
	if resp.Events[3].Kind != "call_activity" {
		t.Fatalf("fourth event kind = %q", resp.Events[3].Kind)
	}
}

func TestServiceDailyBriefRequiresEntitlement(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakeBilling{
		entitlementsResp: billing.EntitlementsResponse{
			Entitlements: map[string]billing.EntitlementValue{
				"daily_brief_enabled": {LimitValue: false},
			},
		},
	})
	svc.now = fixedCareTime

	_, err := svc.DailyBrief(
		context.Background(),
		"family-1",
		"user-1",
		time.Time{},
	)
	if err == nil {
		t.Fatal("expected entitlement error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Status != 402 || apiErr.Code != "entitlement_required" {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
}

func TestServiceWeeklyReportBuildsDeterministicSummary(t *testing.T) {
	store := &fakeStore{
		windowData: WindowData{
			Medications: []MedicationInfo{
				{ID: "med-1", Name: "Aspirin", Dosage: "1 pill", Active: true},
			},
			MedicationLogs: []MedicationLogEntry{
				{
					MedicationID: "med-1",
					TakenAt:      time.Date(2026, 4, 24, 8, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
				{
					MedicationID: "med-1",
					TakenAt:      time.Date(2026, 4, 25, 8, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
					Skipped:      true,
				},
			},
			Symptoms: []SymptomEntry{
				{
					Symptom:  "dizziness",
					Severity: "moderate",
					LoggedAt: time.Date(2026, 4, 26, 9, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
			Metrics: []MetricEntry{
				{
					MetricType: "blood_pressure",
					Value:      "120/80",
					Unit:       "mmHg",
					LoggedAt:   time.Date(2026, 4, 28, 7, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
			Audits: []AuditEntry{
				{
					ToolName:  "emergency_alert",
					Result:    `{"status":"alert_sent","family_notified":2}`,
					CreatedAt: time.Date(2026, 4, 27, 10, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
				{
					ToolName:  "escalate_to_family",
					Result:    `{"status":"no_answer","tried":"Mom","remaining":["Dad"]}`,
					CreatedAt: time.Date(2026, 4, 27, 10, 10, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
			Calls: []CallUsageEntry{
				{
					UsageType:  "video_calls",
					Quantity:   1,
					Unit:       "calls",
					OccurredAt: time.Date(2026, 4, 27, 11, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
				{
					UsageType:  "video_call_minutes",
					Quantity:   15,
					Unit:       "minutes",
					OccurredAt: time.Date(2026, 4, 27, 11, 15, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
		},
	}
	svc := NewService(store, &fakeBilling{
		entitlementsResp: billing.EntitlementsResponse{
			Entitlements: map[string]billing.EntitlementValue{
				"weekly_report_enabled": {LimitValue: true},
			},
		},
	})
	svc.now = fixedCareTime

	resp, err := svc.WeeklyReport(
		context.Background(),
		"family-1",
		"user-1",
		time.Time{},
	)
	if err != nil {
		t.Fatalf("WeeklyReport() error: %v", err)
	}
	if resp.Totals.TakenMedications != 1 || resp.Totals.SkippedMedications != 1 {
		t.Fatalf("unexpected medication totals: %+v", resp.Totals)
	}
	if len(resp.Summary) != 6 {
		t.Fatalf("summary len = %d", len(resp.Summary))
	}
	if resp.Summary[0] != "Medication logs: 1 taken, 1 skipped across 1 active medications." {
		t.Fatalf("summary[0] = %q", resp.Summary[0])
	}
	if resp.Summary[2] != "Latest metrics: blood_pressure 120/80 mmHg." {
		t.Fatalf("summary[2] = %q", resp.Summary[2])
	}
	if resp.Summary[4] != "Incidents: 1 SOS alert, 1 escalation attempt." {
		t.Fatalf("summary[4] = %q", resp.Summary[4])
	}
}

func TestServiceIncidentReportUsesLatestEmergencyAudit(t *testing.T) {
	store := &fakeStore{
		windowData: WindowData{
			Audits: []AuditEntry{
				{
					ToolName:  "emergency_alert",
					Result:    `{"status":"alert_sent","family_notified":1,"location":{"lat":10.1,"lng":106.1}}`,
					CreatedAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
				{
					ToolName:  "emergency_alert",
					Params:    map[string]any{"lat": 10.7, "lng": 106.8},
					Result:    `{"status":"alert_sent","family_notified":3}`,
					CreatedAt: time.Date(2026, 4, 29, 8, 15, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
				{
					ToolName:  "escalate_to_family",
					Result:    `{"status":"answered","contact_name":"Mom"}`,
					CreatedAt: time.Date(2026, 4, 29, 8, 20, 0, 0, time.FixedZone("ICT", 7*60*60)),
				},
			},
		},
	}
	svc := NewService(store, &fakeBilling{
		entitlementsResp: billing.EntitlementsResponse{
			Entitlements: map[string]billing.EntitlementValue{
				"incident_report_enabled": {LimitValue: true},
			},
		},
	})
	svc.now = fixedCareTime

	resp, err := svc.IncidentReport(
		context.Background(),
		"family-1",
		"user-1",
	)
	if err != nil {
		t.Fatalf("IncidentReport() error: %v", err)
	}
	if resp.AlertAt.Day() != 29 {
		t.Fatalf("alert day = %d", resp.AlertAt.Day())
	}
	if resp.FamilyNotified != 3 {
		t.Fatalf("family_notified = %d", resp.FamilyNotified)
	}
	if len(resp.Escalations) != 1 || resp.Escalations[0].Status != "answered" {
		t.Fatalf("unexpected escalations: %+v", resp.Escalations)
	}
	if resp.Location.Lat != 10.7 || resp.Location.Lng != 106.8 {
		t.Fatalf("unexpected location: %+v", resp.Location)
	}
}

func TestServiceExportPDFRecordsUsageAndReturnsPlaceholderData(t *testing.T) {
	store := &fakeStore{
		windowData: WindowData{
			Medications: []MedicationInfo{
				{ID: "med-1", Name: "Aspirin", Dosage: "1 pill", Active: true},
			},
		},
	}
	billingSvc := &fakeBilling{
		entitlementsResp: billing.EntitlementsResponse{
			Entitlements: map[string]billing.EntitlementValue{
				"weekly_report_enabled":     {LimitValue: true},
				"weekly_report_pdf_enabled": {LimitValue: true},
				"export_format":             {LimitValue: "pdf_csv"},
			},
		},
	}
	svc := NewService(store, billingSvc)
	svc.now = fixedCareTime

	resp, err := svc.ExportPDF(
		context.Background(),
		"family-1",
		"user-1",
		ExportPDFRequest{ReportType: "weekly_report"},
	)
	if err != nil {
		t.Fatalf("ExportPDF() error: %v", err)
	}
	if billingSvc.recordInput.UsageType != billing.UsagePDFExports {
		t.Fatalf("usage type = %q", billingSvc.recordInput.UsageType)
	}
	if billingSvc.recordInput.UserID != "user-1" {
		t.Fatalf("usage user = %q", billingSvc.recordInput.UserID)
	}
	if billingSvc.recordInput.CaredUserID != "user-1" {
		t.Fatalf("usage billed context = %+v", billingSvc.recordInput)
	}
	if billingSvc.recordInput.Source != "care_dashboard_pdf_export" {
		t.Fatalf("usage source = %q", billingSvc.recordInput.Source)
	}
	if resp.Format != "pdf_placeholder" {
		t.Fatalf("format = %q", resp.Format)
	}
	if resp.UsageRecordID != "usage-1" {
		t.Fatalf("usage record id = %q", resp.UsageRecordID)
	}
	if resp.ReportType != "weekly_report" {
		t.Fatalf("report type = %q", resp.ReportType)
	}
}

func TestPutPermissionsDeniedWhenTargetNotLinked(t *testing.T) {
	store := &fakeStore{denyFamilyLink: true}
	bill := &fakeBilling{
		entitlementsResp: billing.EntitlementsResponse{
			Entitlements: map[string]billing.EntitlementValue{
				"permission_matrix_level": {LimitValue: "advanced"},
			},
		},
	}
	svc := NewService(store, bill)
	err := svc.PutPermissions(
		context.Background(),
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
		map[string]any{"timeline": "read"},
	)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 403 || apiErr.Code != "family_member_not_linked" {
		t.Fatalf("want family_member_not_linked 403, got %v", err)
	}
}

func TestFilterCareEventsForCoachQueryMatchesTokens(t *testing.T) {
	sum := "đau đầu nặng"
	rows := []CareEventRow{
		{ID: "1", EventType: "health.symptom.logged", Title: "Khác", Summary: &sum, OccurredAt: time.Now()},
		{ID: "2", EventType: "medication.taken", Title: "Thuốc", OccurredAt: time.Now()},
	}
	out := filterCareEventsForCoachQuery(rows, "đau đầu", 5)
	if len(out) != 1 || out[0].ID != "1" {
		t.Fatalf("want one matched row, got %+v", out)
	}
}
