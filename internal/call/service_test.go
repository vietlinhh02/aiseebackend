package call

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
)

type fakeUsageService struct {
	requireQuotaCalls []billing.QuotaCheckInput
	requireQuotaErr   error
}

func (f *fakeUsageService) RequireQuota(
	_ context.Context,
	input billing.QuotaCheckInput,
) error {
	f.requireQuotaCalls = append(f.requireQuotaCalls, input)
	return f.requireQuotaErr
}

func (f *fakeUsageService) RecordUsage(
	context.Context,
	billing.UsageRecordInput,
) (billing.UsageRecord, error) {
	return billing.UsageRecord{}, nil
}

func (f *fakeUsageService) StartUsageSession(
	context.Context,
	billing.UsageSessionInput,
) (billing.UsageSession, error) {
	return billing.UsageSession{}, nil
}

func (f *fakeUsageService) CommitUsageSession(
	context.Context,
	billing.CommitUsageSessionInput,
) (billing.UsageRecord, error) {
	return billing.UsageRecord{}, nil
}

func TestGetUserTokenRequiresAIMinutesQuota(t *testing.T) {
	billingSvc := &fakeUsageService{
		requireQuotaErr: &billing.QuotaExceededError{
			Code:      "quota_exceeded",
			UsageType: billing.UsageAIMinutes,
			Required:  1,
		},
	}
	svc := NewService(nil, nil, nil, billingSvc)

	_, err := svc.GetUserToken(context.Background(), "user-1", "User")
	if err == nil {
		t.Fatal("expected AI quota error")
	}
	var quotaErr *billing.QuotaExceededError
	if !errors.As(err, &quotaErr) {
		t.Fatalf("expected quota error, got %T: %v", err, err)
	}
	if len(billingSvc.requireQuotaCalls) != 1 {
		t.Fatalf("expected 1 quota check, got %d", len(billingSvc.requireQuotaCalls))
	}
	if got := billingSvc.requireQuotaCalls[0].UsageType; got != billing.UsageAIMinutes {
		t.Fatalf("expected AI minutes quota check, got %s", got)
	}
}

func TestCreateRoomVideoDoesNotRequireQuota(t *testing.T) {
	billingSvc := &fakeUsageService{
		requireQuotaErr: &billing.QuotaExceededError{
			Code:      "quota_exceeded",
			UsageType: billing.UsageVideoCalls,
			Required:  1,
		},
	}
	svc := NewService(
		lk.NewClient("http://127.0.0.1:1", "key", "secret"),
		nil,
		nil,
		billingSvc,
	)
	caller := testUser("11111111-1111-1111-1111-111111111111", "family")
	callee := testUser("22222222-2222-2222-2222-222222222222", "user")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := svc.CreateRoom(ctx, caller, callee, "video")
	if err == nil {
		t.Fatal("expected create room to fail against test livekit endpoint")
	}
	var quotaErr *billing.QuotaExceededError
	if errors.As(err, &quotaErr) {
		t.Fatalf("video room should not fail on quota: %+v", quotaErr)
	}
	if len(billingSvc.requireQuotaCalls) != 0 {
		t.Fatalf("expected no quota checks for video room creation, got %d", len(billingSvc.requireQuotaCalls))
	}
}

func testUser(id string, role string) dbgen.User {
	return dbgen.User{
		ID:   pgUUID(id),
		Role: role,
	}
}

func pgUUID(raw string) pgtype.UUID {
	return pgtype.UUID{
		Bytes: uuid.MustParse(raw),
		Valid: true,
	}
}
