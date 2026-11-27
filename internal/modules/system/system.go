// Package system exposes installation status to the panel: whether every
// component of the stack is up and talking to each other.
package system

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/lock"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/scheduler"
)

// Handler serves system status and the failed-jobs (dead letter) view.
type Handler struct {
	version string
	checker *health.Checker
	jobs    *FailedJobs
	q       *db.Queries
	log     *slog.Logger
}

// NewHandler wires every component check.
func NewHandler(version string, pool *pgxpool.Pool, rdb goredis.UniversalClient, insp *asynq.Inspector, cf *centrifugo.Client, log *slog.Logger) *Handler {
	c := health.NewChecker(3*time.Second).
		Add("postgres", health.Postgres(pool)).
		Add("schema", schemaCheck(pool)).
		Add("redis", health.Redis(rdb)).
		Add("worker", workerCheck(insp)).
		Add("scheduler", schedulerCheck(rdb)).
		Add("centrifugo", centrifugoCheck(cf))
	return &Handler{version: version, checker: c, jobs: NewFailedJobs(insp), q: db.New(pool), log: log}
}

// FailedJobs is the dead letter, for the platform admin panel
// (PlatformScope) and tests.
func (h *Handler) FailedJobs() *FailedJobs { return h.jobs }

// FailedJobsTotal is the number of ctx's tenant's failed jobs in every
// queue.
func (h *Handler) FailedJobsTotal(ctx context.Context) (int, error) {
	sc, err := TenantScope(ctx)
	if err != nil {
		return 0, err
	}
	counts, err := h.jobs.Count(sc)
	total := 0
	for _, n := range counts {
		total += n
	}
	return total, err
}

// GetSystemStatus implements api.ServerInterface.
func (h *Handler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	var failed int64
	if n, err := h.FailedJobsTotal(r.Context()); err == nil { // Redis trouble shows in the redis check
		failed = int64(n)
	}
	results, ok := h.checker.Run(r.Context())
	comps := make(map[string]api.ComponentStatus, len(results))
	for k, v := range results {
		c := api.ComponentStatus{Status: api.ComponentStatusStatus(v.Status), Detail: v.Detail}
		if v.Error != "" {
			e := v.Error
			c.Error = &e
		}
		comps[k] = c
	}
	httpx.JSON(w, r, http.StatusOK, api.SystemStatus{Version: h.version, Healthy: ok, Components: comps, CheckedAt: time.Now().UTC(), FailedJobs: failed})
}

// ReadyHandler serves the same checks on the internal port: 200 only when
// every component — including worker, scheduler and Centrifugo — is healthy.
func (h *Handler) ReadyHandler(w http.ResponseWriter, r *http.Request) { h.checker.ReadyHandler(w, r) }

func schemaCheck(pool *pgxpool.Pool) health.Check {
	return func(ctx context.Context) (any, error) {
		st, err := database.Status(ctx, pool)
		if err != nil {
			return nil, err
		}
		if !st.UpToDate() {
			return st, errors.New("migrations pending")
		}
		return st, nil
	}
}

type workerDetail struct {
	Servers int            `json:"servers"`
	Workers int            `json:"active_tasks"`
	Queues  map[string]int `json:"pending_by_queue"`
}

func workerCheck(insp *asynq.Inspector) health.Check {
	return func(ctx context.Context) (any, error) {
		type result struct {
			d   workerDetail
			err error
		}
		ch := make(chan result, 1)
		go func() { // the inspector API takes no context
			servers, err := insp.Servers()
			if err != nil {
				ch <- result{err: err}
				return
			}
			d := workerDetail{Servers: len(servers), Queues: map[string]int{}}
			for _, s := range servers {
				d.Workers += len(s.ActiveWorkers)
			}
			for _, q := range queue.AllQueues {
				if info, err := insp.GetQueueInfo(q); err == nil {
					d.Queues[q] = info.Pending
				}
			}
			ch <- result{d: d}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-ch:
			if r.err != nil {
				return nil, r.err
			}
			if r.d.Servers == 0 {
				return r.d, errors.New("no worker process is connected to the queue")
			}
			return r.d, nil
		}
	}
}

func schedulerCheck(rdb goredis.UniversalClient) health.Check {
	return func(ctx context.Context) (any, error) {
		held, err := lock.IsHeldByAnyone(ctx, rdb, scheduler.LeaderKey)
		if err != nil {
			return nil, err
		}
		if !held {
			return map[string]bool{"leader": false}, errors.New("no scheduler holds the leader lease")
		}
		return map[string]bool{"leader": true}, nil
	}
}

func centrifugoCheck(cf *centrifugo.Client) health.Check {
	return func(ctx context.Context) (any, error) {
		nodes, err := cf.Info(ctx)
		if err != nil {
			return nil, err
		}
		if len(nodes) == 0 {
			return nil, errors.New("no centrifugo nodes reported")
		}
		return nodes, nil
	}
}
