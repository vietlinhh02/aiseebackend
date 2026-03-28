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
	"github.com/team-dang/aiseebackend/internal/dbgen"
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
	Status      string   `json:"status"`
	RoomName    string   `json:"room_name"`
	Token       string   `json:"token"`
	ContactName string   `json:"contact_name"`
	Remaining   []string `json:"remaining"`
}

type Service struct {
	queries   *dbgen.Queries
	pusher    *push.Pusher
	lk        *lk.Client
	lastAlert sync.Map
}

func NewService(
	queries *dbgen.Queries,
	pusher *push.Pusher,
	lkClient *lk.Client,
) *Service {
	return &Service{
		queries: queries,
		pusher:  pusher,
		lk:      lkClient,
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
		s.pusher.Send(ctx, push.UserTokens{
			FcmToken:      m.FcmToken,
			ApnsToken:     m.ApnsToken,
			ApnsVoipToken: m.ApnsVoipToken,
		}, push.Notification{
			Title: "SOS - " + user.DisplayName,
			Body:  "Can giup do khan cap!",
			Type:  "voip",
			Data: map[string]string{
				"type":      "emergency",
				"room_name": roomName,
				"user_id":   userID,
				"lat":       fmt.Sprintf("%.6f", lat),
				"lng":       fmt.Sprintf("%.6f", lng),
			},
		})
		notified++
	}

	s.lastAlert.Store(userID, time.Now())

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

	s.pusher.Send(ctx, push.UserTokens{
		FcmToken:      target.FcmToken,
		ApnsToken:     target.ApnsToken,
		ApnsVoipToken: target.ApnsVoipToken,
	}, push.Notification{
		Title: "SOS - " + caller.DisplayName,
		Body:  "Can giup do khan cap!",
		Type:  "voip",
		Data: map[string]string{
			"type":      "escalation",
			"room_name": roomName,
			"user_id":   userID,
		},
	})

	answered, err := s.waitForAnswer(
		ctx, roomName, userID,
	)
	if err != nil {
		return EscalateResult{
			Status:      "no_answer",
			ContactName: target.DisplayName,
			Remaining:   remaining,
		}, err
	}

	if answered {
		return EscalateResult{
			Status:      "answered",
			RoomName:    roomName,
			Token:       token,
			ContactName: target.DisplayName,
		}, nil
	}

	return EscalateResult{
		Status:      "no_answer",
		ContactName: target.DisplayName,
		Remaining:   remaining,
	}, nil
}

func (s *Service) waitForAnswer(
	ctx context.Context,
	roomName, callerID string,
) (bool, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.After(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			s.lk.DeleteRoom(
				context.Background(), roomName,
			)
			return false, ctx.Err()
		case <-deadline:
			s.lk.DeleteRoom(
				context.Background(), roomName,
			)
			return false, nil
		case <-ticker.C:
			participants, err := s.lk.ListParticipants(
				ctx, roomName,
			)
			if err != nil {
				slog.Warn("list participants failed",
					"room", roomName, "error", err,
				)
				continue
			}
			for _, p := range participants {
				if p.Identity != callerID &&
					p.Identity != "" {
					return true, nil
				}
			}
		}
	}
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
