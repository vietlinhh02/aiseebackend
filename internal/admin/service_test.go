package admin

import (
	"context"
	"testing"
)

type fakeAdminStore struct {
	listPage   int32
	listLimit  int32
	listResult []AdminBillingTransaction
	userID     string
	userPage   int32
	userLimit  int32
	userResult []AdminBillingTransaction
	days       int32
	summary    TrackingSummary
	timeline   []UserTimelineEvent
	tracked    AdminTrackingEventInput
}

func (f *fakeAdminStore) SearchUsers(context.Context, string, int32, int32) ([]UserResult, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetUserProfile(context.Context, string) (*UserProfile, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetFamilyLinks(context.Context, string) ([]FamilyLinkResult, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetUserSubscription(context.Context, string) (*AdminSubscription, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetUserUsage(context.Context, string, int32, int32) ([]AdminUsageEntry, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetUserEntitlements(context.Context, string) ([]AdminEntitlementResult, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetSubscriptionEvents(context.Context, int32, int32) ([]AdminSubEvent, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetAuditLogs(context.Context, string, int32, int32) ([]map[string]any, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetAnalytics(context.Context) (AdminAnalytics, error) {
	return AdminAnalytics{}, nil
}

func (f *fakeAdminStore) GetAICostSummary(context.Context, int32) (AICostSummary, error) {
	return AICostSummary{}, nil
}

func (f *fakeAdminStore) GetUserAICostEvents(context.Context, string, int32) ([]AICostEventResult, error) {
	return nil, nil
}

func (f *fakeAdminStore) GetTrackingSummary(_ context.Context, days int32) (TrackingSummary, error) {
	f.days = days
	return f.summary, nil
}

func (f *fakeAdminStore) GetUserTimeline(context.Context, string, int32) ([]UserTimelineEvent, error) {
	return f.timeline, nil
}

func (f *fakeAdminStore) RecordAdminTrackingEvent(
	_ context.Context,
	_ string,
	input AdminTrackingEventInput,
) (TrackingEvent, error) {
	f.tracked = input
	return TrackingEvent{ID: "track-1", EventType: input.EventType}, nil
}

func (f *fakeAdminStore) ListBillingTransactions(
	_ context.Context,
	page,
	limit int32,
) ([]AdminBillingTransaction, error) {
	f.listPage = page
	f.listLimit = limit
	return f.listResult, nil
}

func (f *fakeAdminStore) ListUserBillingTransactions(
	_ context.Context,
	userID string,
	page,
	limit int32,
) ([]AdminBillingTransaction, error) {
	f.userID = userID
	f.userPage = page
	f.userLimit = limit
	return f.userResult, nil
}

func (f *fakeAdminStore) IsAdmin(context.Context, string) (bool, error) {
	return true, nil
}

func TestServiceListBillingTransactions(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{
		listResult: []AdminBillingTransaction{{ID: "txn-1"}},
	}
	svc := NewService(store)

	got, err := svc.ListBillingTransactions(context.Background(), 3, 50)
	if err != nil {
		t.Fatalf("ListBillingTransactions error = %v", err)
	}
	if store.listPage != 3 || store.listLimit != 50 {
		t.Fatalf("store args = (%d,%d), want (3,50)", store.listPage, store.listLimit)
	}
	if len(got) != 1 || got[0].ID != "txn-1" {
		t.Fatalf("got = %+v", got)
	}
}

func TestServiceListUserBillingTransactions(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{
		userResult: []AdminBillingTransaction{{ID: "txn-2"}},
	}
	svc := NewService(store)

	got, err := svc.ListUserBillingTransactions(context.Background(), "user-7", 2, 25)
	if err != nil {
		t.Fatalf("ListUserBillingTransactions error = %v", err)
	}
	if store.userID != "user-7" || store.userPage != 2 || store.userLimit != 25 {
		t.Fatalf(
			"store args = (%q,%d,%d), want (user-7,2,25)",
			store.userID,
			store.userPage,
			store.userLimit,
		)
	}
	if len(got) != 1 || got[0].ID != "txn-2" {
		t.Fatalf("got = %+v", got)
	}
}

func TestServiceGetTrackingSummaryDefaultsWindow(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{
		summary: TrackingSummary{
			AISessionsStarted: 10,
			AISessionsFailed:  2,
		},
	}
	svc := NewService(store)

	got, err := svc.GetTrackingSummary(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetTrackingSummary error = %v", err)
	}
	if store.days != 30 {
		t.Fatalf("days = %d, want 30", store.days)
	}
	if got.AISessionsStarted != 10 || got.AISessionsFailed != 2 {
		t.Fatalf("summary = %+v", got)
	}
}
