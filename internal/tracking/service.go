package tracking

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Store interface {
	RecordEvent(context.Context, EventInput) (Event, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

var validCategories = map[string]bool{
	"ai_ops":        true,
	"safety_care":   true,
	"admin_support": true,
	"system":        true,
}

var validSeverities = map[string]bool{
	"info":     true,
	"warning":  true,
	"critical": true,
}

var redactedMetadataKeys = map[string]bool{
	"audio":      true,
	"image":      true,
	"transcript": true,
	"video":      true,
	"voice_url":  true,
}

func (s *Service) RecordEvent(ctx context.Context, input EventInput) (Event, error) {
	input.EventType = strings.TrimSpace(input.EventType)
	input.Category = strings.TrimSpace(input.Category)
	input.UserID = strings.TrimSpace(input.UserID)
	input.Severity = strings.TrimSpace(input.Severity)
	input.Source = strings.TrimSpace(input.Source)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.Status = strings.TrimSpace(input.Status)

	if input.EventType == "" {
		return Event{}, fmt.Errorf("event_type is required")
	}
	if input.UserID == "" {
		return Event{}, fmt.Errorf("user_id is required")
	}
	if !validCategories[input.Category] {
		return Event{}, fmt.Errorf("unsupported category: %s", input.Category)
	}
	if input.Source == "" {
		return Event{}, fmt.Errorf("source is required")
	}
	if input.SourceID == "" {
		return Event{}, fmt.Errorf("source_id is required")
	}
	if input.Severity == "" {
		input.Severity = defaultSeverity(input.EventType)
	}
	if !validSeverities[input.Severity] {
		return Event{}, fmt.Errorf("unsupported severity: %s", input.Severity)
	}
	if input.Status == "" {
		input.Status = defaultStatus(input.EventType)
	}
	if input.DurationMS < 0 {
		return Event{}, fmt.Errorf("duration_ms must be >= 0")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	input.Metadata = sanitizeMetadata(input.Metadata)
	return s.store.RecordEvent(ctx, input)
}

func defaultSeverity(eventType string) string {
	if strings.Contains(eventType, "failed") ||
		strings.Contains(eventType, "fallback") ||
		strings.Contains(eventType, "skipped") ||
		strings.Contains(eventType, "blocked") ||
		strings.Contains(eventType, "abandoned") {
		return "warning"
	}
	if strings.Contains(eventType, "emergency") {
		return "critical"
	}
	return "info"
}

func defaultStatus(eventType string) string {
	switch {
	case strings.Contains(eventType, "failed"):
		return "failed"
	case strings.Contains(eventType, "completed"),
		strings.Contains(eventType, "ended"),
		strings.Contains(eventType, "taken"),
		strings.Contains(eventType, "acknowledged"):
		return "completed"
	case strings.Contains(eventType, "skipped"),
		strings.Contains(eventType, "abandoned"):
		return "warning"
	default:
		return "recorded"
	}
}

func sanitizeMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	clean := make(map[string]any, len(metadata))
	for key, value := range metadata {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if redactedMetadataKeys[normalized] {
			continue
		}
		clean[key] = value
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}
