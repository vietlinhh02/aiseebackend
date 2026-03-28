package medication

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
	"github.com/team-dang/aiseebackend/pkg/reminder"
	"golang.org/x/text/unicode/norm"
)

const schedulerName = "medication"

var hcmTZ = mustLoadTZ("Asia/Ho_Chi_Minh")

func mustLoadTZ(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("load timezone " + name + ": " + err.Error())
	}
	return loc
}

// MedicationWithStatus pairs a medication with whether
// it has been taken today.
type MedicationWithStatus struct {
	Medication dbgen.Medication `json:"medication"`
	TakenToday bool             `json:"taken_today"`
}

type Service struct {
	queries   *dbgen.Queries
	scheduler *reminder.Scheduler
	pusher    *push.Pusher
}

func NewService(
	queries *dbgen.Queries,
	scheduler *reminder.Scheduler,
	pusher *push.Pusher,
) *Service {
	return &Service{
		queries:   queries,
		scheduler: scheduler,
		pusher:    pusher,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID, name, dosage, frequency string,
	times []string,
) (dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Medication{}, err
	}

	med, err := s.queries.CreateMedication(
		ctx, dbgen.CreateMedicationParams{
			UserID:    uid,
			Name:      name,
			Dosage:    dosage,
			Times:     times,
			Frequency: frequency,
		},
	)
	if err != nil {
		return dbgen.Medication{}, fmt.Errorf(
			"create medication: %w", err,
		)
	}

	medID := pgutil.PgUUIDToString(med.ID)
	for i, t := range times {
		due, parseErr := nextOccurrence(t)
		if parseErr != nil {
			slog.Warn("skip scheduling: bad time format",
				"medication_id", medID,
				"time", t,
				"error", parseErr,
			)
			continue
		}
		key := medID + ":" + strconv.Itoa(i)
		if schedErr := s.scheduler.Schedule(
			ctx, schedulerName,
			reminder.Reminder{Key: key, DueAt: due},
		); schedErr != nil {
			slog.Error("schedule medication reminder",
				"key", key, "error", schedErr,
			)
		}
	}

	return med, nil
}

func (s *Service) List(
	ctx context.Context, userID string,
) ([]dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetMedicationsByUser(ctx, uid)
}

func (s *Service) ListWithStatus(
	ctx context.Context, userID string,
) ([]MedicationWithStatus, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}

	meds, err := s.queries.GetMedicationsByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get medications: %w", err)
	}

	result := make([]MedicationWithStatus, 0, len(meds))
	for _, med := range meds {
		logs, logErr := s.queries.GetMedicationLogToday(
			ctx, med.ID,
		)
		if logErr != nil {
			return nil, fmt.Errorf(
				"get logs for %s: %w",
				pgutil.PgUUIDToString(med.ID), logErr,
			)
		}
		taken := false
		for _, log := range logs {
			if !log.Skipped.Valid || !log.Skipped.Bool {
				taken = true
				break
			}
		}
		result = append(result, MedicationWithStatus{
			Medication: med,
			TakenToday: taken,
		})
	}
	return result, nil
}

// FindByName resolves a medication by name for a user.
// Tries exact case-insensitive SQL match first, then falls
// back to in-memory Vietnamese-normalized fuzzy matching.
func (s *Service) FindByName(
	ctx context.Context, userID, name string,
) (dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Medication{}, err
	}

	exact, err := s.queries.GetMedicationByName(
		ctx, dbgen.GetMedicationByNameParams{
			UserID: uid,
			Lower:  name,
		},
	)
	if err != nil {
		return dbgen.Medication{}, fmt.Errorf(
			"exact name lookup: %w", err,
		)
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return dbgen.Medication{}, fmt.Errorf(
			"multiple medications match '%s', "+
				"please be more specific", name,
		)
	}

	all, err := s.queries.GetMedicationsByUser(ctx, uid)
	if err != nil {
		return dbgen.Medication{}, fmt.Errorf(
			"list medications for fuzzy match: %w", err,
		)
	}

	normalized := normalizeVN(name)
	var matches []dbgen.Medication
	for _, med := range all {
		if strings.Contains(
			normalizeVN(med.Name), normalized,
		) {
			matches = append(matches, med)
		}
	}

	switch len(matches) {
	case 0:
		return dbgen.Medication{}, fmt.Errorf(
			"medication not found: %s", name,
		)
	case 1:
		return matches[0], nil
	default:
		return dbgen.Medication{}, fmt.Errorf(
			"multiple medications match '%s', "+
				"please be more specific", name,
		)
	}
}

func (s *Service) MarkTaken(
	ctx context.Context, userID, medicationID string,
) (dbgen.MedicationLog, error) {
	_, err := s.getMedication(ctx, userID, medicationID)
	if err != nil {
		return dbgen.MedicationLog{}, err
	}

	medUUID, err := pgutil.ParsePgUUID(medicationID)
	if err != nil {
		return dbgen.MedicationLog{}, err
	}

	return s.queries.CreateMedicationLog(
		ctx, dbgen.CreateMedicationLogParams{
			MedicationID: medUUID,
			Skipped:      pgtype.Bool{Bool: false, Valid: true},
		},
	)
}

func (s *Service) MarkSkipped(
	ctx context.Context, userID, medicationID string,
) (dbgen.MedicationLog, error) {
	_, err := s.getMedication(ctx, userID, medicationID)
	if err != nil {
		return dbgen.MedicationLog{}, err
	}

	medUUID, err := pgutil.ParsePgUUID(medicationID)
	if err != nil {
		return dbgen.MedicationLog{}, err
	}

	return s.queries.CreateMedicationLog(
		ctx, dbgen.CreateMedicationLogParams{
			MedicationID: medUUID,
			Skipped:      pgtype.Bool{Bool: true, Valid: true},
		},
	)
}

func (s *Service) Deactivate(
	ctx context.Context, userID, medicationID string,
) error {
	med, err := s.getMedication(ctx, userID, medicationID)
	if err != nil {
		return err
	}

	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return err
	}
	medUUID, err := pgutil.ParsePgUUID(medicationID)
	if err != nil {
		return err
	}

	if err := s.queries.DeactivateMedication(
		ctx, dbgen.DeactivateMedicationParams{
			ID:     medUUID,
			UserID: uid,
		},
	); err != nil {
		return fmt.Errorf("deactivate medication: %w", err)
	}

	medID := pgutil.PgUUIDToString(med.ID)
	for i := range med.Times {
		key := medID + ":" + strconv.Itoa(i)
		if unErr := s.scheduler.Unschedule(
			ctx, schedulerName, key,
		); unErr != nil {
			slog.Error("unschedule medication reminder",
				"key", key, "error", unErr,
			)
		}
	}
	return nil
}

// LoadAll implements reminder.Handler. It loads all active
// medications and returns a reminder for each scheduled time.
func (s *Service) LoadAll(
	ctx context.Context,
) ([]reminder.Reminder, error) {
	meds, err := s.queries.GetActiveMedicationsAll(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"load active medications: %w", err,
		)
	}

	var reminders []reminder.Reminder
	for _, med := range meds {
		medID := pgutil.PgUUIDToString(med.ID)
		for i, t := range med.Times {
			due, parseErr := nextOccurrence(t)
			if parseErr != nil {
				slog.Warn("skip reminder: bad time format",
					"medication_id", medID,
					"time", t,
					"error", parseErr,
				)
				continue
			}
			reminders = append(reminders, reminder.Reminder{
				Key:   medID + ":" + strconv.Itoa(i),
				DueAt: due,
			})
		}
	}
	return reminders, nil
}

// OnDue implements reminder.Handler. Key format:
// "medication_id:time_index".
func (s *Service) OnDue(
	ctx context.Context, key string,
) error {
	medID, timeIdx, err := parseReminderKey(key)
	if err != nil {
		return err
	}

	medUUID, err := pgutil.ParsePgUUID(medID)
	if err != nil {
		return fmt.Errorf("parse medication id: %w", err)
	}

	meds, err := s.queries.GetActiveMedicationsAll(ctx)
	if err != nil {
		return fmt.Errorf("get medication: %w", err)
	}

	var med dbgen.Medication
	found := false
	for _, m := range meds {
		if m.ID == medUUID {
			med = m
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	if timeIdx >= len(med.Times) {
		return nil
	}

	logs, err := s.queries.GetMedicationLogToday(
		ctx, med.ID,
	)
	if err != nil {
		return fmt.Errorf("check today's log: %w", err)
	}
	if len(logs) > 0 {
		return s.reschedule(ctx, key, med.Times[timeIdx])
	}

	user, err := s.queries.GetUserByID(ctx, med.UserID)
	if err != nil {
		slog.Error("get user for push",
			"user_id", pgutil.PgUUIDToString(med.UserID),
			"error", err,
		)
	} else {
		s.pusher.Send(ctx, push.UserTokens{
			FcmToken:      user.FcmToken,
			ApnsToken:     user.ApnsToken,
			ApnsVoipToken: user.ApnsVoipToken,
		}, push.Notification{
			Title: "Nhắc uống thuốc",
			Body: fmt.Sprintf(
				"Đã đến giờ uống %s (%s)",
				med.Name, med.Dosage,
			),
			Type: "normal",
			Data: map[string]string{
				"type":          "medication_reminder",
				"medication_id": medID,
			},
		})
	}

	return s.reschedule(ctx, key, med.Times[timeIdx])
}

func (s *Service) reschedule(
	ctx context.Context, key, timeStr string,
) error {
	now := time.Now().In(hcmTZ)
	tomorrow := now.AddDate(0, 0, 1)

	parts := strings.SplitN(timeStr, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid time format: %s", timeStr)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("parse hour: %w", err)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("parse minute: %w", err)
	}

	due := time.Date(
		tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
		hour, minute, 0, 0, hcmTZ,
	)
	return s.scheduler.Schedule(
		ctx, schedulerName,
		reminder.Reminder{Key: key, DueAt: due},
	)
}

func (s *Service) getMedication(
	ctx context.Context, userID, medicationID string,
) (dbgen.Medication, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Medication{}, err
	}
	medUUID, err := pgutil.ParsePgUUID(medicationID)
	if err != nil {
		return dbgen.Medication{}, err
	}
	return s.queries.GetMedicationByID(
		ctx, dbgen.GetMedicationByIDParams{
			ID:     medUUID,
			UserID: uid,
		},
	)
}

func parseReminderKey(key string) (string, int, error) {
	idx := strings.LastIndex(key, ":")
	if idx < 0 {
		return "", 0, fmt.Errorf(
			"invalid reminder key format: %s", key,
		)
	}
	medID := key[:idx]
	timeIdx, err := strconv.Atoi(key[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf(
			"parse time index from key %s: %w", key, err,
		)
	}
	return medID, timeIdx, nil
}

// nextOccurrence parses "HH:MM" in Asia/Ho_Chi_Minh and
// returns the next occurrence (today if in the future,
// tomorrow if already past).
func nextOccurrence(timeStr string) (time.Time, error) {
	parts := strings.SplitN(timeStr, ":", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf(
			"invalid time format %q: expected HH:MM", timeStr,
		)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"parse hour from %q: %w", timeStr, err,
		)
	}
	if hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf(
			"hour out of range in %q: %d", timeStr, hour,
		)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"parse minute from %q: %w", timeStr, err,
		)
	}
	if minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf(
			"minute out of range in %q: %d", timeStr, minute,
		)
	}

	now := time.Now().In(hcmTZ)
	candidate := time.Date(
		now.Year(), now.Month(), now.Day(),
		hour, minute, 0, 0, hcmTZ,
	)
	if candidate.Before(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate, nil
}

// normalizeVN lowercases and strips Vietnamese diacritics
// for fuzzy matching.
func normalizeVN(s string) string {
	s = strings.ToLower(s)
	t := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range t {
		if !unicode.Is(unicode.Mn, r) {
			if r == '\u0111' {
				b.WriteRune('d')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
