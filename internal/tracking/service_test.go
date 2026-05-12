package tracking

import (
	"context"
	"testing"
	"time"
)

type fakeStore struct {
	input EventInput
	event Event
}

func (f *fakeStore) RecordEvent(_ context.Context, input EventInput) (Event, error) {
	f.input = input
	if f.event.ID == "" {
		f.event = Event{
			ID:         "event-1",
			UserID:     input.UserID,
			EventType:  input.EventType,
			Category:   input.Category,
			Severity:   input.Severity,
			Source:     input.Source,
			SourceID:   input.SourceID,
			Status:     input.Status,
			OccurredAt: input.OccurredAt,
			Metadata:   input.Metadata,
		}
	}
	return f.event, nil
}

func TestRecordEventDefaultsAndSanitizesMetadata(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	svc := NewService(store)
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)

	event, err := svc.RecordEvent(context.Background(), EventInput{
		UserID:     "user-1",
		EventType:  "ai_session_failed",
		Category:   "ai_ops",
		Source:     "agent_worker",
		SourceID:   "session-1",
		OccurredAt: now,
		Metadata: map[string]any{
			"error_code": "gemini_quota_exceeded",
			"transcript": "private speech",
			"voice_url":  "s3://private",
		},
	})
	if err != nil {
		t.Fatalf("RecordEvent error = %v", err)
	}
	if store.input.Severity != "warning" {
		t.Fatalf("severity = %q, want warning", store.input.Severity)
	}
	if store.input.Status != "failed" {
		t.Fatalf("status = %q, want failed", store.input.Status)
	}
	if _, ok := store.input.Metadata["transcript"]; ok {
		t.Fatal("transcript metadata was not redacted")
	}
	if _, ok := store.input.Metadata["voice_url"]; ok {
		t.Fatal("voice_url metadata was not redacted")
	}
	if store.input.Metadata["error_code"] != "gemini_quota_exceeded" {
		t.Fatalf("metadata = %+v, want error_code preserved", store.input.Metadata)
	}
	if event.ID != "event-1" {
		t.Fatalf("event = %+v", event)
	}
}

func TestRecordEventRejectsUnsupportedCategory(t *testing.T) {
	t.Parallel()

	_, err := NewService(&fakeStore{}).RecordEvent(context.Background(), EventInput{
		UserID:    "user-1",
		EventType: "ai_session_started",
		Category:  "marketing",
		Source:    "agent_worker",
		SourceID:  "session-1",
	})
	if err == nil {
		t.Fatal("RecordEvent error = nil, want validation error")
	}
}
