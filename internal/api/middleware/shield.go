package middleware

import (
	"sync"
	"time"
)

// Per-replica flood shield. Advertised caps come from Postgres; this only
// sheds abusive bursts before they hit the shared store. 200 rps is well
// above configured RPM values (admin 300/min, ingest 10/min).
const (
	shieldCapacity     = 200
	shieldRefillPerSec = 200.0
	shieldIdleTTL      = 2 * time.Minute
)

type burstShield struct {
	mu      sync.Mutex
	buckets map[string]shieldBucket
}

type shieldBucket struct {
	tokens float64
	last   time.Time
}

func newBurstShield() *burstShield {
	return &burstShield{buckets: make(map[string]shieldBucket)}
}

func (s *burstShield) allow(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buckets) > 1024 {
		s.evictIdle(now)
	}

	b := s.buckets[key]
	if b.last.IsZero() {
		b.tokens = shieldCapacity
		b.last = now
	} else {
		elapsed := now.Sub(b.last).Seconds()
		if elapsed > 0 {
			b.tokens += elapsed * shieldRefillPerSec
			if b.tokens > shieldCapacity {
				b.tokens = shieldCapacity
			}
		}
	}

	if b.tokens < 1 {
		b.last = now
		s.buckets[key] = b

		return false
	}

	b.tokens--
	b.last = now
	s.buckets[key] = b

	return true
}

func (s *burstShield) evictIdle(now time.Time) {
	for key, b := range s.buckets {
		if now.Sub(b.last) > shieldIdleTTL {
			delete(s.buckets, key)
		}
	}
}
