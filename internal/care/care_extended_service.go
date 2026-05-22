package care

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/pkg/i18n"
)

func scopeAllows(scopes map[string]any, key string, needWrite bool) bool {
	raw, ok := scopes[key]
	if !ok {
		return !needWrite
	}
	s := strings.ToLower(entitlementLimitToString(raw))
	if s == "none" || s == "" {
		return false
	}
	if needWrite {
		return s == "write" || s == "admin"
	}
	return s == "read" || s == "write" || s == "admin"
}

func (s *Service) checkFamilyScope(
	ctx context.Context,
	requesterID, caredUserID, scope string,
	needWrite bool,
) error {
	if s.billing == nil {
		return nil
	}
	level, err := s.entitlementString(ctx, requesterID, caredUserID, "permission_matrix_level")
	if err != nil {
		return err
	}
	if level == "" || level == "none" {
		return nil
	}
	scopes, err := s.store.GetMemberPermissions(ctx, caredUserID, requesterID)
	if err != nil {
		return fmt.Errorf("member permissions: %w", err)
	}
	if !scopeAllows(scopes, scope, needWrite) {
		return &APIError{
			Status:  403,
			Code:    "permission_denied",
			Message: "Insufficient permission for this action (" + scope + ")",
		}
	}
	return nil
}

func (s *Service) SyncRuleBasedAlerts(ctx context.Context, userID string) error {
	_, end := s.windowForDay(time.Time{})
	start := end.AddDate(0, 0, -14)
	data, err := s.store.LoadWindow(ctx, userID, start, end)
	if err != nil {
		return err
	}
	skipped := 0
	for _, log := range data.MedicationLogs {
		if log.Skipped {
			skipped++
		}
	}
	if skipped >= 2 {
		if err := s.ensureAlert(ctx, userID, "medication_skipped_streak", "warning",
			"Medication adherence",
			strPtr(fmt.Sprintf("%d medication doses were skipped in the last 14 days.", skipped)),
			map[string]any{"skipped_count": skipped},
		); err != nil {
			return err
		}
	}
	for _, sy := range data.Symptoms {
		sev := strings.ToLower(strings.TrimSpace(sy.Severity))
		if sev == "high" || sev == "severe" || sev == "critical" {
			_ = s.ensureAlert(ctx, userID, "symptom_severity", "warning",
				"Symptom logged with elevated severity",
				strPtr(sy.Symptom+": "+sy.Severity),
				map[string]any{"symptom": sy.Symptom, "severity": sy.Severity},
			)
			break
		}
	}
	for _, ev := range data.CareEvents {
		if ev.EventType != "sos.triggered" {
			continue
		}
		if time.Since(ev.OccurredAt) <= 48*time.Hour {
			return s.ensureAlert(ctx, userID, "sos_recent", "critical",
				"Recent SOS event",
				strPtr("An SOS alert was triggered in the last 48 hours."),
				map[string]any{"event_id": ev.ID},
			)
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }

const alertSameRuleRecreateCooldown = 72 * time.Hour

func (s *Service) ensureAlert(
	ctx context.Context,
	userID, ruleCode, severity, title string,
	body *string,
	meta map[string]any,
) error {
	existing, err := s.store.ListCareAlerts(ctx, userID, "", 200)
	if err != nil {
		return err
	}
	now := s.now()
	for _, a := range existing {
		if a.RuleCode != ruleCode {
			continue
		}
		if a.Status == "open" {
			return nil
		}
		if now.Sub(a.UpdatedAt) < alertSameRuleRecreateCooldown {
			return nil
		}
	}
	id, err := s.store.InsertCareAlert(ctx, userID, ruleCode, severity, title, body, meta)
	if err != nil {
		return err
	}
	return s.store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    userID,
		ActorRole: "system",
		EventType: "alert.created",
		Severity:  severity,
		Title:     title,
		Summary:   body,
		Metadata:  map[string]any{"alert_id": id, "rule_code": ruleCode},
	})
}

func (s *Service) ListAlerts(
	ctx context.Context,
	requesterID, caredUserID, status string,
	limit int,
) ([]CareAlertRow, error) {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, caredUserID, "smart_alert_enabled",
		"Smart alerts are not available on this plan",
	); err != nil {
		return nil, err
	}
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "alerts", false); err != nil {
		return nil, err
	}
	if err := s.SyncRuleBasedAlerts(ctx, caredUserID); err != nil {
		return nil, err
	}
	return s.store.ListCareAlerts(ctx, caredUserID, status, limit)
}

func (s *Service) CaredUserIDForCareAlert(ctx context.Context, alertID string) (string, error) {
	al, err := s.store.GetCareAlertByID(ctx, alertID)
	if err != nil {
		return "", err
	}
	if al == nil {
		return "", &APIError{Status: 404, Code: "alert_not_found", Message: "Alert not found"}
	}
	return al.UserID, nil
}

func (s *Service) CaredUserIDForCareTask(ctx context.Context, taskID string) (string, error) {
	t, err := s.store.GetCareTaskByID(ctx, taskID)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", &APIError{Status: 404, Code: "task_not_found", Message: "Task not found"}
	}
	return t.UserID, nil
}

func (s *Service) AckAlert(ctx context.Context, requesterID, caredUserID, alertID string) error {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, caredUserID, "smart_alert_enabled",
		"Smart alerts are not available on this plan",
	); err != nil {
		return err
	}
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "alerts", true); err != nil {
		return err
	}
	al, err := s.store.GetCareAlertForUser(ctx, alertID, caredUserID)
	if err != nil || al == nil {
		return &APIError{Status: 404, Code: "alert_not_found", Message: "Alert not found"}
	}
	if err := s.store.UpdateCareAlertAck(ctx, alertID, requesterID); err != nil {
		if err == pgx.ErrNoRows {
			return &APIError{Status: 409, Code: "invalid_alert_state", Message: "Alert cannot be acknowledged"}
		}
		return err
	}
	return s.store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    caredUserID,
		ActorID:   &requesterID,
		ActorRole: "family",
		EventType: "alert.acknowledged",
		Severity:  "info",
		Title:     "Alert acknowledged",
		Metadata:  map[string]any{"alert_id": alertID},
	})
}

func (s *Service) ResolveAlert(ctx context.Context, requesterID, caredUserID, alertID string) error {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, caredUserID, "smart_alert_enabled",
		"Smart alerts are not available on this plan",
	); err != nil {
		return err
	}
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "alerts", true); err != nil {
		return err
	}
	al, err := s.store.GetCareAlertForUser(ctx, alertID, caredUserID)
	if err != nil || al == nil {
		return &APIError{Status: 404, Code: "alert_not_found", Message: "Alert not found"}
	}
	if err := s.store.UpdateCareAlertResolve(ctx, alertID, requesterID); err != nil {
		if err == pgx.ErrNoRows {
			return &APIError{Status: 409, Code: "invalid_alert_state", Message: "Alert cannot be resolved"}
		}
		return err
	}
	return s.store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    caredUserID,
		ActorID:   &requesterID,
		ActorRole: "family",
		EventType: "alert.resolved",
		Severity:  "info",
		Title:     "Alert resolved",
		Metadata:  map[string]any{"alert_id": alertID},
	})
}

func (s *Service) DismissAlert(ctx context.Context, requesterID, caredUserID, alertID string) error {
	if err := s.requireBoolEntitlement(
		ctx, requesterID, caredUserID, "smart_alert_enabled",
		"Smart alerts are not available on this plan",
	); err != nil {
		return err
	}
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "alerts", true); err != nil {
		return err
	}
	al, err := s.store.GetCareAlertForUser(ctx, alertID, caredUserID)
	if err != nil || al == nil {
		return &APIError{Status: 404, Code: "alert_not_found", Message: "Alert not found"}
	}
	if err := s.store.UpdateCareAlertDismiss(ctx, alertID, requesterID); err != nil {
		if err == pgx.ErrNoRows {
			return &APIError{Status: 409, Code: "invalid_alert_state", Message: "Alert cannot be dismissed"}
		}
		return err
	}
	return s.store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    caredUserID,
		ActorID:   &requesterID,
		ActorRole: "family",
		EventType: "alert.dismissed",
		Severity:  "info",
		Title:     "Alert dismissed",
		Metadata:  map[string]any{"alert_id": alertID, "outcome": "dismissed"},
	})
}

func (s *Service) ListTasks(ctx context.Context, requesterID, caredUserID string, limit int) ([]CareTaskRow, error) {
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "tasks", false); err != nil {
		return nil, err
	}
	return s.store.ListCareTasks(ctx, caredUserID, limit)
}

type TaskCreateInput struct {
	Title string     `json:"title"`
	DueAt *time.Time `json:"due_at,omitempty"`
	Meta  map[string]any
}

func (s *Service) CreateTask(
	ctx context.Context,
	requesterID, caredUserID string,
	in TaskCreateInput,
) (string, error) {
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "tasks", true); err != nil {
		return "", err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return "", &APIError{Status: 400, Code: "invalid_title", Message: "title is required"}
	}
	id, err := s.store.InsertCareTask(ctx, caredUserID, requesterID, title, in.DueAt, in.Meta)
	if err != nil {
		return "", err
	}
	_ = s.store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    caredUserID,
		ActorID:   &requesterID,
		ActorRole: "family",
		EventType: "task.created",
		Severity:  "info",
		Title:     "Task created: " + title,
		Metadata:  map[string]any{"task_id": id},
	})
	return id, nil
}

type TaskUpdateInput struct {
	Title  string     `json:"title"`
	DueAt  *time.Time `json:"due_at,omitempty"`
	Status string     `json:"status,omitempty"`
	Meta   map[string]any
}

func (s *Service) UpdateTask(
	ctx context.Context,
	requesterID, caredUserID, taskID string,
	in TaskUpdateInput,
) error {
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "tasks", true); err != nil {
		return err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return &APIError{Status: 400, Code: "invalid_title", Message: "title is required"}
	}
	return s.store.UpdateCareTask(ctx, taskID, caredUserID, title, in.DueAt, in.Status, in.Meta)
}

func (s *Service) CompleteTask(ctx context.Context, requesterID, caredUserID, taskID string) error {
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "tasks", true); err != nil {
		return err
	}
	if err := s.store.CompleteCareTask(ctx, taskID, caredUserID); err != nil {
		if err == pgx.ErrNoRows {
			return &APIError{Status: 404, Code: "task_not_found", Message: "Task not found or not open"}
		}
		return err
	}
	return s.store.InsertCareEvent(ctx, InsertCareEventInput{
		UserID:    caredUserID,
		ActorID:   &requesterID,
		ActorRole: "family",
		EventType: "task.completed",
		Severity:  "info",
		Title:     "Task completed",
		Metadata:  map[string]any{"task_id": taskID},
	})
}

func (s *Service) GetPermissions(
	ctx context.Context,
	requesterID, caredUserID string,
) (map[string]any, error) {
	return s.store.GetMemberPermissions(ctx, caredUserID, requesterID)
}

func (s *Service) PutPermissions(
	ctx context.Context,
	requesterID, caredUserID, familyMemberID string,
	scopes map[string]any,
) error {
	level, err := s.entitlementString(ctx, requesterID, caredUserID, "permission_matrix_level")
	if err != nil {
		return err
	}
	if level != "advanced" && level != "basic" {
		return &APIError{
			Status:  402,
			Code:    "permission_matrix_not_available",
			Message: "Permission matrix is not available on this plan",
		}
	}
	if _, perr := uuid.Parse(strings.TrimSpace(familyMemberID)); perr != nil {
		return &APIError{Status: 400, Code: "invalid_family_member", Message: "family_id must be a valid user UUID"}
	}
	linked, err := s.store.HasFamilyLinkBetween(ctx, caredUserID, familyMemberID)
	if err != nil {
		return err
	}
	if !linked {
		return &APIError{
			Status:  403,
			Code:    "family_member_not_linked",
			Message: "Target user is not linked to this cared user via family_links",
		}
	}
	return s.store.UpsertMemberPermissions(ctx, caredUserID, familyMemberID, scopes)
}

type CoachCitation struct {
	CareEventID string    `json:"care_event_id"`
	EventType   string    `json:"event_type"`
	OccurredAt  time.Time `json:"occurred_at"`
	Excerpt     string    `json:"excerpt"`
}

type CoachQueryResponse struct {
	Answer      string          `json:"answer"`
	Citations   []CoachCitation `json:"citations"`
	Disclaimer  string          `json:"disclaimer"`
	UsageRecord string          `json:"usage_record_id,omitempty"`
}

func (s *Service) CoachQuery(
	ctx context.Context,
	requesterID, caredUserID, question string,
) (CoachQueryResponse, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return CoachQueryResponse{}, &APIError{Status: 400, Code: "invalid_question", Message: "question is required"}
	}
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "timeline", false); err != nil {
		return CoachQueryResponse{}, err
	}
	if err := s.requireBillingOrDevBypass(); err != nil {
		return CoachQueryResponse{}, err
	}
	var usageID string
	if s.billing != nil {
		rec, err := s.billing.RecordBillableUsage(ctx, billing.UsageRecordInput{
			UserID:      caredUserID,
			CaredUserID: caredUserID,
			UsageType:   billing.UsageCareCoachQuestions,
			Quantity:    1,
			Unit:        "questions",
			Source:      "care_coach_query",
			SourceID:    uuid.NewString(),
			Metadata:    map[string]any{"requester_id": requesterID},
		})
		if err != nil {
			return CoachQueryResponse{}, err
		}
		usageID = rec.ID
	}
	rows, err := s.store.ListCareEventsForCitation(ctx, caredUserID, 48)
	if err != nil {
		return CoachQueryResponse{}, err
	}
	rows = filterCareEventsForCoachQuery(rows, q, 12)
	cites := make([]CoachCitation, 0, len(rows))
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		ex := r.Title
		if r.Summary != nil && strings.TrimSpace(*r.Summary) != "" {
			ex = ex + ": " + *r.Summary
		}
		cites = append(cites, CoachCitation{
			CareEventID: r.ID,
			EventType:   r.EventType,
			OccurredAt:  r.OccurredAt,
			Excerpt:     ex,
		})
		parts = append(parts, fmt.Sprintf("- [%s] %s", r.EventType, ex))
	}
	lang := i18n.FromContext(ctx)
	answer := i18n.T(lang, "care.coach.fallback_recent")
	if len(parts) > 0 {
		answer = i18n.T(lang, "care.coach.timeline_intro") +
			strings.Join(parts, "\n") +
			i18n.T(lang, "care.coach.timeline_outro")
	}
	return CoachQueryResponse{
		Answer:      answer,
		Citations:   cites,
		Disclaimer:  i18n.T(lang, "care.coach.disclaimer"),
		UsageRecord: usageID,
	}, nil
}

type PatternInsight struct {
	Code   string  `json:"code"`
	Title  string  `json:"title"`
	Detail string  `json:"detail"`
	Score  float64 `json:"score,omitempty"`
}

type PatternRunResponse struct {
	Insights    []PatternInsight `json:"insights"`
	UsageRecord string           `json:"usage_record_id,omitempty"`
}

func (s *Service) RunPatterns(
	ctx context.Context,
	requesterID, caredUserID string,
) (PatternRunResponse, error) {
	if err := s.checkFamilyScope(ctx, requesterID, caredUserID, "timeline", false); err != nil {
		return PatternRunResponse{}, err
	}
	if err := s.requireBillingOrDevBypass(); err != nil {
		return PatternRunResponse{}, err
	}
	var usageID string
	if s.billing != nil {
		rec, err := s.billing.RecordBillableUsage(ctx, billing.UsageRecordInput{
			UserID:      caredUserID,
			CaredUserID: caredUserID,
			UsageType:   billing.UsagePatternDetectionRuns,
			Quantity:    1,
			Unit:        "runs",
			Source:      "care_pattern_detection",
			SourceID:    uuid.NewString(),
			Metadata:    map[string]any{"requester_id": requesterID},
		})
		if err != nil {
			return PatternRunResponse{}, err
		}
		usageID = rec.ID
	}
	_, end := s.windowForDay(time.Time{})
	start := end.AddDate(0, 0, -30)
	data, err := s.store.LoadWindow(ctx, caredUserID, start, end)
	if err != nil {
		return PatternRunResponse{}, err
	}
	insights := make([]PatternInsight, 0)
	sosN := 0
	for _, a := range data.Audits {
		if a.ToolName == "emergency_alert" {
			sosN++
		}
	}
	if sosN >= 2 {
		insights = append(insights, PatternInsight{
			Code:   "sos_frequency",
			Title:  "Multiple SOS events",
			Detail: fmt.Sprintf("%d SOS alerts in the last 30 days.", sosN),
			Score:  float64(sosN),
		})
	}
	skipped := 0
	for _, l := range data.MedicationLogs {
		if l.Skipped {
			skipped++
		}
	}
	if skipped > 0 {
		insights = append(insights, PatternInsight{
			Code:   "medication_skips",
			Title:  "Medication skips",
			Detail: fmt.Sprintf("%d skipped doses in the last 30 days.", skipped),
			Score:  float64(skipped),
		})
	}
	return PatternRunResponse{Insights: insights, UsageRecord: usageID}, nil
}

func filterCareEventsForCoachQuery(rows []CareEventRow, question string, limit int) []CareEventRow {
	if len(rows) == 0 || limit <= 0 {
		return rows
	}
	q := strings.ToLower(strings.TrimSpace(question))
	tokens := coachQueryTokens(q)
	if len(tokens) == 0 {
		if len(rows) > limit {
			return rows[:limit]
		}
		return rows
	}
	out := make([]CareEventRow, 0, limit)
	for _, r := range rows {
		hay := strings.ToLower(r.EventType + " " + r.Title)
		if r.Summary != nil {
			hay += " " + strings.ToLower(*r.Summary)
		}
		matched := false
		for _, t := range tokens {
			if strings.Contains(hay, t) {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, r)
			if len(out) >= limit {
				return out
			}
		}
	}
	if len(out) == 0 {
		if len(rows) > limit {
			return rows[:limit]
		}
		return rows
	}
	return out
}

func coachQueryTokens(lower string) []string {
	fields := strings.Fields(lower)
	var out []string
	for _, f := range fields {
		if utf8.RuneCountInString(f) >= 2 {
			out = append(out, f)
		}
	}
	return out
}
