package auth

import (
	"sync"
	"time"
)

// RateLimiter is a simple in-memory fixed-window attempt counter keyed by an
// arbitrary string (e.g. username or client IP tag). It is used to throttle
// login attempts and manual announcements.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*window
	max      int
	window   time.Duration
	now      func() time.Time
}

type window struct {
	count int
	reset time.Time
}

// NewRateLimiter builds a limiter allowing max events per window per key.
func NewRateLimiter(max int, w time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]*window),
		max:      max,
		window:   w,
		now:      time.Now,
	}
}

// SetClock overrides the clock (tests).
func (r *RateLimiter) SetClock(now func() time.Time) { r.now = now }

// Allow records an attempt for key and reports whether it is permitted and how
// long until the window resets if not.
func (r *RateLimiter) Allow(key string) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	w, ok := r.attempts[key]
	if !ok || now.After(w.reset) {
		r.attempts[key] = &window{count: 1, reset: now.Add(r.window)}
		return true, 0
	}
	if w.count >= r.max {
		return false, w.reset.Sub(now)
	}
	w.count++
	return true, 0
}

// Reset clears the counter for a key (e.g. after a successful login).
func (r *RateLimiter) Reset(key string) {
	r.mu.Lock()
	delete(r.attempts, key)
	r.mu.Unlock()
}

// Sweep removes expired windows to bound memory.
func (r *RateLimiter) Sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for k, w := range r.attempts {
		if now.After(w.reset) {
			delete(r.attempts, k)
		}
	}
}
