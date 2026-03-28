package appointment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
	"github.com/team-dang/aiseebackend/pkg/reminder"
)

const schedulerName = "appointment"

var vnLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic("load timezone Asia/Ho_Chi_Minh: " + err.Error())
	}
	return loc
}()

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
	userID, title, location string,
	datetime time.Time,
	notes string,
) (dbgen.Appointment, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Appointment{}, err
	}

	apt, err := s.queries.CreateAppointment(
		ctx, dbgen.CreateAppointmentParams{
			UserID:   uid,
			Title:    title,
			Location: pgtype.Text{String: location, Valid: location != ""},
			Datetime: pgtype.Timestamptz{Time: datetime, Valid: true},
			Notes:    pgtype.Text{String: notes, Valid: notes != ""},
		},
	)
	if err != nil {
		return dbgen.Appointment{}, fmt.Errorf(
			"create appointment: %w", err,
		)
	}

	aptID := pgutil.PgUUIDToString(apt.ID)
	s.scheduleReminders(ctx, aptID, datetime)
	return apt, nil
}

func (s *Service) List(
	ctx context.Context, userID string,
) ([]dbgen.Appointment, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetAppointmentsByUser(ctx, uid)
}

func (s *Service) Update(
	ctx context.Context,
	userID, appointmentID, title, location string,
	datetime time.Time,
	notes string,
) (dbgen.Appointment, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return dbgen.Appointment{}, err
	}
	aptUUID, err := pgutil.ParsePgUUID(appointmentID)
	if err != nil {
		return dbgen.Appointment{}, err
	}

	apt, err := s.queries.UpdateAppointment(
		ctx, dbgen.UpdateAppointmentParams{
			ID:       aptUUID,
			UserID:   uid,
			Title:    title,
			Location: pgtype.Text{String: location, Valid: location != ""},
			Datetime: pgtype.Timestamptz{Time: datetime, Valid: true},
			Notes:    pgtype.Text{String: notes, Valid: notes != ""},
		},
	)
	if err != nil {
		return dbgen.Appointment{}, fmt.Errorf(
			"update appointment: %w", err,
		)
	}

	s.unscheduleReminders(ctx, appointmentID)
	s.scheduleReminders(ctx, appointmentID, datetime)
	return apt, nil
}

func (s *Service) Delete(
	ctx context.Context, userID, appointmentID string,
) error {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return err
	}
	aptUUID, err := pgutil.ParsePgUUID(appointmentID)
	if err != nil {
		return err
	}

	if err := s.queries.DeleteAppointment(
		ctx, dbgen.DeleteAppointmentParams{
			ID:     aptUUID,
			UserID: uid,
		},
	); err != nil {
		return fmt.Errorf("delete appointment: %w", err)
	}

	s.unscheduleReminders(ctx, appointmentID)
	return nil
}

// LoadAll implements reminder.Handler. It loads upcoming
// appointments and generates 1h/15m reminders for each,
// skipping those already reminded or past due.
func (s *Service) LoadAll(
	ctx context.Context,
) ([]reminder.Reminder, error) {
	apts, err := s.queries.GetUpcomingAppointmentsAll(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"load upcoming appointments: %w", err,
		)
	}

	now := time.Now()
	var reminders []reminder.Reminder
	for _, apt := range apts {
		aptID := pgutil.PgUUIDToString(apt.ID)
		dt := apt.Datetime.Time

		if !apt.Reminded1h.Valid || !apt.Reminded1h.Bool {
			due1h := dt.Add(-1 * time.Hour)
			if due1h.After(now) {
				reminders = append(reminders, reminder.Reminder{
					Key:   aptID + ":1h",
					DueAt: due1h,
				})
			}
		}

		if !apt.Reminded15m.Valid || !apt.Reminded15m.Bool {
			due15m := dt.Add(-15 * time.Minute)
			if due15m.After(now) {
				reminders = append(reminders, reminder.Reminder{
					Key:   aptID + ":15m",
					DueAt: due15m,
				})
			}
		}
	}
	return reminders, nil
}

// OnDue implements reminder.Handler. Key format:
// "appointment_id:1h" or "appointment_id:15m".
func (s *Service) OnDue(
	ctx context.Context, key string,
) error {
	aptID, advance, err := parseReminderKey(key)
	if err != nil {
		return err
	}

	aptUUID, err := pgutil.ParsePgUUID(aptID)
	if err != nil {
		return fmt.Errorf("parse appointment id: %w", err)
	}

	apts, err := s.queries.GetUpcomingAppointmentsAll(ctx)
	if err != nil {
		return fmt.Errorf("get upcoming appointments: %w", err)
	}

	var apt dbgen.Appointment
	found := false
	for _, a := range apts {
		if a.ID == aptUUID {
			apt = a
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	user, err := s.queries.GetUserByID(ctx, apt.UserID)
	if err != nil {
		slog.Error("get user for appointment push",
			"user_id", pgutil.PgUUIDToString(apt.UserID),
			"error", err,
		)
		return nil
	}

	body := formatNotificationBody(apt, advance)
	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      user.FcmToken,
		ApnsToken:     user.ApnsToken,
		ApnsVoipToken: user.ApnsVoipToken,
	}, push.Notification{
		Title: "Lich hen sap toi",
		Body:  body,
		Type:  "normal",
		Data: map[string]string{
			"type":           "appointment_reminder",
			"appointment_id": aptID,
			"advance":        advance,
		},
	})

	return markReminded(ctx, s.queries, apt.ID, advance)
}

func (s *Service) scheduleReminders(
	ctx context.Context, aptID string, datetime time.Time,
) {
	now := time.Now()

	due1h := datetime.Add(-1 * time.Hour)
	if due1h.After(now) {
		if err := s.scheduler.Schedule(
			ctx, schedulerName,
			reminder.Reminder{Key: aptID + ":1h", DueAt: due1h},
		); err != nil {
			slog.Error("schedule 1h appointment reminder",
				"appointment_id", aptID, "error", err,
			)
		}
	}

	due15m := datetime.Add(-15 * time.Minute)
	if due15m.After(now) {
		if err := s.scheduler.Schedule(
			ctx, schedulerName,
			reminder.Reminder{Key: aptID + ":15m", DueAt: due15m},
		); err != nil {
			slog.Error("schedule 15m appointment reminder",
				"appointment_id", aptID, "error", err,
			)
		}
	}
}

func (s *Service) unscheduleReminders(
	ctx context.Context, aptID string,
) {
	for _, suffix := range []string{":1h", ":15m"} {
		if err := s.scheduler.Unschedule(
			ctx, schedulerName, aptID+suffix,
		); err != nil {
			slog.Error("unschedule appointment reminder",
				"key", aptID+suffix, "error", err,
			)
		}
	}
}

func parseReminderKey(key string) (string, string, error) {
	idx := strings.LastIndex(key, ":")
	if idx < 0 {
		return "", "", fmt.Errorf(
			"invalid reminder key format: %s", key,
		)
	}
	aptID := key[:idx]
	advance := key[idx+1:]
	if advance != "1h" && advance != "15m" {
		return "", "", fmt.Errorf(
			"invalid advance value in key %s: %s", key, advance,
		)
	}
	return aptID, advance, nil
}

func formatNotificationBody(
	apt dbgen.Appointment, advance string,
) string {
	dt := apt.Datetime.Time.In(vnLocation)
	timeStr := dt.Format("15:04")

	loc := ""
	if apt.Location.Valid && apt.Location.String != "" {
		loc = apt.Location.String
	}

	advanceVN := "1 gio"
	if advance == "15m" {
		advanceVN = "15 phut"
	}

	if loc != "" {
		return fmt.Sprintf(
			"%s luc %s tai %s (con %s)",
			apt.Title, timeStr, loc, advanceVN,
		)
	}
	return fmt.Sprintf(
		"%s luc %s (con %s)",
		apt.Title, timeStr, advanceVN,
	)
}

func markReminded(
	ctx context.Context,
	queries *dbgen.Queries,
	id pgtype.UUID,
	advance string,
) error {
	if advance == "1h" {
		return queries.MarkAppointmentReminded1h(ctx, id)
	}
	return queries.MarkAppointmentReminded15m(ctx, id)
}
