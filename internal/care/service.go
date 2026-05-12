package care

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/team-dang/aiseebackend/internal/billing"
)

type Store interface {
	LoadWindow(
		ctx context.Context,
		userID string,
		start time.Time,
		end time.Time,
	) (WindowData, error)
}

type BillingClient interface {
	CurrentEntitlements(
		ctx context.Context,
		userID string,
	) (billing.EntitlementsResponse, error)
	RecordBillableUsage(
		ctx context.Context,
		input billing.UsageRecordInput,
	) (billing.UsageRecord, error)
}

type Service struct {
	store   Store
	billing BillingClient
	now     func() time.Time
	loc     *time.Location
}

type WindowData struct {
	Medications    []MedicationInfo
	MedicationLogs []MedicationLogEntry
	Symptoms       []SymptomEntry
	Metrics        []MetricEntry
	Audits         []AuditEntry
	Calls          []CallUsageEntry
}

type MedicationInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Dosage string `json:"dosage"`
	Active bool   `json:"active"`
}

type MedicationLogEntry struct {
	MedicationID string    `json:"medication_id"`
	TakenAt      time.Time `json:"taken_at"`
	Skipped      bool      `json:"skipped"`
}

type SymptomEntry struct {
	Symptom  string    `json:"symptom"`
	Severity string    `json:"severity,omitempty"`
	Notes    string    `json:"notes,omitempty"`
	LoggedAt time.Time `json:"logged_at"`
}

type MetricEntry struct {
	MetricType string    `json:"metric_type"`
	Value      string    `json:"value"`
	Unit       string    `json:"unit,omitempty"`
	LoggedAt   time.Time `json:"logged_at"`
}

type AuditEntry struct {
	ToolName  string         `json:"tool_name"`
	Params    map[string]any `json:"params,omitempty"`
	Result    string         `json:"result,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type CallUsageEntry struct {
	UsageType  string         `json:"usage_type"`
	Quantity   float64        `json:"quantity"`
	Unit       string         `json:"unit"`
	Source     string         `json:"source,omitempty"`
	SourceID   string         `json:"source_id,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type TimelineOptions struct {
	Days  int `json:"days"`
	Limit int `json:"limit"`
}

type TimelineEvent struct {
	Kind       string         `json:"kind"`
	Title      string         `json:"title"`
	Detail     string         `json:"detail,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type TimelineResponse struct {
	RequestedDays int             `json:"requested_days"`
	EffectiveDays int             `json:"effective_days"`
	Limit         int             `json:"limit"`
	WindowStart   time.Time       `json:"window_start"`
	WindowEnd     time.Time       `json:"window_end"`
	Events        []TimelineEvent `json:"events"`
}

type ReportTotals struct {
	ActiveMedications  int     `json:"active_medications"`
	TakenMedications   int     `json:"taken_medications"`
	SkippedMedications int     `json:"skipped_medications"`
	SymptomCount       int     `json:"symptom_count"`
	MetricCount        int     `json:"metric_count"`
	SOSAlertCount      int     `json:"sos_alert_count"`
	EscalationCount    int     `json:"escalation_count"`
	VideoCallCount     int     `json:"video_call_count"`
	VideoCallMinutes   float64 `json:"video_call_minutes"`
	VoiceCallMinutes   float64 `json:"voice_call_minutes"`
}

type DailyBriefResponse struct {
	Date    string       `json:"date"`
	Summary []string     `json:"summary"`
	Totals  ReportTotals `json:"totals"`
}

type WeeklyReportResponse struct {
	StartDate string       `json:"start_date"`
	EndDate   string       `json:"end_date"`
	Summary   []string     `json:"summary"`
	Totals    ReportTotals `json:"totals"`
}

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type IncidentEscalation struct {
	OccurredAt  time.Time `json:"occurred_at"`
	Status      string    `json:"status"`
	ContactName string    `json:"contact_name,omitempty"`
	Tried       string    `json:"tried,omitempty"`
	Remaining   []string  `json:"remaining,omitempty"`
	RawToolName string    `json:"raw_tool_name,omitempty"`
}

type IncidentReportResponse struct {
	AlertAt        time.Time            `json:"alert_at"`
	FamilyNotified int                  `json:"family_notified"`
	Location       GeoPoint             `json:"location"`
	Escalations    []IncidentEscalation `json:"escalations"`
	Summary        []string             `json:"summary"`
}

type ExportPDFRequest struct {
	ReportType string `json:"report_type"`
	Date       string `json:"date,omitempty"`
	EndDate    string `json:"end_date,omitempty"`
}

type ExportResponse struct {
	ExportID      string    `json:"export_id"`
	ReportType    string    `json:"report_type"`
	Format        string    `json:"format"`
	UsageRecordID string    `json:"usage_record_id,omitempty"`
	GeneratedAt   time.Time `json:"generated_at"`
	Data          any       `json:"data,omitempty"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

func NewService(store Store, billingClient BillingClient) *Service {
	return &Service{
		store:   store,
		billing: billingClient,
		now:     time.Now,
		loc:     careLocation(),
	}
}

func (s *Service) Timeline(
	ctx context.Context,
	requesterID string,
	userID string,
	opts TimelineOptions,
) (TimelineResponse, error) {
	requestedDays := opts.Days
	if requestedDays <= 0 {
		requestedDays = 7
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	effectiveDays := requestedDays
	if s.billing != nil {
		allowedDays, err := s.entitlementInt(
			ctx, requesterID, "care_timeline_days",
		)
		if err != nil {
			return TimelineResponse{}, err
		}
		if allowedDays <= 0 {
			return TimelineResponse{}, entitlementError(
				"care_timeline_days",
				"Care timeline is not available on this plan",
			)
		}
		if effectiveDays > allowedDays {
			effectiveDays = allowedDays
		}
	}

	start, end := s.windowForDays(effectiveDays, s.now())
	data, err := s.store.LoadWindow(ctx, userID, start, end)
	if err != nil {
		return TimelineResponse{}, fmt.Errorf("load care window: %w", err)
	}

	events := buildTimelineEvents(data)
	if len(events) > limit {
		events = events[:limit]
	}

	return TimelineResponse{
		RequestedDays: requestedDays,
		EffectiveDays: effectiveDays,
		Limit:         limit,
		WindowStart:   start,
		WindowEnd:     end,
		Events:        events,
	}, nil
}

func (s *Service) DailyBrief(
	ctx context.Context,
	requesterID string,
	userID string,
	day time.Time,
) (DailyBriefResponse, error) {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, "daily_brief_enabled",
		"Daily brief is not available on this plan",
	); err != nil {
		return DailyBriefResponse{}, err
	}

	start, end := s.windowForDay(day)
	data, err := s.store.LoadWindow(ctx, userID, start, end)
	if err != nil {
		return DailyBriefResponse{}, fmt.Errorf("load daily brief window: %w", err)
	}
	totals, summary := summarizeWindow(data)
	return DailyBriefResponse{
		Date:    start.Format("2006-01-02"),
		Summary: summary,
		Totals:  totals,
	}, nil
}

func (s *Service) WeeklyReport(
	ctx context.Context,
	requesterID string,
	userID string,
	endDate time.Time,
) (WeeklyReportResponse, error) {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, "weekly_report_enabled",
		"Weekly report is not available on this plan",
	); err != nil {
		return WeeklyReportResponse{}, err
	}

	start, end := s.windowForWeek(endDate)
	data, err := s.store.LoadWindow(ctx, userID, start, end)
	if err != nil {
		return WeeklyReportResponse{}, fmt.Errorf("load weekly report window: %w", err)
	}
	totals, summary := summarizeWindow(data)
	return WeeklyReportResponse{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Add(-time.Nanosecond).Format("2006-01-02"),
		Summary:   summary,
		Totals:    totals,
	}, nil
}

func (s *Service) IncidentReport(
	ctx context.Context,
	requesterID string,
	userID string,
) (IncidentReportResponse, error) {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, "incident_report_enabled",
		"Incident report is not available on this plan",
	); err != nil {
		return IncidentReportResponse{}, err
	}

	_, end := s.windowForDay(time.Time{})
	start := end.AddDate(0, 0, -30)
	data, err := s.store.LoadWindow(ctx, userID, start, end)
	if err != nil {
		return IncidentReportResponse{}, fmt.Errorf("load incident window: %w", err)
	}

	alert, ok := latestEmergencyAlert(data.Audits)
	if !ok {
		return IncidentReportResponse{}, &APIError{
			Status:  404,
			Code:    "incident_not_found",
			Message: "No SOS incident found",
		}
	}

	resp := IncidentReportResponse{
		AlertAt:        alert.At,
		FamilyNotified: alert.FamilyNotified,
		Location:       alert.Location,
		Escalations:    collectEscalations(data.Audits, alert.At),
	}
	resp.Summary = []string{
		fmt.Sprintf(
			"SOS alert sent at %s with %d family notifications.",
			alert.At.In(s.loc).Format(time.RFC3339),
			alert.FamilyNotified,
		),
		fmt.Sprintf(
			"Escalation attempts recorded: %d.",
			len(resp.Escalations),
		),
	}
	return resp, nil
}

func (s *Service) ExportPDF(
	ctx context.Context,
	requesterID string,
	userID string,
	req ExportPDFRequest,
) (ExportResponse, error) {
	reportType := strings.TrimSpace(req.ReportType)
	if reportType == "" {
		return ExportResponse{}, &APIError{
			Status:  400,
			Code:    "invalid_report_type",
			Message: "report_type is required",
		}
	}

	var reportData any
	var err error
	switch reportType {
	case "daily_brief":
		var day time.Time
		if strings.TrimSpace(req.Date) != "" {
			day, err = parseCareDate(req.Date, s.loc)
			if err != nil {
				return ExportResponse{}, invalidInputError(err.Error())
			}
		}
		reportData, err = s.DailyBrief(ctx, requesterID, userID, day)
	case "weekly_report":
		var endDate time.Time
		if strings.TrimSpace(req.EndDate) != "" {
			endDate, err = parseCareDate(req.EndDate, s.loc)
			if err != nil {
				return ExportResponse{}, invalidInputError(err.Error())
			}
		}
		reportData, err = s.WeeklyReport(ctx, requesterID, userID, endDate)
	case "incident_report":
		reportData, err = s.IncidentReport(ctx, requesterID, userID)
	default:
		return ExportResponse{}, &APIError{
			Status:  400,
			Code:    "invalid_report_type",
			Message: "report_type must be one of daily_brief, weekly_report, incident_report",
		}
	}
	if err != nil {
		return ExportResponse{}, err
	}

	var usageRecordID string
	if s.billing != nil {
		record, err := s.billing.RecordBillableUsage(
			ctx, billing.UsageRecordInput{
				UserID:    requesterID,
				UsageType: billing.UsagePDFExports,
				Quantity:  1,
				Unit:      "exports",
				Source:    "care_dashboard_pdf_export",
				SourceID:  uuid.NewString(),
				Metadata: map[string]any{
					"cared_user_id": userID,
					"report_type":   reportType,
				},
			},
		)
		if err != nil {
			return ExportResponse{}, err
		}
		usageRecordID = record.ID
	}

	return ExportResponse{
		ExportID:      uuid.NewString(),
		ReportType:    reportType,
		Format:        "pdf_placeholder",
		UsageRecordID: usageRecordID,
		GeneratedAt:   s.now().In(s.loc),
		Data:          reportData,
	}, nil
}

func (s *Service) windowForDays(days int, ref time.Time) (time.Time, time.Time) {
	end := startOfDay(ref.In(s.loc), s.loc).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -days)
	return start, end
}

func (s *Service) windowForDay(day time.Time) (time.Time, time.Time) {
	ref := day
	if ref.IsZero() {
		ref = s.now()
	}
	start := startOfDay(ref.In(s.loc), s.loc)
	return start, start.AddDate(0, 0, 1)
}

func (s *Service) windowForWeek(endDate time.Time) (time.Time, time.Time) {
	ref := endDate
	if ref.IsZero() {
		ref = s.now()
	}
	end := startOfDay(ref.In(s.loc), s.loc).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -7)
	return start, end
}

func (s *Service) requireBoolEntitlement(
	ctx context.Context,
	requesterID string,
	key string,
	message string,
) error {
	if s.billing == nil {
		return nil
	}
	value, err := s.entitlementBool(ctx, requesterID, key)
	if err != nil {
		return err
	}
	if !value {
		return entitlementError(key, message)
	}
	return nil
}

func (s *Service) entitlementBool(
	ctx context.Context,
	requesterID string,
	key string,
) (bool, error) {
	resp, err := s.billing.CurrentEntitlements(ctx, requesterID)
	if err != nil {
		return false, fmt.Errorf("current entitlements: %w", err)
	}
	value, ok := resp.Entitlements[key]
	if !ok {
		return false, nil
	}
	return asBool(value.LimitValue), nil
}

func (s *Service) entitlementInt(
	ctx context.Context,
	requesterID string,
	key string,
) (int, error) {
	resp, err := s.billing.CurrentEntitlements(ctx, requesterID)
	if err != nil {
		return 0, fmt.Errorf("current entitlements: %w", err)
	}
	value, ok := resp.Entitlements[key]
	if !ok {
		return 0, nil
	}
	return asInt(value.LimitValue), nil
}

func buildTimelineEvents(data WindowData) []TimelineEvent {
	medicationNames := make(map[string]string, len(data.Medications))
	activeMedications := 0
	for _, med := range data.Medications {
		medicationNames[med.ID] = med.Name
		if med.Active {
			activeMedications++
		}
	}
	_ = activeMedications

	events := make([]TimelineEvent, 0, len(data.MedicationLogs)+len(data.Symptoms)+len(data.Metrics)+len(data.Audits)+len(data.Calls))
	for _, symptom := range data.Symptoms {
		title := "Symptom logged: " + symptom.Symptom
		detail := strings.TrimSpace(symptom.Severity)
		events = append(events, TimelineEvent{
			Kind:       "symptom_logged",
			Title:      title,
			Detail:     detail,
			OccurredAt: symptom.LoggedAt,
		})
	}
	for _, metric := range data.Metrics {
		title := fmt.Sprintf("Metric logged: %s %s", metric.MetricType, metric.Value)
		if metric.Unit != "" {
			title += " " + metric.Unit
		}
		events = append(events, TimelineEvent{
			Kind:       "metric_logged",
			Title:      title,
			OccurredAt: metric.LoggedAt,
		})
	}
	for _, log := range data.MedicationLogs {
		name := medicationNames[log.MedicationID]
		if name == "" {
			name = "Unknown medication"
		}
		kind := "medication_taken"
		title := "Medication taken: " + name
		if log.Skipped {
			kind = "medication_skipped"
			title = "Medication skipped: " + name
		}
		events = append(events, TimelineEvent{
			Kind:       kind,
			Title:      title,
			OccurredAt: log.TakenAt,
		})
	}
	for _, audit := range data.Audits {
		switch audit.ToolName {
		case "emergency_alert":
			detail := ""
			payload := parseJSONMap(audit.Result)
			if notified := asInt(payload["family_notified"]); notified > 0 {
				detail = fmt.Sprintf("Family notified: %d", notified)
			}
			events = append(events, TimelineEvent{
				Kind:       "sos_alert",
				Title:      "SOS alert sent",
				Detail:     detail,
				OccurredAt: audit.CreatedAt,
			})
		case "escalate_to_family":
			detail := escalationDetail(parseJSONMap(audit.Result))
			events = append(events, TimelineEvent{
				Kind:       "sos_escalation",
				Title:      "SOS escalated to family",
				Detail:     detail,
				OccurredAt: audit.CreatedAt,
			})
		}
	}
	for _, call := range data.Calls {
		events = append(events, TimelineEvent{
			Kind:       "call_activity",
			Title:      callTitle(call),
			OccurredAt: call.OccurredAt,
		})
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].Title < events[j].Title
		}
		return events[i].OccurredAt.After(events[j].OccurredAt)
	})
	return events
}

func summarizeWindow(data WindowData) (ReportTotals, []string) {
	totals := ReportTotals{}
	for _, med := range data.Medications {
		if med.Active {
			totals.ActiveMedications++
		}
	}
	for _, log := range data.MedicationLogs {
		if log.Skipped {
			totals.SkippedMedications++
			continue
		}
		totals.TakenMedications++
	}
	totals.SymptomCount = len(data.Symptoms)
	totals.MetricCount = len(data.Metrics)

	sosCount := 0
	escalationCount := 0
	for _, audit := range data.Audits {
		switch audit.ToolName {
		case "emergency_alert":
			sosCount++
		case "escalate_to_family":
			escalationCount++
		}
	}
	totals.SOSAlertCount = sosCount
	totals.EscalationCount = escalationCount

	for _, call := range data.Calls {
		switch call.UsageType {
		case "video_calls":
			totals.VideoCallCount += int(call.Quantity)
		case "video_call_minutes":
			totals.VideoCallMinutes += call.Quantity
		case "voice_call_minutes":
			totals.VoiceCallMinutes += call.Quantity
		}
	}

	summary := []string{
		fmt.Sprintf(
			"Medication logs: %d taken, %d skipped across %d active medications.",
			totals.TakenMedications,
			totals.SkippedMedications,
			totals.ActiveMedications,
		),
		symptomSummary(data.Symptoms),
		metricSummary(data.Metrics),
		callSummary(totals),
		incidentSummary(totals),
	}
	return totals, summary
}

type emergencyAlert struct {
	At             time.Time
	FamilyNotified int
	Location       GeoPoint
}

func latestEmergencyAlert(audits []AuditEntry) (emergencyAlert, bool) {
	var latest emergencyAlert
	found := false
	for _, audit := range audits {
		if audit.ToolName != "emergency_alert" {
			continue
		}
		if !found || audit.CreatedAt.After(latest.At) {
			payload := parseJSONMap(audit.Result)
			location := geoFromMap(payload["location"])
			if location == (GeoPoint{}) {
				location = geoFromParams(audit.Params)
			}
			latest = emergencyAlert{
				At:             audit.CreatedAt,
				FamilyNotified: asInt(payload["family_notified"]),
				Location:       location,
			}
			found = true
		}
	}
	return latest, found
}

func collectEscalations(audits []AuditEntry, alertAt time.Time) []IncidentEscalation {
	windowEnd := alertAt.Add(2 * time.Hour)
	escalations := make([]IncidentEscalation, 0)
	for _, audit := range audits {
		if audit.ToolName != "escalate_to_family" {
			continue
		}
		if audit.CreatedAt.Before(alertAt) || audit.CreatedAt.After(windowEnd) {
			continue
		}
		payload := parseJSONMap(audit.Result)
		escalation := IncidentEscalation{
			OccurredAt:  audit.CreatedAt,
			Status:      strings.TrimSpace(asString(payload["status"])),
			ContactName: strings.TrimSpace(asString(payload["contact_name"])),
			Tried:       strings.TrimSpace(asString(payload["tried"])),
			Remaining:   asStringSlice(payload["remaining"]),
			RawToolName: audit.ToolName,
		}
		escalations = append(escalations, escalation)
	}
	sort.SliceStable(escalations, func(i, j int) bool {
		return escalations[i].OccurredAt.Before(escalations[j].OccurredAt)
	})
	return escalations
}

func symptomSummary(symptoms []SymptomEntry) string {
	if len(symptoms) == 0 {
		return "No symptoms logged."
	}
	sort.SliceStable(symptoms, func(i, j int) bool {
		if symptoms[i].LoggedAt.Equal(symptoms[j].LoggedAt) {
			return symptoms[i].Symptom < symptoms[j].Symptom
		}
		return symptoms[i].LoggedAt.Before(symptoms[j].LoggedAt)
	})
	parts := make([]string, 0, len(symptoms))
	for _, symptom := range symptoms {
		if strings.TrimSpace(symptom.Severity) != "" {
			parts = append(parts, fmt.Sprintf("%s (%s)", symptom.Symptom, symptom.Severity))
			continue
		}
		parts = append(parts, symptom.Symptom)
	}
	return "Symptoms logged: " + strings.Join(parts, ", ") + "."
}

func metricSummary(metrics []MetricEntry) string {
	if len(metrics) == 0 {
		return "No metrics logged."
	}
	latestByType := make(map[string]MetricEntry, len(metrics))
	for _, metric := range metrics {
		current, ok := latestByType[metric.MetricType]
		if !ok || metric.LoggedAt.After(current.LoggedAt) {
			latestByType[metric.MetricType] = metric
		}
	}
	keys := make([]string, 0, len(latestByType))
	for key := range latestByType {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		metric := latestByType[key]
		part := fmt.Sprintf("%s %s", metric.MetricType, metric.Value)
		if metric.Unit != "" {
			part += " " + metric.Unit
		}
		parts = append(parts, part)
	}
	return "Latest metrics: " + strings.Join(parts, "; ") + "."
}

func callSummary(totals ReportTotals) string {
	parts := make([]string, 0, 3)
	if totals.VideoCallCount > 0 {
		parts = append(parts, fmt.Sprintf("%d video call", totals.VideoCallCount))
		if totals.VideoCallCount != 1 {
			parts[len(parts)-1] += "s"
		}
	}
	if totals.VideoCallMinutes > 0 {
		parts = append(parts, fmt.Sprintf("%.1f video minutes", totals.VideoCallMinutes))
	}
	if totals.VoiceCallMinutes > 0 {
		parts = append(parts, fmt.Sprintf("%.1f voice minutes", totals.VoiceCallMinutes))
	}
	if len(parts) == 0 {
		return "No call activity recorded."
	}
	return "Calls: " + strings.Join(parts, ", ") + "."
}

func incidentSummary(totals ReportTotals) string {
	return fmt.Sprintf(
		"Incidents: %d SOS alert%s, %d escalation %s.",
		totals.SOSAlertCount,
		pluralSuffix(totals.SOSAlertCount),
		totals.EscalationCount,
		pluralWord("attempt", totals.EscalationCount),
	)
}

func entitlementError(featureKey, message string) *APIError {
	return &APIError{
		Status:  402,
		Code:    "entitlement_required",
		Message: message + " (" + featureKey + ")",
	}
}

func invalidInputError(message string) *APIError {
	return &APIError{
		Status:  400,
		Code:    "invalid_input",
		Message: message,
	}
}

func parseCareDate(value string, loc *time.Location) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid date %q, expected YYYY-MM-DD",
			value,
		)
	}
	return parsed, nil
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	inLoc := t.In(loc)
	return time.Date(
		inLoc.Year(), inLoc.Month(), inLoc.Day(),
		0, 0, 0, 0, loc,
	)
}

func careLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.UTC
	}
	return loc
}

func parseJSONMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil
	}
	return value
}

func geoFromMap(value any) GeoPoint {
	m, ok := value.(map[string]any)
	if !ok {
		return GeoPoint{}
	}
	return GeoPoint{
		Lat: asFloat64(m["lat"]),
		Lng: asFloat64(m["lng"]),
	}
}

func geoFromParams(params map[string]any) GeoPoint {
	if params == nil {
		return GeoPoint{}
	}
	return GeoPoint{
		Lat: asFloat64(params["lat"]),
		Lng: asFloat64(params["lng"]),
	}
}

func escalationDetail(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if contact := strings.TrimSpace(asString(payload["contact_name"])); contact != "" {
		return contact
	}
	if tried := strings.TrimSpace(asString(payload["tried"])); tried != "" {
		return tried
	}
	return strings.TrimSpace(asString(payload["status"]))
}

func callTitle(call CallUsageEntry) string {
	switch call.UsageType {
	case "video_call_minutes":
		return fmt.Sprintf("Video call activity: %.1f minutes", call.Quantity)
	case "voice_call_minutes":
		return fmt.Sprintf("Voice call activity: %.1f minutes", call.Quantity)
	case "video_calls":
		return fmt.Sprintf("Video call activity: %.0f call%s", call.Quantity, pluralSuffixInt(int(call.Quantity)))
	default:
		return "Call activity recorded"
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func asInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	default:
		return 0
	}
}

func asFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func asStringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, entry := range raw {
		if text := strings.TrimSpace(asString(entry)); text != "" {
			items = append(items, text)
		}
	}
	return items
}

func pluralSuffix(value int) string {
	if value == 1 {
		return ""
	}
	return "s"
}

func pluralSuffixInt(value int) string {
	if value == 1 {
		return ""
	}
	return "s"
}

func pluralWord(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return nil, false
	}
	return apiErr, true
}
