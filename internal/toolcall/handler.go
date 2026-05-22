package toolcall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/appointment"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/call"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/internal/emergency"
	"github.com/team-dang/aiseebackend/internal/health"
	"github.com/team-dang/aiseebackend/internal/medication"
	"github.com/team-dang/aiseebackend/internal/music"
	"github.com/team-dang/aiseebackend/internal/user"
	"github.com/team-dang/aiseebackend/internal/volunteer"
	"github.com/team-dang/aiseebackend/pkg/i18n"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type ToolCallRequest struct {
	Tool   string         `json:"tool"`
	UserID string         `json:"user_id"`
	Params map[string]any `json:"params"`
}

type Handler struct {
	chat        *chat.Service
	call        *call.Service
	user        *user.Service
	medication  *medication.Service
	health      *health.Service
	appointment *appointment.Service
	emergency   *emergency.Service
	music       *music.Service
	volunteer   *volunteer.Service
	billing     usageService
	queries     *dbgen.Queries
}

type usageService interface {
	RequireQuota(context.Context, billing.QuotaCheckInput) error
	RecordUsage(context.Context, billing.UsageRecordInput) (billing.UsageRecord, error)
}

func NewHandler(
	chatSvc *chat.Service,
	callSvc *call.Service,
	userSvc *user.Service,
	medSvc *medication.Service,
	healthSvc *health.Service,
	aptSvc *appointment.Service,
	emergSvc *emergency.Service,
	musicSvc *music.Service,
	volSvc *volunteer.Service,
	billingSvc usageService,
	queries *dbgen.Queries,
) *Handler {
	return &Handler{
		chat:        chatSvc,
		call:        callSvc,
		user:        userSvc,
		medication:  medSvc,
		health:      healthSvc,
		appointment: aptSvc,
		emergency:   emergSvc,
		music:       musicSvc,
		volunteer:   volSvc,
		billing:     billingSvc,
		queries:     queries,
	}
}

func (h *Handler) HandleToolCall(
	w http.ResponseWriter, r *http.Request,
) {
	var req ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(
			w, http.StatusBadRequest,
			"invalid_body", "Invalid request body",
		)
		return
	}

	slog.Info("tool call received",
		"tool", req.Tool, "user_id", req.UserID,
	)

	result, err := h.dispatch(r.Context(), req)

	// Audit log (best-effort)
	h.logAudit(r.Context(), req, result, err)

	if err != nil {
		if h.writeQuotaError(w, err) {
			return
		}
		response.Err(
			w, http.StatusBadRequest, "tool_error", err.Error(),
		)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) dispatch(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	switch req.Tool {
	case "call_family_contact":
		return h.handleCallFamilyContact(ctx, req)
	case "call_support_contact":
		return h.handleCallSupportContact(ctx, req)
	case "make_voice_call":
		return h.handleMakeCall(ctx, req, "voice")
	case "make_video_call":
		return h.handleMakeCall(ctx, req, "video")
	case "send_chat_message":
		return h.handleSendMessage(ctx, req)
	case "read_unread_messages":
		return h.handleReadUnread(ctx, req)
	case "list_family_contacts":
		return h.handleListFamilyContacts(ctx, req)
	case "set_medication_reminder":
		return h.handleSetMedication(ctx, req)
	case "list_medication_reminders":
		return h.handleListMedications(ctx, req)
	case "mark_medication_taken":
		return h.handleMarkMedicationTaken(ctx, req)
	case "mark_medication_skipped":
		return h.handleMarkMedicationSkipped(ctx, req)
	case "log_health_symptom":
		return h.handleLogSymptom(ctx, req)
	case "log_health_metric":
		return h.handleLogMetric(ctx, req)
	case "set_appointment_reminder":
		return h.handleSetAppointment(ctx, req)
	case "emergency_alert":
		return h.handleEmergencyAlert(ctx, req)
	case "escalate_to_family":
		return h.handleEscalateToFamily(ctx, req)
	case "play_music":
		return h.handlePlayMusic(ctx, req)
	case "escalate_to_volunteer":
		return h.handleEscalateToVolunteer(ctx, req)
	case "rate_volunteer":
		return h.handleRateVolunteer(ctx, req)
	case "update_location":
		return h.handleUpdateLocation(ctx, req)
	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Tool)
	}
}

const assistantCallAnswerTimeoutSeconds = 30

func (h *Handler) handleMakeCall(
	ctx context.Context, req ToolCallRequest, callType string,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)
	if contactName == "" {
		return nil, fmt.Errorf("contact_name is required")
	}

	member, err := h.user.ResolveContact(
		ctx, req.UserID, contactName,
	)
	if err != nil {
		return nil, err
	}

	caller, err := h.user.GetMe(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get caller: %w", err)
	}

	result, err := h.call.CreateRoom(
		ctx, caller, member.User, callType,
	)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}

	return map[string]string{
		"status":      "room_created",
		"room_name":   result.RoomName,
		"token":       result.Token,
		"callee_name": member.User.DisplayName,
	}, nil
}

func (h *Handler) handleCallFamilyContact(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName := strings.TrimSpace(stringParam(req.Params, "contact_name"))
	if contactName == "" {
		return map[string]any{
			"status":  "invalid_request",
			"mode":    "family",
			"message": "contact_name is required",
		}, nil
	}

	member, err := h.user.ResolveContact(ctx, req.UserID, contactName)
	if err != nil {
		return h.familyResolveError(ctx, req.UserID, contactName, err), nil
	}

	caller, err := h.user.GetMe(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get caller: %w", err)
	}

	callType := normalizeAssistantCallType(stringParam(req.Params, "call_type"))
	result, err := h.call.CreateRoom(ctx, caller, member.User, callType)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}

	resp := map[string]any{
		"status":                 "prepared",
		"mode":                   "family",
		"room_name":              result.RoomName,
		"token":                  result.Token,
		"call_type":              callType,
		"contact_id":             pgutil.PgUUIDToString(member.User.ID),
		"contact_name":           displayNameOrFallback(member.User.DisplayName, contactName),
		"contact_role":           "family",
		"relationship":           member.Relationship,
		"answer_timeout_seconds": assistantCallAnswerTimeoutSeconds,
	}
	if fallbackContacts, err := h.familyFallbackContacts(ctx, req.UserID, pgutil.PgUUIDToString(member.User.ID)); err == nil && len(fallbackContacts) > 0 {
		resp["fallback_contacts"] = fallbackContacts
	}
	if member.User.AvatarUrl.Valid {
		resp["avatar_url"] = member.User.AvatarUrl.String
	}
	return resp, nil
}

func (h *Handler) handleCallSupportContact(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName := strings.TrimSpace(stringParam(req.Params, "contact_name"))
	if contactName != "" && !isGenericSupportRequest(contactName) {
		return h.handleCallFamilyContact(ctx, req)
	}

	callType := normalizeAssistantCallType(stringParam(req.Params, "call_type"))
	result, err := h.volunteer.StartAssistantMatch(ctx, req.UserID, callType)
	if err != nil {
		return nil, fmt.Errorf("start assistant volunteer match: %w", err)
	}

	resp := map[string]any{
		"status":    result.Status,
		"mode":      "volunteer",
		"call_type": callType,
	}
	if result.MatchID != "" {
		resp["match_id"] = result.MatchID
	}
	if result.Message != "" {
		resp["message"] = result.Message
	}
	if result.Status != "matched" {
		return resp, nil
	}

	resp["status"] = "prepared"
	resp["room_name"] = result.RoomName
	resp["token"] = result.UserToken
	resp["contact_id"] = result.VolunteerID
	resp["contact_name"] = displayNameOrFallback(
		result.VolunteerName,
		i18n.T(i18n.FromContext(ctx), "toolcall.fallback_helper_name"),
	)
	resp["contact_role"] = "volunteer"
	resp["answer_timeout_seconds"] = assistantCallAnswerTimeoutSeconds
	return resp, nil
}

func (h *Handler) handleSendMessage(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)
	message, _ := req.Params["message"].(string)
	if contactName == "" || message == "" {
		return nil, fmt.Errorf(
			"contact_name and message are required",
		)
	}

	member, err := h.user.ResolveContact(
		ctx, req.UserID, contactName,
	)
	if err != nil {
		return nil, err
	}

	receiverID := pgutil.PgUUIDToString(member.User.ID)
	msg, err := h.chat.SendMessage(
		ctx, req.UserID, receiverID, message, "", "",
	)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	return map[string]any{
		"status":        "sent",
		"message_id":    pgutil.PgUUIDToString(msg.ID),
		"receiver_name": member.User.DisplayName,
	}, nil
}

func (h *Handler) handleReadUnread(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)

	var msgs []dbgen.Message
	var err error

	if contactName != "" {
		member, resolveErr := h.user.ResolveContact(
			ctx, req.UserID, contactName,
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		senderID := pgutil.PgUUIDToString(member.User.ID)
		msgs, err = h.chat.GetUnreadByContact(
			ctx, req.UserID, senderID,
		)
	} else {
		msgs, err = h.chat.GetUnreadMessages(ctx, req.UserID)
	}

	if err != nil {
		return nil, fmt.Errorf("get unread: %w", err)
	}

	type msgInfo struct {
		SenderName string `json:"sender_name"`
		Content    string `json:"content"`
		SentAt     string `json:"sent_at"`
	}
	var results []msgInfo
	for _, m := range msgs {
		sender, _ := h.queries.GetUserByID(ctx, m.SenderID)
		results = append(results, msgInfo{
			SenderName: sender.DisplayName,
			Content:    m.ContentText.String,
			SentAt: m.CreatedAt.Time.Format(
				"2006-01-02T15:04:05+07:00",
			),
		})
	}

	return map[string]any{
		"messages": results,
		"count":    len(results),
	}, nil
}

func (h *Handler) handleListFamilyContacts(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	uid, err := pgutil.ParsePgUUID(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	rows, err := h.queries.GetFamilyMembersOrdered(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get family members: %w", err)
	}

	return buildFamilyContactsResponse(rows), nil
}

func (h *Handler) handleSetMedication(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	name, _ := req.Params["medicine_name"].(string)
	dosage, _ := req.Params["dosage"].(string)
	frequency, _ := req.Params["frequency"].(string)
	if name == "" || dosage == "" {
		return nil, fmt.Errorf("medicine_name and dosage are required")
	}
	if frequency == "" {
		frequency = "daily"
	}

	rawTimes, _ := req.Params["times"].([]any)
	times := make([]string, 0, len(rawTimes))
	for _, t := range rawTimes {
		if s, ok := t.(string); ok {
			times = append(times, s)
		}
	}

	med, err := h.medication.Create(
		ctx, req.UserID, name, dosage, frequency, times,
	)
	if err != nil {
		return nil, fmt.Errorf("create medication: %w", err)
	}

	return map[string]any{
		"status": "created",
		"medication": map[string]any{
			"name":   med.Name,
			"dosage": med.Dosage,
			"times":  med.Times,
		},
		"schedule_action": map[string]any{
			"action": "schedule_reminder",
			"params": map[string]any{
				"type":      "medication",
				"name":      med.Name,
				"times":     med.Times,
				"frequency": med.Frequency,
			},
		},
	}, nil
}

func (h *Handler) handleListMedications(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	meds, err := h.medication.ListWithStatus(
		ctx, req.UserID,
	)
	if err != nil {
		return nil, fmt.Errorf("list medications: %w", err)
	}

	type medInfo struct {
		Name       string   `json:"name"`
		Dosage     string   `json:"dosage"`
		Times      []string `json:"times"`
		TakenToday bool     `json:"today_taken"`
	}
	results := make([]medInfo, 0, len(meds))
	for _, m := range meds {
		results = append(results, medInfo{
			Name:       m.Medication.Name,
			Dosage:     m.Medication.Dosage,
			Times:      m.Medication.Times,
			TakenToday: m.TakenToday,
		})
	}

	return map[string]any{
		"medications": results,
		"count":       len(results),
	}, nil
}

func (h *Handler) handleMarkMedicationTaken(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	name, _ := req.Params["medicine_name"].(string)
	if name == "" {
		return nil, fmt.Errorf("medicine_name is required")
	}

	med, err := h.medication.FindByName(
		ctx, req.UserID, name,
	)
	if err != nil {
		return nil, err
	}

	medID := pgutil.PgUUIDToString(med.ID)
	_, err = h.medication.MarkTaken(
		ctx, req.UserID, medID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark taken: %w", err)
	}

	return map[string]any{
		"status": "taken",
		"name":   med.Name,
	}, nil
}

func (h *Handler) handleMarkMedicationSkipped(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	name, _ := req.Params["medicine_name"].(string)
	if name == "" {
		return nil, fmt.Errorf("medicine_name is required")
	}

	med, err := h.medication.FindByName(
		ctx, req.UserID, name,
	)
	if err != nil {
		return nil, err
	}

	medID := pgutil.PgUUIDToString(med.ID)
	_, err = h.medication.MarkSkipped(
		ctx, req.UserID, medID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark skipped: %w", err)
	}

	return map[string]any{
		"status": "skipped",
		"name":   med.Name,
	}, nil
}

func (h *Handler) handleLogSymptom(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	symptom, _ := req.Params["symptom"].(string)
	severity, _ := req.Params["severity"].(string)
	notes, _ := req.Params["notes"].(string)
	if symptom == "" {
		return nil, fmt.Errorf("symptom is required")
	}

	entry, err := h.health.LogSymptom(
		ctx, req.UserID, symptom, severity, notes,
	)
	if err != nil {
		return nil, fmt.Errorf("log symptom: %w", err)
	}

	return map[string]any{
		"status":  "logged",
		"symptom": entry.Symptom,
		"id":      pgutil.PgUUIDToString(entry.ID),
	}, nil
}

func (h *Handler) handleLogMetric(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	metricType, _ := req.Params["metric_type"].(string)
	value, _ := req.Params["value"].(string)
	unit, _ := req.Params["unit"].(string)
	if metricType == "" || value == "" {
		return nil, fmt.Errorf(
			"metric_type and value are required",
		)
	}

	entry, err := h.health.LogMetric(
		ctx, req.UserID, metricType, value, unit,
	)
	if err != nil {
		return nil, fmt.Errorf("log metric: %w", err)
	}

	return map[string]any{
		"status":      "logged",
		"metric_type": entry.MetricType,
		"value":       entry.Value,
		"id":          pgutil.PgUUIDToString(entry.ID),
	}, nil
}

var hcmTZ = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic("load timezone Asia/Ho_Chi_Minh: " + err.Error())
	}
	return loc
}()

func (h *Handler) handleSetAppointment(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	title, _ := req.Params["title"].(string)
	datetimeStr, _ := req.Params["datetime"].(string)
	location, _ := req.Params["location"].(string)
	notes, _ := req.Params["notes"].(string)
	if title == "" || datetimeStr == "" {
		return nil, fmt.Errorf(
			"title and datetime are required",
		)
	}

	dt, err := parseVNDatetime(datetimeStr)
	if err != nil {
		return nil, fmt.Errorf("parse datetime: %w", err)
	}

	apt, err := h.appointment.Create(
		ctx, req.UserID, title, location, dt, notes,
	)
	if err != nil {
		return nil, fmt.Errorf("create appointment: %w", err)
	}

	formatted := apt.Datetime.Time.In(hcmTZ).Format(
		"2006-01-02 15:04",
	)

	return map[string]any{
		"status": "created",
		"appointment": map[string]any{
			"title":    apt.Title,
			"datetime": formatted,
			"location": location,
		},
		"schedule_action": map[string]any{
			"action": "schedule_reminder",
			"params": map[string]any{
				"type":     "appointment",
				"title":    apt.Title,
				"datetime": datetimeStr,
			},
		},
	}, nil
}

func parseVNDatetime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	t, err := time.ParseInLocation(
		"2006-01-02 15:04", s, hcmTZ,
	)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"expected YYYY-MM-DD HH:MM or RFC3339, got %q",
			s,
		)
	}
	return t, nil
}

func (h *Handler) handleEmergencyAlert(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	lat, _ := req.Params["lat"].(float64)
	lng, _ := req.Params["lng"].(float64)

	result, err := h.emergency.Alert(ctx, req.UserID, lat, lng)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":          "alert_sent",
		"room_name":       result.RoomName,
		"token":           result.Token,
		"family_notified": result.FamilyNotified,
		"location": map[string]float64{
			"lat": result.Lat,
			"lng": result.Lng,
		},
	}, nil
}

func (h *Handler) handleEscalateToFamily(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	contactName, _ := req.Params["contact_name"].(string)

	var skip []string
	if rawSkip, ok := req.Params["skip"].([]any); ok {
		for _, s := range rawSkip {
			if name, ok := s.(string); ok {
				skip = append(skip, name)
			}
		}
	}

	result, err := h.emergency.Escalate(
		ctx, req.UserID, contactName, skip,
	)
	if err != nil {
		return nil, err
	}

	resp := map[string]any{
		"status": result.Status,
	}
	switch result.Status {
	case "prepared":
		resp["mode"] = "family"
		resp["call_type"] = "voice"
		resp["room_name"] = result.RoomName
		resp["token"] = result.Token
		resp["contact_id"] = result.ContactID
		resp["contact_name"] = result.ContactName
		resp["contact_role"] = "family"
		resp["relationship"] = result.Relationship
		resp["answer_timeout_seconds"] = assistantCallAnswerTimeoutSeconds
		if result.AvatarURL != "" {
			resp["avatar_url"] = result.AvatarURL
		}
		if len(result.Remaining) > 0 {
			resp["remaining"] = result.Remaining
		}
	case "no_contacts":
		// status only
	}
	return resp, nil
}

func (h *Handler) handlePlayMusic(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	query, _ := req.Params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if err := h.requireQuota(
		ctx, req.UserID, billing.UsageMusicSessions, 1,
	); err != nil {
		return nil, err
	}
	result, err := h.music.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := h.recordUsage(ctx, billing.UsageRecordInput{
		UserID:    req.UserID,
		UsageType: billing.UsageMusicSessions,
		Quantity:  1,
		Unit:      "sessions",
		Source:    "agent",
		SourceID:  query,
		Metadata: map[string]any{
			"title": result.Title,
		},
	}); err != nil {
		return nil, err
	}

	return map[string]any{
		"status": "found",
		"title":  result.Title,
		"play_action": map[string]any{
			"action": "play_audio",
			"params": map[string]string{
				"url":   result.URL,
				"title": result.Title,
			},
		},
	}, nil
}

func (h *Handler) requireQuota(
	ctx context.Context,
	userID string,
	usageType billing.UsageType,
	quantity float64,
) error {
	if h.billing == nil {
		return nil
	}
	return h.billing.RequireQuota(ctx, billing.QuotaCheckInput{
		UserID:    userID,
		UsageType: usageType,
		Quantity:  quantity,
	})
}

func (h *Handler) recordUsage(
	ctx context.Context,
	input billing.UsageRecordInput,
) error {
	if h.billing == nil {
		return nil
	}
	_, err := h.billing.RecordUsage(ctx, input)
	return err
}

func (h *Handler) writeQuotaError(w http.ResponseWriter, err error) bool {
	var quotaErr *billing.QuotaExceededError
	if !errors.As(err, &quotaErr) {
		return false
	}
	response.JSON(w, http.StatusPaymentRequired, map[string]any{
		"error": map[string]any{
			"code":         quotaErr.Code,
			"message":      "Quota exceeded",
			"usage_type":   quotaErr.UsageType,
			"required":     quotaErr.Required,
			"remaining":    quotaErr.Remaining,
			"plan_code":    quotaErr.PlanCode,
			"reset_period": quotaErr.ResetPeriod,
		},
	})
	return true
}

func (h *Handler) handleEscalateToVolunteer(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	result, err := h.volunteer.StartMatch(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf(
			"start volunteer match: %w", err,
		)
	}
	return map[string]any{
		"status":   "searching",
		"match_id": result.MatchID,
		"poll_url": "/api/v1/volunteers/match/" +
			result.MatchID + "/status",
	}, nil
}

func (h *Handler) handleRateVolunteer(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	matchID, _ := req.Params["match_id"].(string)
	score, _ := req.Params["score"].(float64)
	volunteerID, _ := req.Params["volunteer_id"].(string)

	if matchID == "" || volunteerID == "" ||
		score < 1 || score > 5 {
		return nil, fmt.Errorf(
			"match_id, volunteer_id, and score (1-5) required",
		)
	}

	if err := h.volunteer.Rate(
		ctx, volunteerID, req.UserID,
		matchID, int16(score),
	); err != nil {
		return nil, err
	}

	return map[string]any{
		"status": "rated",
		"score":  int(score),
	}, nil
}

func (h *Handler) handleUpdateLocation(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	lat, _ := req.Params["lat"].(float64)
	lng, _ := req.Params["lng"].(float64)

	uid, err := pgutil.ParsePgUUID(req.UserID)
	if err != nil {
		return nil, err
	}

	if err := h.queries.UpdateUserLocation(
		ctx, dbgen.UpdateUserLocationParams{
			ID:      uid,
			LastLat: pgtype.Float8{Float64: lat, Valid: true},
			LastLng: pgtype.Float8{Float64: lng, Valid: true},
		},
	); err != nil {
		return nil, fmt.Errorf("update location: %w", err)
	}

	return map[string]any{
		"status": "location_updated",
		"lat":    lat,
		"lng":    lng,
	}, nil
}

func buildFamilyContactsResponse(
	rows []dbgen.GetFamilyMembersOrderedRow,
) map[string]any {
	contacts := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		displayName := row.DisplayName
		if displayName == "" {
			displayName = row.Email
		}
		contacts = append(contacts, map[string]any{
			"id":                 pgutil.PgUUIDToString(row.ID),
			"display_name":       displayName,
			"relationship":       row.Relationship,
			"role":               row.Role,
			"is_primary_contact": row.IsPrimaryContact,
		})
	}

	return map[string]any{
		"status":          "ok",
		"has_family_link": len(contacts) > 0,
		"count":           len(contacts),
		"contacts":        contacts,
	}
}

func stringParam(params map[string]any, key string) string {
	raw, _ := params[key].(string)
	return raw
}

func normalizeAssistantCallType(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "video") {
		return "video"
	}
	return "voice"
}

func familyResolveError(contactName string, err error) map[string]any {
	message := err.Error()
	status := "contact_not_found"
	switch {
	case strings.Contains(message, "multiple contacts match"):
		status = "multiple_contacts"
	case strings.Contains(message, "contact not found"):
		status = "contact_not_found"
	}
	return map[string]any{
		"status":                 status,
		"mode":                   "family",
		"requested_contact_name": contactName,
		"message":                message,
	}
}

func (h *Handler) familyFallbackContacts(
	ctx context.Context,
	userID string,
	excludeUserID string,
) ([]map[string]any, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	rows, err := h.queries.GetFamilyMembersOrdered(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get family members ordered: %w", err)
	}
	return buildFamilyFallbackContacts(rows, excludeUserID), nil
}

func (h *Handler) familyResolveError(
	ctx context.Context,
	userID string,
	contactName string,
	err error,
) map[string]any {
	resp := familyResolveError(contactName, err)
	if fallbackContacts, fallbackErr := h.familyFallbackContacts(ctx, userID, ""); fallbackErr == nil && len(fallbackContacts) > 0 {
		resp["fallback_contacts"] = fallbackContacts
	}
	return resp
}

func displayNameOrFallback(displayName, fallback string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	return fallback
}

func isGenericSupportRequest(contactName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(contactName))
	if normalized == "" {
		return true
	}
	genericTerms := []string{
		"ho tro",
		"nguoi ho tro",
		"nguoi giup",
		"nguoi tro giup",
		"tinh nguyen vien",
		"volunteer",
		"support",
	}
	for _, term := range genericTerms {
		if strings.Contains(normalized, term) {
			return true
		}
	}
	return false
}

func buildFamilyFallbackContacts(
	rows []dbgen.GetFamilyMembersOrderedRow,
	excludeUserID string,
) []map[string]any {
	fallback := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := pgutil.PgUUIDToString(row.ID)
		if excludeUserID != "" && id == excludeUserID {
			continue
		}
		entry := map[string]any{
			"id":                 id,
			"display_name":       displayNameOrFallback(row.DisplayName, row.Email),
			"role":               row.Role,
			"relationship":       row.Relationship,
			"is_primary_contact": row.IsPrimaryContact,
		}
		if row.AvatarUrl.Valid {
			entry["avatar_url"] = row.AvatarUrl.String
		}
		fallback = append(fallback, entry)
		if len(fallback) >= 3 {
			break
		}
	}
	return fallback
}

func (h *Handler) logAudit(
	ctx context.Context,
	req ToolCallRequest,
	result any,
	toolErr error,
) {
	uid, err := pgutil.ParsePgUUID(req.UserID)
	if err != nil {
		return
	}

	paramsJSON, _ := json.Marshal(req.Params)
	var resultStr string
	if toolErr != nil {
		resultStr = fmt.Sprintf("error: %s", toolErr.Error())
	} else {
		resultJSON, _ := json.Marshal(result)
		resultStr = string(resultJSON)
	}

	_ = h.queries.CreateAuditLog(ctx, dbgen.CreateAuditLogParams{
		UserID:   uid,
		ToolName: req.Tool,
		Params:   paramsJSON,
		Result:   pgtype.Text{String: resultStr, Valid: true},
	})
}
