// Package system exposes installation status to the panel: whether every
// component of the stack is up and talking to each other.
package system

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/lock"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/scheduler"
)

// Status is the response of GET /api/v1/system/status.
type Status struct {
	Version    string                   `json:"version"`
	Healthy    bool                     `json:"healthy"`
	Components map[string]health.Result `json:"components"`
	CheckedAt  time.Time                `json:"checked_at"`
}

// Handler serves system status.
type Handler struct {
	version string
	checker *health.Checker
}

// NewHandler wires every component check.
func NewHandler(version string, pool *pgxpool.Pool, rdb goredis.UniversalClient, insp *asynq.Inspector, cf *centrifugo.Client) *Handler {
	c := health.NewChecker(3*time.Second).
		Add("postgres", health.Postgres(pool)).
		Add("schema", schemaCheck(pool)).
		Add("redis", health.Redis(rdb)).
		Add("worker", workerCheck(insp)).
		Add("scheduler", schedulerCheck(rdb)).
		Add("centrifugo", centrifugoCheck(cf))
	return &Handler{version: version, checker: c}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	results, ok := h.checker.Run(r.Context())
	httpx.JSON(w, r, http.StatusOK, Status{
		Version: h.version, Healthy: ok, Components: results, CheckedAt: time.Now().UTC(),
	})
}

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
