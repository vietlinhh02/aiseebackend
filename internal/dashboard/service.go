package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
)

// Querier abstracts the sqlc-generated queries used by this
// package, enabling stub-based testing.
type Querier interface {
	GetMedicationsByUser(
		ctx context.Context, uid pgtype.UUID,
	) ([]dbgen.Medication, error)
	GetMedicationLogsToday(
		ctx context.Context, uid pgtype.UUID,
	) ([]dbgen.MedicationLog, error)
	GetRecentSymptoms(
		ctx context.Context, p dbgen.GetRecentSymptomsParams,
	) ([]dbgen.HealthSymptom, error)
	GetRecentMetrics(
		ctx context.Context, p dbgen.GetRecentMetricsParams,
	) ([]dbgen.HealthMetric, error)
	GetAuditLogsByUser(
		ctx context.Context, p dbgen.GetAuditLogsByUserParams,
	) ([]dbgen.AuditLog, error)
	GetUserLocation(
		ctx context.Context, uid pgtype.UUID,
	) (dbgen.GetUserLocationRow, error)
	GetAppointmentsByUser(
		ctx context.Context, uid pgtype.UUID,
	) ([]dbgen.Appointment, error)
	CreateMedication(
		ctx context.Context, p dbgen.CreateMedicationParams,
	) (dbgen.Medication, error)
	UpdateMedication(
		ctx context.Context, p dbgen.UpdateMedicationParams,
	) (dbgen.Medication, error)
	DeactivateMedication(
		ctx context.Context, p dbgen.DeactivateMedicationParams,
	) error
	CreateAuditLog(
		ctx context.Context, p dbgen.CreateAuditLogParams,
	) error
}

type Service struct {
	q Querier
}

func NewService(q Querier) *Service {
	return &Service{q: q}
}

// --- Response types ---

type MedWithAdherence struct {
	Medication dbgen.Medication `json:"medication"`
	TakenToday bool             `json:"taken_today"`
}

type HealthResponse struct {
	Medications    []MedWithAdherence    `json:"medications"`
	RecentSymptoms []dbgen.HealthSymptom `json:"recent_symptoms"`
	RecentMetrics  []dbgen.HealthMetric  `json:"recent_metrics"`
}

type MedStatus struct {
	Medication dbgen.Medication `json:"medication"`
	Taken      bool             `json:"taken"`
	Skipped    bool             `json:"skipped"`
}

type MedicationsResponse struct {
	Medications []MedStatus `json:"medications"`
	Count       int         `json:"count"`
}

type LocationResponse struct {
	Lat       pgtype.Float8      `json:"lat"`
	Lng       pgtype.Float8      `json:"lng"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

type CreateMedRequest struct {
	Name      string   `json:"name"`
	Dosage    string   `json:"dosage"`
	Times     []string `json:"times"`
	Frequency string   `json:"frequency"`
}

type UpdateMedRequest struct {
	Name      string   `json:"name"`
	Dosage    string   `json:"dosage"`
	Times     []string `json:"times"`
	Frequency string   `json:"frequency"`
}

// Health returns active medications, today's adherence,
// recent symptoms, and recent metrics for a user.
func (s *Service) Health(
	ctx context.Context, userID string,
) (HealthResponse, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return HealthResponse{}, err
	}

	meds, err := s.q.GetMedicationsByUser(ctx, uid)
	if err != nil {
		return HealthResponse{},
			fmt.Errorf("get medications: %w", err)
	}

	logs, err := s.q.GetMedicationLogsToday(ctx, uid)
	if err != nil {
		return HealthResponse{},
			fmt.Errorf("get medication logs: %w", err)
	}

	takenSet := make(map[pgtype.UUID]bool)
	for _, log := range logs {
		if !(log.Skipped.Valid && log.Skipped.Bool) {
			takenSet[log.MedicationID] = true
		}
	}

	medsWithAdherence := make(
		[]MedWithAdherence, 0, len(meds),
	)
	for _, med := range meds {
		medsWithAdherence = append(
			medsWithAdherence, MedWithAdherence{
				Medication: med,
				TakenToday: takenSet[med.ID],
			},
		)
	}

	symptoms, err := s.q.GetRecentSymptoms(
		ctx, dbgen.GetRecentSymptomsParams{
			UserID: uid, Limit: 10,
		},
	)
	if err != nil {
		return HealthResponse{},
			fmt.Errorf("get symptoms: %w", err)
	}

	metrics, err := s.q.GetRecentMetrics(
		ctx, dbgen.GetRecentMetricsParams{
			UserID: uid, Limit: 10,
		},
	)
	if err != nil {
		return HealthResponse{},
			fmt.Errorf("get metrics: %w", err)
	}

	return HealthResponse{
		Medications:    medsWithAdherence,
		RecentSymptoms: symptoms,
		RecentMetrics:  metrics,
	}, nil
}

// Medications returns the full medication list with today's
// taken/skipped status per medication.
func (s *Service) Medications(
	ctx context.Context, userID string,
) (MedicationsResponse, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return MedicationsResponse{}, err
	}

	meds, err := s.q.GetMedicationsByUser(ctx, uid)
	if err != nil {
		return MedicationsResponse{},
			fmt.Errorf("get medications: %w", err)
	}

	logs, err := s.q.GetMedicationLogsToday(ctx, uid)
	if err != nil {
		return MedicationsResponse{},
			fmt.Errorf("get medication logs: %w", err)
	}

	takenSet := make(map[pgtype.UUID]bool)
	skippedSet := make(map[pgtype.UUID]bool)
	for _, log := range logs {
		if log.Skipped.Valid && log.Skipped.Bool {
			skippedSet[log.MedicationID] = true
		} else {
			takenSet[log.MedicationID] = true
		}
	}

	result := make([]MedStatus, 0, len(meds))
	for _, med := range meds {
		result = append(result, MedStatus{
			Medication: med,
			Taken:      takenSet[med.ID],
			Skipped:    skippedSet[med.ID],
		})
	}

	return MedicationsResponse{Medications: result, Count: len(result)}, nil
}

// Appointments returns upcoming appointments for a user.
func (s *Service) Appointments(
	ctx context.Context, userID string,
) ([]dbgen.Appointment, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}

	return s.q.GetAppointmentsByUser(ctx, uid)
}

// AuditLog returns paginated audit logs for a user.
func (s *Service) AuditLog(
	ctx context.Context,
	userID string,
	page, limit int32,
) ([]dbgen.AuditLog, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * limit
	return s.q.GetAuditLogsByUser(
		ctx, dbgen.GetAuditLogsByUserParams{
			UserID: uid,
			Limit:  limit,
			Offset: offset,
		},
	)
}

// Location returns the user's last known location.
func (s *Service) Location(
	ctx context.Context, userID string,
) (LocationResponse, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return LocationResponse{}, err
	}

	row, err := s.q.GetUserLocation(ctx, uid)
	if err != nil {
		return LocationResponse{},
			fmt.Errorf("get user location: %w", err)
	}

	return LocationResponse{
		Lat:       row.LastLat,
		Lng:       row.LastLng,
		UpdatedAt: row.LastLocationAt,
	}, nil
}

// CreateMedication creates a medication for a user and
// logs the action to the audit trail.
func (s *Service) CreateMedication(
	ctx context.Context,
	userID, familyID string,
	req CreateMedRequest,
) (dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Medication{}, err
	}

	freq := req.Frequency
	if freq == "" {
		freq = "daily"
	}

	med, err := s.q.CreateMedication(
		ctx, dbgen.CreateMedicationParams{
			UserID:    uid,
			Name:      req.Name,
			Dosage:    req.Dosage,
			Times:     req.Times,
			Frequency: freq,
		},
	)
	if err != nil {
		return dbgen.Medication{},
			fmt.Errorf("create medication: %w", err)
	}

	s.logFamilyAudit(
		ctx, userID, familyID,
		"family_create_medication",
		map[string]any{
			"name":   req.Name,
			"dosage": req.Dosage,
		},
	)

	return med, nil
}

// UpdateMedication updates a medication and logs the action.
func (s *Service) UpdateMedication(
	ctx context.Context,
	userID, medID, familyID string,
	req UpdateMedRequest,
) (dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Medication{}, err
	}
	muid, err := pgutil.ParsePgUUID(medID)
	if err != nil {
		return dbgen.Medication{}, err
	}

	freq := req.Frequency
	if freq == "" {
		freq = "daily"
	}

	med, err := s.q.UpdateMedication(
		ctx, dbgen.UpdateMedicationParams{
			ID:        muid,
			UserID:    uid,
			Name:      req.Name,
			Dosage:    req.Dosage,
			Times:     req.Times,
			Frequency: freq,
		},
	)
	if err != nil {
		return dbgen.Medication{},
			fmt.Errorf("update medication: %w", err)
	}

	s.logFamilyAudit(
		ctx, userID, familyID,
		"family_update_medication",
		map[string]any{
			"medication_id": medID,
			"name":          req.Name,
		},
	)

	return med, nil
}

// DeleteMedication soft-deletes a medication and logs the
// action.
func (s *Service) DeleteMedication(
	ctx context.Context,
	userID, medID, familyID string,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return err
	}
	muid, err := pgutil.ParsePgUUID(medID)
	if err != nil {
		return err
	}

	if err := s.q.DeactivateMedication(
		ctx, dbgen.DeactivateMedicationParams{
			ID:     muid,
			UserID: uid,
		},
	); err != nil {
		return fmt.Errorf("deactivate medication: %w", err)
	}

	s.logFamilyAudit(
		ctx, userID, familyID,
		"family_delete_medication",
		map[string]any{"medication_id": medID},
	)

	return nil
}

// logFamilyAudit writes an audit log entry that includes the
// family member's ID in the params JSON.
func (s *Service) logFamilyAudit(
	ctx context.Context,
	userID, familyID, toolName string,
	params map[string]any,
) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		slog.Error("audit: parse user_id",
			"user_id", userID, "error", err,
		)
		return
	}

	merged := make(map[string]any, len(params)+2)
	for k, v := range params {
		merged[k] = v
	}
	merged["family_id"] = familyID
	merged["timestamp"] = time.Now().UTC().Format(
		time.RFC3339,
	)
	raw, err := json.Marshal(merged)
	if err != nil {
		slog.Error("audit: marshal params", "error", err)
		return
	}

	if err := s.q.CreateAuditLog(
		ctx, dbgen.CreateAuditLogParams{
			UserID:   uid,
			ToolName: toolName,
			Params:   raw,
			Result:   pgtype.Text{String: "ok", Valid: true},
		},
	); err != nil {
		slog.Error("audit: write log",
			"tool", toolName, "error", err,
		)
	}
}
