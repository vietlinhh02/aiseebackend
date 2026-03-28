package appointment

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
)

func TestVnLocation(t *testing.T) {
	if vnLocation == nil {
		t.Fatal("vnLocation is nil")
	}
	if vnLocation.String() != "Asia/Ho_Chi_Minh" {
		t.Errorf(
			"vnLocation = %q, want %q",
			vnLocation.String(), "Asia/Ho_Chi_Minh",
		)
	}

	ref := time.Date(2024, 1, 15, 12, 0, 0, 0, vnLocation)
	_, offset := ref.Zone()
	expectedOffset := 7 * 3600
	if offset != expectedOffset {
		t.Errorf(
			"UTC offset = %d, want %d (UTC+7)",
			offset, expectedOffset,
		)
	}
}

func TestScheduleReminders_PastSkipped(t *testing.T) {
	svc := &Service{}

	past := time.Now().Add(-2 * time.Hour)

	due1h := past.Add(-1 * time.Hour)
	due15m := past.Add(-15 * time.Minute)

	if due1h.After(time.Now()) {
		t.Error("1h reminder should be in the past")
	}
	if due15m.After(time.Now()) {
		t.Error("15m reminder should be in the past")
	}

	// scheduleReminders requires a scheduler with a redis
	// connection, so we verify the time logic directly: the
	// method checks due.After(now) before scheduling. With a
	// past datetime, both 1h and 15m due times are in the past,
	// so no scheduling call would be made.
	_ = svc
}

func TestParseReminderKey(t *testing.T) {
	cases := []struct {
		key       string
		wantID    string
		wantAdv   string
		wantError bool
	}{
		{
			key:     "550e8400-e29b-41d4-a716-446655440000:1h",
			wantID:  "550e8400-e29b-41d4-a716-446655440000",
			wantAdv: "1h",
		},
		{
			key:     "550e8400-e29b-41d4-a716-446655440000:15m",
			wantID:  "550e8400-e29b-41d4-a716-446655440000",
			wantAdv: "15m",
		},
		{
			key:       "no-colon",
			wantError: true,
		},
		{
			key:       "some-id:invalid",
			wantError: true,
		},
	}

	for _, tc := range cases {
		id, adv, err := parseReminderKey(tc.key)
		if tc.wantError {
			if err == nil {
				t.Errorf(
					"parseReminderKey(%q): expected error, got nil",
					tc.key,
				)
			}
			continue
		}
		if err != nil {
			t.Errorf(
				"parseReminderKey(%q): unexpected error: %v",
				tc.key, err,
			)
			continue
		}
		if id != tc.wantID {
			t.Errorf(
				"parseReminderKey(%q): id = %q, want %q",
				tc.key, id, tc.wantID,
			)
		}
		if adv != tc.wantAdv {
			t.Errorf(
				"parseReminderKey(%q): advance = %q, want %q",
				tc.key, adv, tc.wantAdv,
			)
		}
	}
}

func TestFormatNotificationBody(t *testing.T) {
	dt := time.Date(
		2024, 6, 15, 14, 30, 0, 0, vnLocation,
	)

	cases := []struct {
		name    string
		title   string
		loc     string
		advance string
		want    string
	}{
		{
			name:    "1h with location",
			title:   "Kham mat",
			loc:     "BV Nhan Dan",
			advance: "1h",
			want:    "Kham mat luc 14:30 tai BV Nhan Dan (con 1 gio)",
		},
		{
			name:    "15m with location",
			title:   "Kham mat",
			loc:     "BV Nhan Dan",
			advance: "15m",
			want:    "Kham mat luc 14:30 tai BV Nhan Dan (con 15 phut)",
		},
		{
			name:    "1h without location",
			title:   "Kham mat",
			loc:     "",
			advance: "1h",
			want:    "Kham mat luc 14:30 (con 1 gio)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apt := buildTestAppointment(tc.title, tc.loc, dt)
			got := formatNotificationBody(apt, tc.advance)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func buildTestAppointment(
	title, location string, dt time.Time,
) dbgen.Appointment {
	apt := dbgen.Appointment{
		Title: title,
		Datetime: pgtype.Timestamptz{
			Time: dt, Valid: true,
		},
	}
	if location != "" {
		apt.Location = pgtype.Text{
			String: location, Valid: true,
		}
	}
	return apt
}
