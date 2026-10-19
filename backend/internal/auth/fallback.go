package auth

import (
	"sync"
	"time"
)

// memLimits is the Limiter's stand-in while Redis is down: the same fixed-window and
// login-lockout rules, kept in this process's memory. It is per instance (a restart or
// another replica starts from zero), which is far weaker than Redis but keeps the brute-force
// brakes on instead of letting every guess through. It covers only what login and token
// refresh need; OTP and signup budgets are not covered, because those flows need Redis anyway.
type memLimits struct {
	mu     sync.Mutex
	hits   map[string]*window   // fixed-window counters
	fails  map[string]*window   // failed logins per email / ip
	locks  map[string]time.Time // login locks
	pruned time.Time
}

type window struct {
	n   int
	end time.Time
}

func newMemLimits() *memLimits {
	return &memLimits{hits: map[string]*window{}, fails: map[string]*window{}, locks: map[string]time.Time{}}
}

// prune drops expired entries, at most once a minute.
func (m *memLimits) prune(now time.Time) {
	if now.Sub(m.pruned) < time.Minute {
		return
	}
	m.pruned = now
	for k, w := range m.hits {
		if now.After(w.end) {
			delete(m.hits, k)
		}
	}
	for k, w := range m.fails {
		if now.After(w.end) {
			delete(m.fails, k)
		}
	}
	for k, t := range m.locks {
		if now.After(t) {
			delete(m.locks, k)
		}
	}
}

// hit counts one request in a fixed window and returns how long to wait once max is passed.
func (m *memLimits) hit(key string, max int, win time.Duration) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.prune(now)
	w := m.hits[key]
	if w == nil || now.After(w.end) {
		w = &window{end: now.Add(win)}
		m.hits[key] = w
	}
	w.n++
	if w.n > max {
		return w.end.Sub(now)
	}
	return 0
}

// locked returns how long login stays refused for the given lock keys.
func (m *memLimits) locked(keys ...string) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	var wait time.Duration
	for _, k := range keys {
		if t, ok := m.locks[k]; ok && t.After(now) {
			wait = max(wait, t.Sub(now))
		}
	}
	return wait
}

// failed records a wrong password for a subject and locks it once max failures were seen,
// for base * 2^(failures-max), capped (the same rule as failScript).
func (m *memLimits) failed(key, lockKey string, maxFails int, base, limit, memory time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.prune(now)
	w := m.fails[key]
	if w == nil || now.After(w.end) {
		w = &window{}
		m.fails[key] = w
	}
	w.n++
	w.end = now.Add(memory)
	if w.n >= maxFails {
		lock := base
		for i := 0; i < min(w.n-maxFails, 20) && lock < limit; i++ {
			lock *= 2
		}
		m.locks[lockKey] = now.Add(min(lock, limit))
	}
}

func (m *memLimits) forget(key, lockKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fails, key)
	delete(m.locks, lockKey)
}
