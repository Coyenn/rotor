package cloud

import (
	"context"
	"sync"
	"time"
)

// limiter is a per-host token-bucket rate limiter. One Client talks to one
// host in practice, but bucketing by host keeps a custom baseURL (tests,
// proxies) from sharing budget with production.
type limiter struct {
	rate  float64 // tokens added per second; <= 0 disables limiting
	burst float64

	mu    sync.Mutex
	hosts map[string]*bucket
}

type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func newLimiter(perSecond float64, burst int) *limiter {
	if burst < 1 {
		burst = 1
	}
	return &limiter{rate: perSecond, burst: float64(burst), hosts: map[string]*bucket{}}
}

// wait blocks until a token is available for host or ctx is done. The bucket
// lock is never held while sleeping, so concurrent callers make independent
// progress and cannot deadlock.
func (l *limiter) wait(ctx context.Context, host string) error {
	if l.rate <= 0 {
		return ctx.Err()
	}
	l.mu.Lock()
	b := l.hosts[host]
	if b == nil {
		b = &bucket{tokens: l.burst, last: time.Now()}
		l.hosts[host] = b
	}
	l.mu.Unlock()

	for {
		b.mu.Lock()
		now := time.Now()
		b.tokens += now.Sub(b.last).Seconds() * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		need := time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
		b.mu.Unlock()
		if err := sleep(ctx, need); err != nil {
			return err
		}
	}
}
