package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_auth_by_doubao/internal/store"
)

type Limiter struct {
	repo *store.Repository
	now  func() time.Time
}

func New(repo *store.Repository) *Limiter {
	return &Limiter{repo: repo, now: time.Now}
}

func (l *Limiter) Allow(ctx context.Context, svc store.Service) error {
	now := l.now()
	if svc.QPS > 0 {
		allowed, err := l.hit(ctx, l.repo.RateSecondKey(svc.ID, now.Unix()), 2*time.Second, svc.QPS)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("qps limit exceeded")
		}
	}
	if svc.QPM > 0 {
		minute := now.Unix() / 60
		allowed, err := l.hit(ctx, l.repo.RateMinuteKey(svc.ID, minute), 70*time.Second, svc.QPM)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("qpm limit exceeded")
		}
	}
	return nil
}

func (l *Limiter) hit(ctx context.Context, key string, ttl time.Duration, limit int) (bool, error) {
	pipe := l.repo.Redis().Pipeline()
	count := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("rate limit redis error: %w", err)
	}
	return count.Val() <= int64(limit), nil
}
