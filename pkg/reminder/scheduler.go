package reminder

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Reminder struct {
	Key   string
	DueAt time.Time
}

type Handler interface {
	LoadAll(ctx context.Context) ([]Reminder, error)
	OnDue(ctx context.Context, key string) error
}

type handlerEntry struct {
	handler  Handler
	redisKey string
}

type Scheduler struct {
	rdb      *redis.Client
	entries  []handlerEntry
	interval time.Duration
	mu       sync.Mutex
	cancel   context.CancelFunc
}

func New(rdb *redis.Client, interval time.Duration) *Scheduler {
	return &Scheduler{
		rdb:      rdb,
		interval: interval,
	}
}

func (s *Scheduler) Register(name string, h Handler) {
	s.entries = append(s.entries, handlerEntry{
		handler:  h,
		redisKey: "reminders:" + name,
	})
}

func (s *Scheduler) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	for _, entry := range s.entries {
		reminders, err := entry.handler.LoadAll(ctx)
		if err != nil {
			return fmt.Errorf(
				"load reminders for %s: %w",
				entry.redisKey, err,
			)
		}
		for _, r := range reminders {
			s.rdb.ZAdd(ctx, entry.redisKey, redis.Z{
				Score:  float64(r.DueAt.Unix()),
				Member: r.Key,
			})
		}
		slog.Info("loaded reminders",
			"key", entry.redisKey,
			"count", len(reminders),
		)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := float64(time.Now().Unix())
	for _, entry := range s.entries {
		members, err := s.rdb.ZRangeByScore(
			ctx, entry.redisKey,
			&redis.ZRangeBy{
				Min: "-inf",
				Max: fmt.Sprintf("%f", now),
			},
		).Result()
		if err != nil {
			slog.Error("redis zrangebyscore failed",
				"key", entry.redisKey, "error", err,
			)
			continue
		}
		for _, member := range members {
			s.rdb.ZRem(ctx, entry.redisKey, member)
			if err := entry.handler.OnDue(ctx, member); err != nil {
				slog.Error("reminder handler failed",
					"key", entry.redisKey,
					"member", member,
					"error", err,
				)
			}
		}
	}
}

func (s *Scheduler) Schedule(
	ctx context.Context, name string, r Reminder,
) error {
	key := "reminders:" + name
	return s.rdb.ZAdd(ctx, key, redis.Z{
		Score:  float64(r.DueAt.Unix()),
		Member: r.Key,
	}).Err()
}

func (s *Scheduler) Unschedule(
	ctx context.Context, name string, memberKey string,
) error {
	key := "reminders:" + name
	return s.rdb.ZRem(ctx, key, memberKey).Err()
}
