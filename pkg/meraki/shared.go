package meraki

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"
)

// Meraki allows each organization 10 requests a second, with a burst of 10
// more. Every panel of a dashboard queries at once, so requests are paced
// per organization instead of waiting for 429s.
const (
	orgRate  = 10
	orgBurst = 10

	// cacheTTL lets panels that ask the same thing share one response.
	// Meraki's own data is a minute or more behind, so 30 seconds is not
	// noticeable.
	cacheTTL = 30 * time.Second
)

// shared is the state every client made from one New call has in common,
// whichever organization it points at.
type shared struct {
	mu       sync.Mutex
	limiters map[string]*limiter
	cache    map[string]cacheEntry
	inflight map[string]*call
}

type cacheEntry struct {
	rows []json.RawMessage
	exp  time.Time
}

type call struct {
	done chan struct{}
	rows []json.RawMessage
	err  error
}

func newShared() *shared {
	return &shared{limiters: map[string]*limiter{}, cache: map[string]cacheEntry{}, inflight: map[string]*call{}}
}

func (s *shared) limiter(org string) *limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.limiters[org]
	if l == nil {
		l = &limiter{interval: time.Second / orgRate, burst: orgBurst}
		s.limiters[org] = l
	}
	return l
}

// cached returns a fresh copy of the rows for key, fetching them at most
// once at a time and keeping them for cacheTTL. Errors are not kept.
func (s *shared) cached(key string, fetch func() ([]json.RawMessage, error)) ([]json.RawMessage, error) {
	now := time.Now()
	s.mu.Lock()
	if e, ok := s.cache[key]; ok && now.Before(e.exp) {
		s.mu.Unlock()
		return slices.Clone(e.rows), nil
	}
	if c, ok := s.inflight[key]; ok {
		s.mu.Unlock()
		<-c.done
		return slices.Clone(c.rows), c.err
	}
	c := &call{done: make(chan struct{})}
	s.inflight[key] = c
	s.mu.Unlock()

	c.rows, c.err = fetch()

	s.mu.Lock()
	delete(s.inflight, key)
	if c.err == nil {
		for k, e := range s.cache {
			if now.After(e.exp) {
				delete(s.cache, k)
			}
		}
		s.cache[key] = cacheEntry{rows: c.rows, exp: time.Now().Add(cacheTTL)}
	}
	s.mu.Unlock()
	close(c.done)
	return slices.Clone(c.rows), c.err
}

// limiter lets burst requests through at once, then one per interval.
type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	burst    int
	tat      time.Time // when the bucket is next empty
}

func (l *limiter) wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	t := l.tat
	if t.Before(now) {
		t = now
	}
	l.tat = t.Add(l.interval)
	wait := t.Sub(now) - time.Duration(l.burst-1)*l.interval
	l.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
