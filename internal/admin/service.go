package admin

import (
	"context"
	"fmt"
	"strings"
)

type adminStore interface {
	SearchUsers(ctx context.Context, query string, page, limit int32) ([]UserResult, error)
	GetUserProfile(ctx context.Context, userID string) (*UserProfile, error)
	GetFamilyLinks(ctx context.Context, userID string) ([]FamilyLinkResult, error)
	GetUserSubscription(ctx context.Context, userID string) (*AdminSubscription, error)
	GetUserUsage(ctx context.Context, userID string, page, limit int32) ([]AdminUsageEntry, error)
	GetUserEntitlements(ctx context.Context, userID string) ([]AdminEntitlementResult, error)
	GetSubscriptionEvents(ctx context.Context, page, limit int32) ([]AdminSubEvent, error)
	GetAuditLogs(ctx context.Context, userID string, page, limit int32) ([]map[string]any, error)
	GetAnalytics(ctx context.Context) (AdminAnalytics, error)
	GetAICostSummary(ctx context.Context, days int32) (AICostSummary, error)
	GetUserAICostEvents(ctx context.Context, userID string, limit int32) ([]AICostEventResult, error)
	GetTrackingSummary(ctx context.Context, days int32) (TrackingSummary, error)
	GetUserTimeline(ctx context.Context, userID string, limit int32) ([]UserTimelineEvent, error)
	RecordAdminTrackingEvent(ctx context.Context, actorID string, input AdminTrackingEventInput) (TrackingEvent, error)
	ListBillingTransactions(ctx context.Context, page, limit int32) ([]AdminBillingTransaction, error)
	ListUserBillingTransactions(ctx context.Context, userID string, page, limit int32) ([]AdminBillingTransaction, error)
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

type Service struct {
	store adminStore
}

func NewService(store adminStore) *Service {
	return &Service{store: store}
}

func (s *Service) SearchUsers(ctx context.Context, query string, page, limit int32) ([]UserResult, error) {
	return s.store.SearchUsers(ctx, query, page, limit)
}

func (s *Service) GetUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	return s.store.GetUserProfile(ctx, userID)
}

func (s *Service) GetFamilyLinks(ctx context.Context, userID string) ([]FamilyLinkResult, error) {
	return s.store.GetFamilyLinks(ctx, userID)
}

func (s *Service) GetUserSubscription(ctx context.Context, userID string) (*AdminSubscription, error) {
	return s.store.GetUserSubscription(ctx, userID)
}

func (s *Service) GetUserUsage(ctx context.Context, userID string, page, limit int32) ([]AdminUsageEntry, error) {
	return s.store.GetUserUsage(ctx, userID, page, limit)
}

func (s *Service) GetUserEntitlements(ctx context.Context, userID string) ([]AdminEntitlementResult, error) {
	return s.store.GetUserEntitlements(ctx, userID)
}

func (s *Service) GetSubscriptionEvents(ctx context.Context, page, limit int32) ([]AdminSubEvent, error) {
	return s.store.GetSubscriptionEvents(ctx, page, limit)
}

func (s *Service) GetAuditLogs(ctx context.Context, userID string, page, limit int32) ([]map[string]any, error) {
	return s.store.GetAuditLogs(ctx, userID, page, limit)
}

func (s *Service) GetAnalytics(ctx context.Context) (AdminAnalytics, error) {
	return s.store.GetAnalytics(ctx)
}

func (s *Service) GetAICostSummary(ctx context.Context, days int32) (AICostSummary, error) {
	return s.store.GetAICostSummary(ctx, days)
}

func (s *Service) GetUserAICostEvents(ctx context.Context, userID string, limit int32) ([]AICostEventResult, error) {
	return s.store.GetUserAICostEvents(ctx, userID, limit)
}

func (s *Service) GetTrackingSummary(ctx context.Context, days int32) (TrackingSummary, error) {
	if days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	return s.store.GetTrackingSummary(ctx, days)
}

func (s *Service) GetUserTimeline(ctx context.Context, userID string, limit int32) ([]UserTimelineEvent, error) {
	return s.store.GetUserTimeline(ctx, userID, limit)
}

func (s *Service) RecordAdminTrackingEvent(
	ctx context.Context,
	actorID string,
	input AdminTrackingEventInput,
) (TrackingEvent, error) {
	if strings.TrimSpace(actorID) == "" {
		return TrackingEvent{}, fmt.Errorf("actor_id is required")
	}
	input.EventType = strings.TrimSpace(input.EventType)
	if input.EventType == "" {
		return TrackingEvent{}, fmt.Errorf("event_type is required")
	}
	return s.store.RecordAdminTrackingEvent(ctx, actorID, input)
}

func (s *Service) ListBillingTransactions(ctx context.Context, page, limit int32) ([]AdminBillingTransaction, error) {
	return s.store.ListBillingTransactions(ctx, page, limit)
}

func (s *Service) ListUserBillingTransactions(ctx context.Context, userID string, page, limit int32) ([]AdminBillingTransaction, error) {
	return s.store.ListUserBillingTransactions(ctx, userID, page, limit)
}

func (s *Service) IsAdmin(ctx context.Context, userID string) (bool, error) {
	return s.store.IsAdmin(ctx, userID)
}
