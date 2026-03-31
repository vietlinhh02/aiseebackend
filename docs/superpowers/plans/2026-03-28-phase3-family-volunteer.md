# Phase 3: Family Dashboard + Volunteer Matching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add family oversight dashboard, volunteer matching, and family medication management to the AiSee backend.

**Architecture:** Domain-separated packages (`internal/dashboard/`, `internal/volunteer/`) following existing codebase patterns. Dashboard is an aggregation layer calling existing services. Volunteer matching uses Redis for ephemeral match state and background goroutines for async candidate polling.

**Tech Stack:** Go, chi router, pgx/v5 + sqlc, Redis, LiveKit server-sdk-go

**Spec:** `docs/superpowers/specs/2026-03-28-aisee-phase3-design.md`

---

## Task 1: Database Migration

**Files:**
- Create: `db/migrations/003_phase3.up.sql`
- Create: `db/migrations/003_phase3.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- db/migrations/003_phase3.up.sql

-- Alter existing volunteers table (created in 001_init with different defaults)
ALTER TABLE volunteers
    ALTER COLUMN rating SET NOT NULL,
    ALTER COLUMN rating SET DEFAULT 0,
    ALTER COLUMN available SET NOT NULL,
    ALTER COLUMN total_calls SET NOT NULL,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Reset unrated volunteers from 5.0 default to 0
UPDATE volunteers SET rating = 0 WHERE total_calls = 0;

CREATE INDEX idx_volunteers_available ON volunteers (rating DESC, total_calls ASC)
    WHERE available = true;

-- Volunteer ratings table
CREATE TABLE volunteer_ratings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    volunteer_id UUID NOT NULL REFERENCES volunteers(user_id),
    rater_id     UUID NOT NULL REFERENCES users(id),
    match_id     TEXT NOT NULL,
    score        SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(match_id, rater_id)
);

CREATE INDEX idx_volunteer_ratings_volunteer ON volunteer_ratings (volunteer_id);

-- Location tracking on users
ALTER TABLE users
    ADD COLUMN last_lat         DOUBLE PRECISION,
    ADD COLUMN last_lng         DOUBLE PRECISION,
    ADD COLUMN last_location_at TIMESTAMPTZ;
```

- [ ] **Step 2: Write down migration**

```sql
-- db/migrations/003_phase3.down.sql

ALTER TABLE users
    DROP COLUMN IF EXISTS last_lat,
    DROP COLUMN IF EXISTS last_lng,
    DROP COLUMN IF EXISTS last_location_at;

DROP TABLE IF EXISTS volunteer_ratings;
DROP INDEX IF EXISTS idx_volunteers_available;

ALTER TABLE volunteers
    DROP COLUMN IF EXISTS created_at;

ALTER TABLE volunteers
    ALTER COLUMN rating DROP NOT NULL,
    ALTER COLUMN rating SET DEFAULT 5.0,
    ALTER COLUMN available DROP NOT NULL,
    ALTER COLUMN total_calls DROP NOT NULL;
```

- [ ] **Step 3: Run migration**

Run: `migrate -path db/migrations -database "$DATABASE_URL" up`
Expected: migration 003 applied successfully.

- [ ] **Step 4: Commit**

```bash
git add db/migrations/003_phase3.up.sql db/migrations/003_phase3.down.sql
git commit -m "feat: add Phase 3 migration — volunteer ratings, user location"
```

---

## Task 2: SQL Queries + sqlc Codegen

**Files:**
- Create: `db/queries/volunteers.sql`
- Create: `db/queries/dashboard.sql`
- Regenerate: `internal/dbgen/` (all files)

- [ ] **Step 1: Write volunteer queries**

Create `db/queries/volunteers.sql`:

```sql
-- name: CreateVolunteer :one
INSERT INTO volunteers (user_id, languages)
VALUES ($1, $2)
RETURNING *;

-- name: GetVolunteerByUserID :one
SELECT * FROM volunteers WHERE user_id = $1;

-- name: UpdateVolunteerAvailability :exec
UPDATE volunteers SET available = $1 WHERE user_id = $2;

-- name: GetAvailableVolunteers :many
SELECT v.*, u.display_name, u.fcm_token, u.apns_token, u.apns_voip_token
FROM volunteers v
JOIN users u ON u.id = v.user_id
WHERE v.available = true
  AND v.user_id != $1
  AND v.user_id NOT IN (
    SELECT family_id FROM family_links WHERE user_id = $1 AND deleted_at IS NULL
    UNION
    SELECT user_id FROM family_links WHERE family_id = $1 AND deleted_at IS NULL
  )
ORDER BY v.rating DESC, v.total_calls ASC
LIMIT 5;

-- name: CreateVolunteerRating :exec
INSERT INTO volunteer_ratings (volunteer_id, rater_id, match_id, score)
VALUES ($1, $2, $3, $4)
ON CONFLICT (match_id, rater_id) DO NOTHING;

-- name: IncrementVolunteerCalls :exec
UPDATE volunteers SET total_calls = total_calls + 1 WHERE user_id = $1;

-- name: RecalcVolunteerRating :exec
UPDATE volunteers SET
    rating = COALESCE((SELECT AVG(score)::REAL FROM volunteer_ratings WHERE volunteer_id = $1), 0)
WHERE user_id = $1;
```

- [ ] **Step 2: Write dashboard queries**

Create `db/queries/dashboard.sql`:

```sql
-- name: GetMedicationLogsToday :many
-- All medication logs for today across all of a user's medications (ICT timezone)
SELECT ml.* FROM medication_logs ml
JOIN medications m ON m.id = ml.medication_id
WHERE m.user_id = $1
  AND ml.taken_at >= (now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date::timestamptz
  AND ml.taken_at < ((now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date + INTERVAL '1 day')::timestamptz;

-- name: GetRecentSymptoms :many
SELECT * FROM health_symptoms
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2;

-- name: GetRecentMetrics :many
SELECT * FROM health_metrics
WHERE user_id = $1
ORDER BY logged_at DESC
LIMIT $2;

-- name: UpdateUserLocation :exec
UPDATE users SET last_lat = $2, last_lng = $3, last_location_at = now()
WHERE id = $1;

-- name: GetUserLocation :one
SELECT last_lat, last_lng, last_location_at FROM users WHERE id = $1;
```

- [ ] **Step 3: Run sqlc generate**

Run: `sqlc generate`
Expected: no errors, `internal/dbgen/` regenerated with new query methods.

- [ ] **Step 4: Verify generated code compiles**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add db/queries/volunteers.sql db/queries/dashboard.sql internal/dbgen/
git commit -m "feat: add SQL queries for volunteers and dashboard"
```

---

## Task 3: AuthorizeFamily Helper

**Files:**
- Modify: `internal/middleware/auth.go`
- Create: `internal/middleware/family_auth.go`
- Create: `internal/middleware/family_auth_test.go`

- [ ] **Step 1: Write failing test for AuthorizeFamily**

Create `internal/middleware/family_auth_test.go`:

```go
package middleware_test

import (
	"context"
	"testing"

	"github.com/team-dang/aiseebackend/internal/middleware"
)

// FamilyChecker stub for testing
type stubFamilyChecker struct {
	linked bool
	err    error
}

func (s *stubFamilyChecker) CheckFamilyLink(
	ctx context.Context, requesterID, targetID string,
) (bool, error) {
	return s.linked, s.err
}

func TestAuthorizeFamily_Success(t *testing.T) {
	checker := &stubFamilyChecker{linked: true}
	err := middleware.AuthorizeFamily(
		withRole(withUserID(context.Background(), "req-id"), "family"),
		"target-id",
		checker,
	)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestAuthorizeFamily_WrongRole(t *testing.T) {
	checker := &stubFamilyChecker{linked: true}
	err := middleware.AuthorizeFamily(
		withRole(withUserID(context.Background(), "req-id"), "user"),
		"target-id",
		checker,
	)
	if err == nil {
		t.Fatal("expected error for non-family role")
	}
}

func TestAuthorizeFamily_NoLink(t *testing.T) {
	checker := &stubFamilyChecker{linked: false}
	err := middleware.AuthorizeFamily(
		withRole(withUserID(context.Background(), "req-id"), "family"),
		"target-id",
		checker,
	)
	if err == nil {
		t.Fatal("expected error when no family link")
	}
}

// helper to inject values into context — uses exported SetContext helpers
func withUserID(ctx context.Context, id string) context.Context {
	return middleware.WithUserID(ctx, id)
}

func withRole(ctx context.Context, role string) context.Context {
	return middleware.WithRole(ctx, role)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/ -run TestAuthorizeFamily -v`
Expected: FAIL — `AuthorizeFamily`, `WithUserID`, `WithRole`, `FamilyChecker` not defined.

- [ ] **Step 3: Implement AuthorizeFamily**

Create `internal/middleware/family_auth.go`:

```go
package middleware

import (
	"context"
	"fmt"
)

// FamilyChecker abstracts the DB call for testing.
type FamilyChecker interface {
	CheckFamilyLink(ctx context.Context, requesterID, targetID string) (bool, error)
}

// WithUserID injects a user ID into the context (for testing).
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// WithRole injects a role into the context (for testing).
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

// AuthorizeFamily verifies that the requester has role "family"
// and a valid family_links relationship to the target user.
func AuthorizeFamily(
	ctx context.Context,
	targetUserID string,
	checker FamilyChecker,
) error {
	role := RoleFromContext(ctx)
	if role != "family" {
		return fmt.Errorf("forbidden: only family role can access dashboard")
	}

	requesterID := UserIDFromContext(ctx)
	if requesterID == "" {
		return fmt.Errorf("forbidden: missing user identity")
	}

	linked, err := checker.CheckFamilyLink(ctx, requesterID, targetUserID)
	if err != nil {
		return fmt.Errorf("check family link: %w", err)
	}
	if !linked {
		return fmt.Errorf("forbidden: no family link to this user")
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/middleware/ -run TestAuthorizeFamily -v`
Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/family_auth.go internal/middleware/family_auth_test.go
git commit -m "feat: add AuthorizeFamily helper with role + link check"
```

---

## Task 4: Dashboard Service + Handler

**Files:**
- Create: `internal/dashboard/service.go`
- Create: `internal/dashboard/handler.go`
- Create: `internal/dashboard/service_test.go`

- [ ] **Step 1: Write failing tests for dashboard service**

Create `internal/dashboard/service_test.go`. Test the `Health` aggregation method:

```go
package dashboard_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dashboard"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

type stubQueries struct {
	meds     []dbgen.Medication
	logs     []dbgen.MedicationLog
	symptoms []dbgen.HealthSymptom
	metrics  []dbgen.HealthMetric
	auditErr error
}

func (s *stubQueries) GetMedicationsByUser(
	ctx context.Context, uid pgtype.UUID,
) ([]dbgen.Medication, error) {
	return s.meds, nil
}

func (s *stubQueries) GetMedicationLogsToday(
	ctx context.Context, uid pgtype.UUID,
) ([]dbgen.MedicationLog, error) {
	return s.logs, nil
}

func (s *stubQueries) GetRecentSymptoms(
	ctx context.Context, p dbgen.GetRecentSymptomsParams,
) ([]dbgen.HealthSymptom, error) {
	return s.symptoms, nil
}

func (s *stubQueries) GetRecentMetrics(
	ctx context.Context, p dbgen.GetRecentMetricsParams,
) ([]dbgen.HealthMetric, error) {
	return s.metrics, nil
}

func TestHealth_ReturnsMedicationsAndSymptoms(t *testing.T) {
	q := &stubQueries{
		meds: []dbgen.Medication{
			{Name: "Aspirin", Dosage: "100mg"},
		},
		symptoms: []dbgen.HealthSymptom{
			{Symptom: "headache"},
		},
	}
	svc := dashboard.NewService(q)
	result, err := svc.Health(context.Background(), "user-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Medications) != 1 {
		t.Errorf("expected 1 medication, got %d", len(result.Medications))
	}
	if len(result.RecentSymptoms) != 1 {
		t.Errorf("expected 1 symptom, got %d", len(result.RecentSymptoms))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/ -run TestHealth -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement dashboard service**

Create `internal/dashboard/service.go`:

```go
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
)

// Querier abstracts the sqlc queries used by the dashboard.
type Querier interface {
	GetMedicationsByUser(ctx context.Context, uid pgtype.UUID) ([]dbgen.Medication, error)
	GetMedicationLogsToday(ctx context.Context, uid pgtype.UUID) ([]dbgen.MedicationLog, error)
	GetRecentSymptoms(ctx context.Context, p dbgen.GetRecentSymptomsParams) ([]dbgen.HealthSymptom, error)
	GetRecentMetrics(ctx context.Context, p dbgen.GetRecentMetricsParams) ([]dbgen.HealthMetric, error)
	GetAuditLogsByUser(ctx context.Context, p dbgen.GetAuditLogsByUserParams) ([]dbgen.AuditLog, error)
	GetUserLocation(ctx context.Context, uid pgtype.UUID) (dbgen.GetUserLocationRow, error)
	GetMedicationByID(ctx context.Context, p dbgen.GetMedicationByIDParams) (dbgen.Medication, error)
	CreateMedication(ctx context.Context, p dbgen.CreateMedicationParams) (dbgen.Medication, error)
	UpdateMedication(ctx context.Context, p dbgen.UpdateMedicationParams) (dbgen.Medication, error)
	DeactivateMedication(ctx context.Context, p dbgen.DeactivateMedicationParams) error
	CreateAuditLog(ctx context.Context, p dbgen.CreateAuditLogParams) error
}

type Service struct {
	q Querier
}

func NewService(q Querier) *Service {
	return &Service{q: q}
}

type MedWithAdherence struct {
	Medication dbgen.Medication `json:"medication"`
	TakenToday bool             `json:"taken_today"`
}

type HealthResponse struct {
	Medications    []MedWithAdherence   `json:"medications"`
	RecentSymptoms []dbgen.HealthSymptom `json:"recent_symptoms"`
	RecentMetrics  []dbgen.HealthMetric  `json:"recent_metrics"`
}

func (s *Service) Health(
	ctx context.Context, userID string,
) (HealthResponse, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return HealthResponse{}, err
	}

	meds, err := s.q.GetMedicationsByUser(ctx, uid)
	if err != nil {
		return HealthResponse{}, fmt.Errorf("get medications: %w", err)
	}

	logs, err := s.q.GetMedicationLogsToday(ctx, uid)
	if err != nil {
		return HealthResponse{}, fmt.Errorf("get med logs: %w", err)
	}

	// Build set of medication IDs that have a non-skipped log today
	takenSet := make(map[pgtype.UUID]bool)
	for _, log := range logs {
		if !log.Skipped.Valid || !log.Skipped.Bool {
			takenSet[log.MedicationID] = true
		}
	}

	medResults := make([]MedWithAdherence, 0, len(meds))
	for _, m := range meds {
		medResults = append(medResults, MedWithAdherence{
			Medication: m,
			TakenToday: takenSet[m.ID],
		})
	}

	symptoms, err := s.q.GetRecentSymptoms(ctx, dbgen.GetRecentSymptomsParams{
		UserID: uid,
		Limit:  10,
	})
	if err != nil {
		return HealthResponse{}, fmt.Errorf("get symptoms: %w", err)
	}

	metrics, err := s.q.GetRecentMetrics(ctx, dbgen.GetRecentMetricsParams{
		UserID: uid,
		Limit:  10,
	})
	if err != nil {
		return HealthResponse{}, fmt.Errorf("get metrics: %w", err)
	}

	return HealthResponse{
		Medications:    medResults,
		RecentSymptoms: symptoms,
		RecentMetrics:  metrics,
	}, nil
}

type MedicationsResponse struct {
	Medications []MedWithAdherence `json:"medications"`
	Count       int                `json:"count"`
}

func (s *Service) Medications(
	ctx context.Context, userID string,
) (MedicationsResponse, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return MedicationsResponse{}, err
	}

	meds, err := s.q.GetMedicationsByUser(ctx, uid)
	if err != nil {
		return MedicationsResponse{}, fmt.Errorf("get medications: %w", err)
	}

	logs, err := s.q.GetMedicationLogsToday(ctx, uid)
	if err != nil {
		return MedicationsResponse{}, fmt.Errorf("get med logs: %w", err)
	}

	takenSet := make(map[pgtype.UUID]bool)
	for _, log := range logs {
		if !log.Skipped.Valid || !log.Skipped.Bool {
			takenSet[log.MedicationID] = true
		}
	}

	results := make([]MedWithAdherence, 0, len(meds))
	for _, m := range meds {
		results = append(results, MedWithAdherence{
			Medication: m,
			TakenToday: takenSet[m.ID],
		})
	}

	return MedicationsResponse{
		Medications: results,
		Count:       len(results),
	}, nil
}

func (s *Service) AuditLog(
	ctx context.Context, userID string,
	page, limit int32,
) ([]dbgen.AuditLog, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * limit
	return s.q.GetAuditLogsByUser(ctx, dbgen.GetAuditLogsByUserParams{
		UserID: uid,
		Limit:  limit,
		Offset: offset,
	})
}

type LocationResponse struct {
	Lat       *float64           `json:"lat"`
	Lng       *float64           `json:"lng"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

func (s *Service) Location(
	ctx context.Context, userID string,
) (LocationResponse, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return LocationResponse{}, err
	}

	row, err := s.q.GetUserLocation(ctx, uid)
	if err != nil {
		return LocationResponse{}, fmt.Errorf("get location: %w", err)
	}

	return LocationResponse{
		Lat:       row.LastLat,
		Lng:       row.LastLng,
		UpdatedAt: row.LastLocationAt,
	}, nil
}

// Family medication management methods

type CreateMedRequest struct {
	Name      string   `json:"name"`
	Dosage    string   `json:"dosage"`
	Times     []string `json:"times"`
	Frequency string   `json:"frequency"`
}

func (s *Service) CreateMedication(
	ctx context.Context, userID, familyID string,
	req CreateMedRequest,
) (dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Medication{}, err
	}

	med, err := s.q.CreateMedication(ctx, dbgen.CreateMedicationParams{
		UserID:    uid,
		Name:      req.Name,
		Dosage:    req.Dosage,
		Times:     req.Times,
		Frequency: req.Frequency,
	})
	if err != nil {
		return dbgen.Medication{}, fmt.Errorf("create medication: %w", err)
	}

	s.logFamilyAudit(ctx, userID, familyID, "family_create_medication", req)
	return med, nil
}

type UpdateMedRequest struct {
	Name      string   `json:"name"`
	Dosage    string   `json:"dosage"`
	Times     []string `json:"times"`
	Frequency string   `json:"frequency"`
}

func (s *Service) UpdateMedication(
	ctx context.Context, userID, medID, familyID string,
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

	med, err := s.q.UpdateMedication(ctx, dbgen.UpdateMedicationParams{
		ID:        muid,
		UserID:    uid,
		Name:      req.Name,
		Dosage:    req.Dosage,
		Times:     req.Times,
		Frequency: req.Frequency,
	})
	if err != nil {
		return dbgen.Medication{}, fmt.Errorf("update medication: %w", err)
	}

	s.logFamilyAudit(ctx, userID, familyID, "family_update_medication", req)
	return med, nil
}

func (s *Service) DeleteMedication(
	ctx context.Context, userID, medID, familyID string,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return err
	}
	muid, err := pgutil.ParsePgUUID(medID)
	if err != nil {
		return err
	}

	if err := s.q.DeactivateMedication(ctx, dbgen.DeactivateMedicationParams{
		ID:     muid,
		UserID: uid,
	}); err != nil {
		return fmt.Errorf("deactivate medication: %w", err)
	}

	s.logFamilyAudit(ctx, userID, familyID, "family_delete_medication",
		map[string]string{"medication_id": medID})
	return nil
}

func (s *Service) logFamilyAudit(
	ctx context.Context,
	userID, familyID, toolName string,
	params any,
) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return
	}

	paramsMap := map[string]any{
		"family_id": familyID,
		"params":    params,
	}
	paramsJSON, _ := json.Marshal(paramsMap)

	_ = s.q.CreateAuditLog(ctx, dbgen.CreateAuditLogParams{
		UserID:   uid,
		ToolName: toolName,
		Params:   paramsJSON,
	})
}
```

**Note:** `encoding/json` is imported at the top of the file (in the import block) for use by `logFamilyAudit`.

- [ ] **Step 4: Implement dashboard handler**

Create `internal/dashboard/handler.go`:

```go
package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	svc     *Service
	checker middleware.FamilyChecker
}

func NewHandler(svc *Service, checker middleware.FamilyChecker) *Handler {
	return &Handler{svc: svc, checker: checker}
}

func (h *Handler) authorize(r *http.Request) (string, error) {
	userID := chi.URLParam(r, "user_id")
	return userID, middleware.AuthorizeFamily(r.Context(), userID, h.checker)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	result, err := h.svc.Health(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "health_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) Medications(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	result, err := h.svc.Medications(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "medications_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) AuditLog(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	logs, err := h.svc.AuditLog(r.Context(), userID, int32(page), int32(limit))
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "audit_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"logs":  logs,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) Location(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	loc, err := h.svc.Location(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "location_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, loc)
}

func (h *Handler) CreateMedication(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	var req CreateMedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.Name == "" || req.Dosage == "" {
		response.Err(w, http.StatusBadRequest, "missing_fields", "name and dosage are required")
		return
	}
	if req.Frequency == "" {
		req.Frequency = "daily"
	}

	familyID := middleware.UserIDFromContext(r.Context())
	med, err := h.svc.CreateMedication(r.Context(), userID, familyID, req)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "create_error", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, med)
}

func (h *Handler) UpdateMedication(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	medID := chi.URLParam(r, "med_id")
	var req UpdateMedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	familyID := middleware.UserIDFromContext(r.Context())
	med, err := h.svc.UpdateMedication(r.Context(), userID, medID, familyID, req)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, med)
}

func (h *Handler) DeleteMedication(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authorize(r)
	if err != nil {
		response.Err(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	medID := chi.URLParam(r, "med_id")
	familyID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.DeleteMedication(r.Context(), userID, medID, familyID); err != nil {
		response.Err(w, http.StatusInternalServerError, "delete_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/dashboard/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/dashboard/
git commit -m "feat: add dashboard service and handler for family oversight"
```

---

## Task 5: Volunteer Service

**Files:**
- Create: `internal/volunteer/service.go`
- Create: `internal/volunteer/service_test.go`

- [ ] **Step 1: Write failing test for volunteer registration**

Create `internal/volunteer/service_test.go`:

```go
package volunteer_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/internal/volunteer"
)

// stubQuerier implements volunteer.Querier with minimal stubs.
// Only the methods under test are filled in; others can panic.
type stubQuerier struct {
	registered bool
	available  bool
}

func (s *stubQuerier) CreateVolunteer(
	ctx context.Context, p dbgen.CreateVolunteerParams,
) (dbgen.Volunteer, error) {
	s.registered = true
	return dbgen.Volunteer{UserID: p.UserID}, nil
}

func (s *stubQuerier) UpdateVolunteerAvailability(
	ctx context.Context, p dbgen.UpdateVolunteerAvailabilityParams,
) error {
	s.available = p.Available
	return nil
}

func (s *stubQuerier) GetVolunteerByUserID(ctx context.Context, uid pgtype.UUID) (dbgen.Volunteer, error) {
	return dbgen.Volunteer{}, nil
}
func (s *stubQuerier) GetAvailableVolunteers(ctx context.Context, uid pgtype.UUID) ([]dbgen.GetAvailableVolunteersRow, error) {
	return nil, nil
}
func (s *stubQuerier) CreateVolunteerRating(ctx context.Context, p dbgen.CreateVolunteerRatingParams) error {
	return nil
}
func (s *stubQuerier) IncrementVolunteerCalls(ctx context.Context, uid pgtype.UUID) error {
	return nil
}
func (s *stubQuerier) RecalcVolunteerRating(ctx context.Context, uid pgtype.UUID) error {
	return nil
}
func (s *stubQuerier) GetUserByID(ctx context.Context, uid pgtype.UUID) (dbgen.User, error) {
	return dbgen.User{}, nil
}

func TestRegister(t *testing.T) {
	store := &stubQuerier{}
	svc := volunteer.NewService(store, nil, nil, nil)
	// Use a valid UUID format for pgutil.ParsePgUUID
	err := svc.Register(context.Background(), "00000000-0000-0000-0000-000000000001", []string{"vi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.registered {
		t.Fatal("expected volunteer to be registered")
	}
}

func TestSetAvailability(t *testing.T) {
	store := &stubQuerier{}
	svc := volunteer.NewService(store, nil, nil, nil)
	err := svc.SetAvailability(context.Background(), "00000000-0000-0000-0000-000000000001", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.available {
		t.Fatal("expected available to be true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/volunteer/ -run TestRegister -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement volunteer service**

Create `internal/volunteer/service.go`:

```go
package volunteer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
)

// Querier abstracts the sqlc queries used by volunteer service.
type Querier interface {
	CreateVolunteer(ctx context.Context, p dbgen.CreateVolunteerParams) (dbgen.Volunteer, error)
	GetVolunteerByUserID(ctx context.Context, uid pgtype.UUID) (dbgen.Volunteer, error)
	UpdateVolunteerAvailability(ctx context.Context, p dbgen.UpdateVolunteerAvailabilityParams) error
	GetAvailableVolunteers(ctx context.Context, uid pgtype.UUID) ([]dbgen.GetAvailableVolunteersRow, error)
	CreateVolunteerRating(ctx context.Context, p dbgen.CreateVolunteerRatingParams) error
	IncrementVolunteerCalls(ctx context.Context, uid pgtype.UUID) error
	RecalcVolunteerRating(ctx context.Context, uid pgtype.UUID) error
	GetUserByID(ctx context.Context, uid pgtype.UUID) (dbgen.User, error)
}

type Service struct {
	q      Querier
	rdb    *redis.Client
	lk     *lk.Client
	pusher *push.Pusher
}

func NewService(
	q Querier,
	rdb *redis.Client,
	lkClient *lk.Client,
	pusher *push.Pusher,
) *Service {
	return &Service{q: q, rdb: rdb, lk: lkClient, pusher: pusher}
}

func (s *Service) Register(
	ctx context.Context, userID string, languages []string,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return err
	}
	if len(languages) == 0 {
		languages = []string{"vi"}
	}
	_, err = s.q.CreateVolunteer(ctx, dbgen.CreateVolunteerParams{
		UserID:    uid,
		Languages: languages,
	})
	if err != nil {
		return fmt.Errorf("create volunteer: %w", err)
	}
	return nil
}

func (s *Service) SetAvailability(
	ctx context.Context, userID string, available bool,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return err
	}
	return s.q.UpdateVolunteerAvailability(ctx,
		dbgen.UpdateVolunteerAvailabilityParams{
			Available: available,
			UserID:    uid,
		},
	)
}

type MatchResult struct {
	MatchID string `json:"match_id"`
	Status  string `json:"status"`
}

const matchKeyPrefix = "match:"
const matchTTL = 5 * time.Minute

// StartMatch begins async volunteer matching.
// Returns immediately with a match_id for polling.
func (s *Service) StartMatch(
	ctx context.Context, userID string,
) (MatchResult, error) {
	matchID := uuid.New().String()
	key := matchKeyPrefix + matchID

	err := s.rdb.HSet(ctx, key, "status", "searching").Err()
	if err != nil {
		return MatchResult{}, fmt.Errorf("redis hset: %w", err)
	}
	s.rdb.Expire(ctx, key, matchTTL)

	go s.matchLoop(userID, matchID)

	return MatchResult{MatchID: matchID, Status: "searching"}, nil
}

func (s *Service) matchLoop(userID, matchID string) {
	ctx := context.Background()
	key := matchKeyPrefix + matchID

	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	user, err := s.q.GetUserByID(ctx, uid)
	if err != nil {
		slog.Error("match: get user", "error", err)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	candidates, err := s.q.GetAvailableVolunteers(ctx, uid)
	if err != nil {
		slog.Error("match: get volunteers", "error", err)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	if len(candidates) == 0 {
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	roomName := "vol-" + matchID[:8]
	if err := s.lk.CreateRoomWithTimeout(ctx, roomName, 300); err != nil {
		slog.Error("match: create room", "error", err)
		s.rdb.HSet(ctx, key, "status", "failed")
		return
	}

	for _, c := range candidates {
		volID := pgutil.PgUUIDToString(c.UserID)

		s.pusher.Send(ctx, push.UserTokens{
			FcmToken:      c.FcmToken,
			ApnsToken:     c.ApnsToken,
			ApnsVoipToken: c.ApnsVoipToken,
		}, push.Notification{
			Title: "Yêu cầu hỗ trợ - " + user.DisplayName,
			Body:  "Một người dùng cần sự giúp đỡ của bạn",
			Type:  "voip",
			Data: map[string]string{
				"type":      "volunteer_match",
				"room_name": roomName,
				"user_id":   userID,
				"match_id":  matchID,
			},
		})

		if joined := s.waitForJoin(ctx, roomName, userID, 30*time.Second); joined {
			token, tokenErr := s.lk.CreateToken(roomName, userID, user.DisplayName)
			if tokenErr != nil {
				slog.Error("match: create token", "error", tokenErr)
				continue
			}

			s.rdb.HSet(ctx, key,
				"status", "matched",
				"volunteer_id", volID,
				"volunteer_name", c.DisplayName,
				"room_name", roomName,
				"user_token", token,
			)

			s.q.IncrementVolunteerCalls(ctx, c.UserID)
			return
		}
	}

	s.lk.DeleteRoom(ctx, roomName)
	s.rdb.HSet(ctx, key, "status", "failed")
}

func (s *Service) waitForJoin(
	ctx context.Context, roomName, callerID string,
	timeout time.Duration,
) bool {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-deadline:
			return false
		case <-ticker.C:
			participants, err := s.lk.ListParticipants(ctx, roomName)
			if err != nil {
				continue
			}
			for _, p := range participants {
				if p.Identity != callerID && p.Identity != "" {
					return true
				}
			}
		}
	}
}

type MatchStatus struct {
	Status        string `json:"status"`
	VolunteerID   string `json:"volunteer_id,omitempty"`
	VolunteerName string `json:"volunteer_name,omitempty"`
	RoomName      string `json:"room_name,omitempty"`
	UserToken     string `json:"user_token,omitempty"`
}

// GetMatchStatus polls Redis for match state.
func (s *Service) GetMatchStatus(
	ctx context.Context, matchID string,
) (MatchStatus, error) {
	key := matchKeyPrefix + matchID
	vals, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return MatchStatus{}, fmt.Errorf("redis hgetall: %w", err)
	}
	if len(vals) == 0 {
		return MatchStatus{Status: "not_found"}, nil
	}

	return MatchStatus{
		Status:        vals["status"],
		VolunteerID:   vals["volunteer_id"],
		VolunteerName: vals["volunteer_name"],
		RoomName:      vals["room_name"],
		UserToken:     vals["user_token"],
	}, nil
}

// Rate records a volunteer rating and recalculates average.
func (s *Service) Rate(
	ctx context.Context,
	volunteerID, raterID, matchID string,
	score int16,
) error {
	vid, err := pgutil.ParsePgUUID(volunteerID)
	if err != nil {
		return err
	}
	rid, err := pgutil.ParsePgUUID(raterID)
	if err != nil {
		return err
	}

	if err := s.q.CreateVolunteerRating(ctx, dbgen.CreateVolunteerRatingParams{
		VolunteerID: vid,
		RaterID:     rid,
		MatchID:     matchID,
		Score:       score,
	}); err != nil {
		return fmt.Errorf("create rating: %w", err)
	}

	return s.q.RecalcVolunteerRating(ctx, vid)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/volunteer/ -v`
Expected: PASS (registration + availability tests).

- [ ] **Step 5: Commit**

```bash
git add internal/volunteer/service.go internal/volunteer/service_test.go
git commit -m "feat: add volunteer service — register, availability, matching, rating"
```

---

## Task 6: Volunteer Handler

**Files:**
- Create: `internal/volunteer/handler.go`

- [ ] **Step 1: Implement volunteer handler**

Create `internal/volunteer/handler.go`:

```go
package volunteer

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var body struct {
		Languages []string `json:"languages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.Languages = []string{"vi"}
	}

	if err := h.svc.Register(r.Context(), userID, body.Languages); err != nil {
		response.Err(w, http.StatusInternalServerError, "register_error", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

func (h *Handler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var body struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if err := h.svc.SetAvailability(r.Context(), userID, body.Available); err != nil {
		response.Err(w, http.StatusInternalServerError, "availability_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"status":    "updated",
		"available": body.Available,
	})
}

func (h *Handler) StartMatch(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	result, err := h.svc.StartMatch(r.Context(), userID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "match_error", err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, result)
}

func (h *Handler) MatchStatus(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "match_id")

	status, err := h.svc.GetMatchStatus(r.Context(), matchID)
	if err != nil {
		response.Err(w, http.StatusInternalServerError, "status_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, status)
}

func (h *Handler) Rate(w http.ResponseWriter, r *http.Request) {
	raterID := middleware.UserIDFromContext(r.Context())

	var body struct {
		VolunteerID string `json:"volunteer_id"`
		MatchID     string `json:"match_id"`
		Score       int16  `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if body.Score < 1 || body.Score > 5 {
		response.Err(w, http.StatusBadRequest, "invalid_score", "Score must be 1-5")
		return
	}

	if err := h.svc.Rate(r.Context(), body.VolunteerID, raterID, body.MatchID, body.Score); err != nil {
		response.Err(w, http.StatusInternalServerError, "rate_error", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "rated"})
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/volunteer/handler.go
git commit -m "feat: add volunteer REST handler"
```

---

## Task 7: Location Update in Emergency Service

**Files:**
- Modify: `internal/emergency/service.go`

- [ ] **Step 1: Write failing test for location update**

Add to `internal/emergency/service_test.go` (create if needed):

```go
// Test that Alert calls UpdateUserLocation
func TestAlert_UpdatesLocation(t *testing.T) {
	// Setup test with mock queries that records if UpdateUserLocation was called
	// ...
}
```

The key assertion: after `Alert()` is called with lat/lng, `UpdateUserLocation` must be called with the same coordinates.

- [ ] **Step 2: Modify emergency service to update location**

In `internal/emergency/service.go`, add `UpdateUserLocation` call after the push notification loop at line 130 (after `s.lastAlert.Store`):

```go
// Update last known location
s.queries.UpdateUserLocation(ctx, dbgen.UpdateUserLocationParams{
    ID:      uid,
    LastLat: &lat,
    LastLng: &lng,
})
```

**Note:** The `UpdateUserLocation` params will be generated by sqlc. The implementer needs to check the exact generated struct fields after running `sqlc generate` in Task 2. The `last_lat` and `last_lng` columns are `DOUBLE PRECISION` (nullable), so the generated fields will be `*float64`.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/emergency/ -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/emergency/service.go internal/emergency/service_test.go
git commit -m "feat: update user location on emergency alert"
```

---

## Task 8: Tool Handlers — escalate_to_volunteer, rate_volunteer, update_location

**Files:**
- Modify: `internal/toolcall/handler.go`

- [ ] **Step 1: Add volunteer service to Handler struct**

In `internal/toolcall/handler.go`, add `volunteer` field to `Handler` struct and update `NewHandler`:

```go
type Handler struct {
	// ... existing fields ...
	volunteer *volunteer.Service
}

func NewHandler(
	// ... existing params ...
	volSvc *volunteer.Service,
) *Handler {
	return &Handler{
		// ... existing fields ...
		volunteer: volSvc,
	}
}
```

- [ ] **Step 2: Add new tool cases to dispatch switch**

Add to the `dispatch` switch in `handler.go`:

```go
case "escalate_to_volunteer":
    return h.handleEscalateToVolunteer(ctx, req)
case "rate_volunteer":
    return h.handleRateVolunteer(ctx, req)
case "update_location":
    return h.handleUpdateLocation(ctx, req)
```

- [ ] **Step 3: Implement tool handler methods**

```go
func (h *Handler) handleEscalateToVolunteer(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	result, err := h.volunteer.StartMatch(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("start volunteer match: %w", err)
	}
	return map[string]any{
		"status":   "searching",
		"match_id": result.MatchID,
		"poll_url": "/api/v1/volunteers/match/" + result.MatchID + "/status",
	}, nil
}

func (h *Handler) handleRateVolunteer(
	ctx context.Context, req ToolCallRequest,
) (any, error) {
	matchID, _ := req.Params["match_id"].(string)
	score, _ := req.Params["score"].(float64)
	volunteerID, _ := req.Params["volunteer_id"].(string)

	if matchID == "" || volunteerID == "" || score < 1 || score > 5 {
		return nil, fmt.Errorf("match_id, volunteer_id, and score (1-5) are required")
	}

	if err := h.volunteer.Rate(
		ctx, volunteerID, req.UserID, matchID, int16(score),
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

	if err := h.queries.UpdateUserLocation(ctx, dbgen.UpdateUserLocationParams{
		ID:      uid,
		LastLat: &lat,
		LastLng: &lng,
	}); err != nil {
		return nil, fmt.Errorf("update location: %w", err)
	}

	return map[string]any{
		"status": "location_updated",
		"lat":    lat,
		"lng":    lng,
	}, nil
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 5: Run existing tool handler tests**

Run: `go test ./internal/toolcall/ -v`
Expected: existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/toolcall/handler.go
git commit -m "feat: add escalate_to_volunteer, rate_volunteer, update_location tools"
```

---

## Task 9: Route Registration

**Files:**
- Modify: `cmd/server/main.go`

**Note:** `user.Service` already has a `CheckFamilyLink(ctx, requesterID, targetID string) (bool, error)` method at `internal/user/service.go:184`. It already satisfies the `middleware.FamilyChecker` interface — no new file needed.

- [ ] **Step 1: Register new routes in main.go**

Add imports for `dashboard` and `volunteer` packages, create services and handlers, and register routes:

```go
// After existing service initialization (around line 120):

// Dashboard
dashService := dashboard.NewService(queries)
dashHandler := dashboard.NewHandler(dashService, userService)

// Volunteer
volService := volunteer.NewService(queries, rdb, lkClient, pusher)
volHandler := volunteer.NewHandler(volService)

// Update toolHandler to include volunteer service
toolHandler := toolcall.NewHandler(
    chatService, callService, userService,
    medService, healthService, aptService,
    emergService, musicService,
    queries,
    volService,  // NEW
)
```

Add route groups inside the `r.Route("/api/v1", ...)` group, within the authenticated section:

```go
// Dashboard (family)
r.Route("/dashboard/{user_id}", func(r chi.Router) {
    r.Get("/health", dashHandler.Health)
    r.Get("/medications", dashHandler.Medications)
    r.Get("/audit-log", dashHandler.AuditLog)
    r.Get("/location", dashHandler.Location)
    r.Post("/medications", dashHandler.CreateMedication)
    r.Put("/medications/{med_id}", dashHandler.UpdateMedication)
    r.Delete("/medications/{med_id}", dashHandler.DeleteMedication)
})

// Volunteers
r.Route("/volunteers", func(r chi.Router) {
    r.Post("/register", volHandler.Register)
    r.Put("/availability", volHandler.SetAvailability)
    r.Post("/match", volHandler.StartMatch)
    r.Get("/match/{match_id}/status", volHandler.MatchStatus)
    r.Post("/rate", volHandler.Rate)
})
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire Phase 3 routes — dashboard + volunteer endpoints"
```

---

## Task 10: Final Integration Test + Cleanup

**Files:**
- All Phase 3 files

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: all tests pass, no regressions.

- [ ] **Step 2: Run linters**

Run: `ruff check` (N/A — Go project), `go vet ./...`
Expected: no warnings.

- [ ] **Step 3: Verify build is clean**

Run: `go build ./...`
Expected: build succeeds with no warnings.

- [ ] **Step 4: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: Phase 3 cleanup and test fixes"
```
