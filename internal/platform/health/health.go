// Package health implements liveness and readiness checks.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
)

// Check probes one dependency. It returns optional detail and an error when
// the dependency is unhealthy.
type Check func(ctx context.Context) (detail any, err error)

// Result is one check's outcome.
type Result struct {
	Status string `json:"status"` // ok | fail
	Detail any    `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Checker runs named checks in parallel with a timeout.
type Checker struct {
	checks  map[string]Check
	timeout time.Duration
}

// NewChecker returns a checker with the given per-run timeout.
func NewChecker(timeout time.Duration) *Checker {
	return &Checker{checks: map[string]Check{}, timeout: timeout}
}

// Add registers a check.
func (c *Checker) Add(name string, fn Check) *Checker { c.checks[name] = fn; return c }

// Run executes every check.
func (c *Checker) Run(ctx context.Context) (map[string]Result, bool) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make(map[string]Result, len(c.checks))
	healthy := true
	for name, fn := range c.checks {
		wg.Go(func() {
			detail, err := fn(ctx)
			r := Result{Status: "ok", Detail: detail}
			if err != nil {
				r.Status, r.Error = "fail", err.Error()
			}
			mu.Lock()
			out[name] = r
			if err != nil {
				healthy = false
			}
			mu.Unlock()
		})
	}
	wg.Wait()
	return out, healthy
}

// LiveHandler answers 200 while the process is serving.
func LiveHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReadyHandler answers 200 when every check passes, 503 otherwise. It is only
// served on internal listeners, so check details are not exposed publicly.
func (c *Checker) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	results, ok := c.Run(r.Context())
	status, code := "ok", http.StatusOK
	if !ok {
		status, code = "fail", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": results})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Postgres pings the pool and reports pool statistics.
func Postgres(pool *pgxpool.Pool) Check {
	return func(ctx context.Context) (any, error) {
		if err := pool.Ping(ctx); err != nil {
			return nil, fmt.Errorf("ping: %w", err)
		}
		s := pool.Stat()
		return map[string]int32{"total_conns": s.TotalConns(), "idle_conns": s.IdleConns(), "max_conns": s.MaxConns()}, nil
	}
}

// Schema fails while migrations are pending.
func Schema(pool *pgxpool.Pool) Check {
	return func(ctx context.Context) (any, error) {
		st, err := database.Status(ctx, pool)
		if err != nil {
			return nil, err
		}
		if !st.UpToDate() {
			return st, fmt.Errorf("schema at version %d, expected %d: run migrations", st.Current, st.Target)
		}
		return st, nil
	}
}

// Redis pings Redis.
func Redis(rdb goredis.UniversalClient) Check {
	return func(ctx context.Context) (any, error) {
		return nil, rdb.Ping(ctx).Err()
	}
}
