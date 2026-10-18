// Package health reports whether the API and its dependencies are usable.
package health

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"backend/pkg/utils"
)

// Check pings one dependency and returns an error if it is unavailable.
type Check func(ctx context.Context) error

// cacheTTL is how long a probe result is reused. /health is public, so without
// this every request would fan out to Postgres, MongoDB and Redis.
const cacheTTL = 3 * time.Second

type Checker struct {
	checks  map[string]Check
	timeout time.Duration

	mu       sync.Mutex
	cachedAt time.Time
	cached   map[string]string
	healthy  bool
}

func New(checks map[string]Check) *Checker {
	return &Checker{checks: checks, timeout: 2 * time.Second}
}

// Handler returns 200 when every dependency responds, otherwise 503 with
// per-dependency status. Raw errors are logged, never returned to the client.
func (c *Checker) Handler(w http.ResponseWriter, r *http.Request) {
	// Holding the lock while probing makes concurrent requests share one run.
	c.mu.Lock()
	if c.cached == nil || time.Since(c.cachedAt) > cacheTTL {
		c.cached, c.healthy = c.probe(r.Context())
		c.cachedAt = time.Now()
	}
	status, healthy := c.cached, c.healthy
	c.mu.Unlock()

	if !healthy {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "service degraded", status)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, status)
}

// probe runs every check and returns the per-dependency status.
func (c *Checker) probe(parent context.Context) (map[string]string, bool) {
	names := make([]string, 0, len(c.checks))
	for name := range c.checks {
		names = append(names, name)
	}
	sort.Strings(names)

	// Run checks concurrently, each with its own timeout, so one hung
	// dependency cannot make the others look unavailable.
	errs := make([]error, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), c.timeout)
			defer cancel()
			errs[i] = c.checks[name](ctx)
		}()
	}
	wg.Wait()

	status := make(map[string]string, len(names))
	healthy := true
	for i, name := range names {
		if errs[i] != nil {
			utils.LogErrorCtx(parent, "health check failed", "dependency", name, "err", errs[i])
			status[name] = "unavailable"
			healthy = false
			continue
		}
		status[name] = "ok"
	}

	return status, healthy
}
