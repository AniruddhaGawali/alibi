// Package health reports whether the API and its dependencies are usable.
package health

import (
	"context"
	"net/http"
	"sort"
	"time"

	"backend/pkg/utils"
)

// Check pings one dependency and returns an error if it is unavailable.
type Check func(ctx context.Context) error

type Checker struct {
	checks  map[string]Check
	timeout time.Duration
}

func New(checks map[string]Check) *Checker {
	return &Checker{checks: checks, timeout: 2 * time.Second}
}

// Handler returns 200 when every dependency responds, otherwise 503 with
// per-dependency status. Raw errors are logged, never returned to the client.
func (c *Checker) Handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), c.timeout)
	defer cancel()

	names := make([]string, 0, len(c.checks))
	for name := range c.checks {
		names = append(names, name)
	}
	sort.Strings(names)

	status := make(map[string]string, len(names))
	healthy := true
	for _, name := range names {
		if err := c.checks[name](ctx); err != nil {
			utils.LogErrorCtx(r.Context(), "health check failed", "dependency", name, "err", err)
			status[name] = "unavailable"
			healthy = false
			continue
		}
		status[name] = "ok"
	}

	if !healthy {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "service degraded", status)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, status)
}
