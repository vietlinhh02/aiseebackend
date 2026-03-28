package reminder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type mockHandler struct {
	mu        sync.Mutex
	reminders []Reminder
	dueCalls  []string
}

func (m *mockHandler) LoadAll(
	_ context.Context,
) ([]Reminder, error) {
	return m.reminders, nil
}

func (m *mockHandler) OnDue(
	_ context.Context, key string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dueCalls = append(m.dueCalls, key)
	return nil
}

func (m *mockHandler) getDueCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.dueCalls))
	copy(cp, m.dueCalls)
	return cp
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skip("redis not available:", err)
	}
	rdb.FlushDB(context.Background())
	return rdb
}

func TestScheduler_FiresDueReminders(t *testing.T) {
	rdb := newTestRedis(t)
	handler := &mockHandler{
		reminders: []Reminder{
			{Key: "med-1:0", DueAt: time.Now().Add(-1 * time.Second)},
			{Key: "med-2:0", DueAt: time.Now().Add(1 * time.Hour)},
		},
	}

	s := New(rdb, 100*time.Millisecond)
	s.Register("test", handler)

	ctx, cancel := context.WithCancel(context.Background())
	go s.Start(ctx)

	time.Sleep(300 * time.Millisecond)
	cancel()

	calls := handler.getDueCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 due call, got %d: %v", len(calls), calls)
	}
	if calls[0] != "med-1:0" {
		t.Errorf("expected med-1:0, got %s", calls[0])
	}
}

func TestScheduler_ScheduleAndUnschedule(t *testing.T) {
	rdb := newTestRedis(t)
	s := New(rdb, 100*time.Millisecond)
	handler := &mockHandler{}
	s.Register("test", handler)

	ctx := context.Background()

	err := s.Schedule(ctx, "test", Reminder{
		Key:   "apt-1:1h",
		DueAt: time.Now().Add(-1 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = s.Unschedule(ctx, "test", "apt-1:1h")
	if err != nil {
		t.Fatal(err)
	}

	count, _ := rdb.ZCard(ctx, "reminders:test").Result()
	if count != 0 {
		t.Errorf("expected 0 members, got %d", count)
	}
}
