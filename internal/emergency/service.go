package emergency

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/pkg/i18n"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/pgutil"
	"github.com/team-dang/aiseebackend/pkg/push"
	"golang.org/x/text/unicode/norm"
)

type AlertResult struct {
	RoomName       string  `json:"room_name"`
	Token          string  `json:"token"`
	FamilyNotified int     `json:"family_notified"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
}

type EscalateResult struct {
	Status       string   `json:"status"`
	RoomName     string   `json:"room_name"`
	Token        string   `json:"token"`
	ContactID    string   `json:"contact_id"`
	ContactName  string   `json:"contact_name"`
	Relationship string   `json:"relationship"`
	AvatarURL    string   `json:"avatar_url"`
	Remaining    []string `json:"remaining"`
}

type Service struct {
	queries   *dbgen.Queries
	pusher    *push.Pusher
	lk        *lk.Client
	lastAlert sync.Map
	hook      SOSDomainHook
}

func NewService(
	queries *dbgen.Queries,
	pusher *push.Pusher,
	lkClient *lk.Client,
	hook SOSDomainHook,
) *Service {
	if hook == nil {
		hook = noopSOSHook{}
	}
	return &Service{
		queries: queries,
		pusher:  pusher,
		lk:      lkClient,
		hook:    hook,
	}
}

func (s *Service) Alert(
	ctx context.Context,
	userID string,
	lat, lng float64,
) (AlertResult, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return AlertResult{}, fmt.Errorf(
			"parse user id: %w", err,
		)
	}

	if last, ok := s.lastAlert.Load(userID); ok {
		if time.Since(last.(time.Time)) < 60*time.Second {
			return AlertResult{}, fmt.Errorf(
				"alert already sent recently",
			)
		}
	}

	user, err := s.queries.GetUserByID(ctx, uid)
	if err != nil {
		return AlertResult{}, fmt.Errorf(
			"get user: %w", err,
		)
	}

	members, err := s.queries.GetFamilyMembersOrdered(
		ctx, uid,
	)
	if err != nil {
		return AlertResult{}, fmt.Errorf(
			"get family members: %w", err,
		)
	}

	roomName := "sos-" + uuid.New().String()[:8]
	if err := s.lk.CreateRoomWithTimeout(
		ctx, roomName, 300,
	); err != nil {
		return AlertResult{}, fmt.Errorf(
			"create sos room: %w", err,
		)
	}

	token, err := s.lk.CreateToken(
		roomName, userID, user.DisplayName,
	)
	if err != nil {
		return AlertResult{}, fmt.Errorf(
			"create token: %w", err,
		)
	}

	notified := 0
	for _, m := range members {
		calleeToken, _ := s.lk.CreateToken(
			roomName, pgutil.PgUUIDToString(m.ID), m.DisplayName,
		)
		lang := i18n.FromUserSettings(m.Settings)
		s.pusher.Send(ctx, push.UserTokens{
			FcmToken:      m.FcmToken,
			ApnsToken:     m.ApnsToken,
			ApnsVoipToken: m.ApnsVoipToken,
		}, push.Notification{
			Title: i18n.T(lang, "notif.emergency.title", user.DisplayName),
			Body:  i18n.T(lang, "notif.emergency.body"),
			Type:  "voip",
			Data: map[string]string{
				"type":        "emergency",
				"room_name":   roomName,
				"user_id":     userID,
				"caller_name": user.DisplayName,
				"token":       calleeToken,
				"lat":         fmt.Sprintf("%.6f", lat),
				"lng":         fmt.Sprintf("%.6f", lng),
			},
		})
		notified++
	}

	s.lastAlert.Store(userID, time.Now())

	if err := s.hook.OnSOSAlert(ctx, userID, roomName, lat, lng, notified); err != nil {
		slog.Error("sos domain hook", "user_id", userID, "error", err)
	}

	// Update last known location
	if err := s.queries.UpdateUserLocation(
		ctx, dbgen.UpdateUserLocationParams{
			ID:      uid,
			LastLat: pgtype.Float8{Float64: lat, Valid: true},
			LastLng: pgtype.Float8{Float64: lng, Valid: true},
		},
	); err != nil {
		slog.Error(
			"update user location",
			"user_id", userID,
			"error", err,
		)
	}

	return AlertResult{
		RoomName:       roomName,
		Token:          token,
		FamilyNotified: notified,
		Lat:            lat,
		Lng:            lng,
	}, nil
}

func (s *Service) Escalate(
	ctx context.Context,
	userID, contactName string,
	skip []string,
) (EscalateResult, error) {
	uid, err := pgutil.ParsePgUUID(userID)
	if err != nil {
		return EscalateResult{}, fmt.Errorf(
			"parse user id: %w", err,
		)
	}

	caller, err := s.queries.GetUserByID(ctx, uid)
	if err != nil {
		return EscalateResult{}, fmt.Errorf(
			"get caller: %w", err,
		)
	}

	allMembers, err := s.queries.GetFamilyMembersOrdered(
		ctx, uid,
	)
	if err != nil {
		return EscalateResult{}, fmt.Errorf(
			"get family members: %w", err,
		)
	}

	filtered := FilterContacts(allMembers, contactName, skip)
	if len(filtered) == 0 {
		return EscalateResult{Status: "no_contacts"}, nil
	}

	target := filtered[0]
	remaining := make([]string, 0, len(filtered)-1)
	for _, m := range filtered[1:] {
		remaining = append(remaining, m.DisplayName)
	}

	roomName := "esc-" + uuid.New().String()[:8]
	if err := s.lk.CreateRoom(ctx, roomName); err != nil {
		return EscalateResult{}, fmt.Errorf(
			"create escalation room: %w", err,
		)
	}

	token, err := s.lk.CreateToken(
		roomName, userID, caller.DisplayName,
	)
	if err != nil {
		return EscalateResult{}, fmt.Errorf(
			"create token: %w", err,
		)
	}

	calleeToken, _ := s.lk.CreateToken(
		roomName, pgutil.PgUUIDToString(target.ID), target.DisplayName,
	)

	lang := i18n.FromUserSettings(target.Settings)
	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      target.FcmToken,
		ApnsToken:     target.ApnsToken,
		ApnsVoipToken: target.ApnsVoipToken,
	}, push.Notification{
		Title: i18n.T(lang, "notif.emergency.title", caller.DisplayName),
		Body:  i18n.T(lang, "notif.emergency.body"),
		Type:  "voip",
		Data: map[string]string{
			"type":        "escalation",
			"room_name":   roomName,
			"user_id":     userID,
			"caller_name": caller.DisplayName,
			"token":       calleeToken,
		},
	})

	if err := s.hook.OnSOSFamilyEscalation(
		ctx, userID, roomName, target.DisplayName, "notified", remaining,
	); err != nil {
		slog.Error("sos escalation domain hook", "user_id", userID, "error", err)
	}

	avatar := ""
	if target.AvatarUrl.Valid {
		avatar = target.AvatarUrl.String
	}
	return EscalateResult{
		Status:       "prepared",
		RoomName:     roomName,
		Token:        token,
		ContactID:    pgutil.PgUUIDToString(target.ID),
		ContactName:  target.DisplayName,
		Relationship: target.Relationship,
		AvatarURL:    avatar,
		Remaining:    remaining,
	}, nil
}

// FilterContacts filters family members by contact name
// and removes already-tried contacts listed in skip.
func FilterContacts(
	members []dbgen.GetFamilyMembersOrderedRow,
	contactName string,
	skip []string,
) []dbgen.GetFamilyMembersOrderedRow {
	result := members

	if contactName != "" {
		needle := normalizeVN(contactName)
		var matched []dbgen.GetFamilyMembersOrderedRow
		for _, m := range result {
			nameNorm := normalizeVN(m.DisplayName)
			relNorm := normalizeVN(m.Relationship)
			if strings.Contains(nameNorm, needle) ||
				strings.Contains(relNorm, needle) {
				matched = append(matched, m)
			}
		}
		result = matched
	}

	if len(skip) > 0 {
		skipSet := make(map[string]bool, len(skip))
		for _, s := range skip {
			skipSet[normalizeVN(s)] = true
		}
		var filtered []dbgen.GetFamilyMembersOrderedRow
		for _, m := range result {
			if !skipSet[normalizeVN(m.DisplayName)] {
				filtered = append(filtered, m)
			}
		}
		result = filtered
	}

	return result
}

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
