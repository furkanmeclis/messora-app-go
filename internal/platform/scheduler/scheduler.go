// Package scheduler enqueues periodic tasks. Several scheduler processes may
// run; three layers keep them from producing duplicate work:
//
//  1. Only the holder of a Redis lease lock ticks (leader election).
//  2. Each tick's TaskID contains the time slot, and the ID stays reserved for
//     two periods, so a leader change inside a slot cannot enqueue it twice.
//  3. Task handlers claim their work atomically in PostgreSQL.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/platform/lock"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
)

// LeaderKey is the Redis key of the scheduler leader lease.
const LeaderKey = "messora:scheduler:leader"

// Job is a periodic task.
type Job struct {
	Name     string        // stable identifier, used in task IDs
	Every    time.Duration // period; slots are aligned to the Unix epoch
	TaskType string
	Payload  []byte
}

// Options configures a Scheduler.
type Options struct {
	LeaseTTL   time.Duration // default 15s
	Tick       time.Duration // default 1s
	Logger     *slog.Logger
	Registerer prometheus.Registerer
}

// Scheduler runs Jobs while it is the leader.
type Scheduler struct {
	rdb    goredis.UniversalClient
	client *asynq.Client
	jobs   []Job
	o      Options

	isLeader prometheus.Gauge
	enqueued *prometheus.CounterVec
	now      func() time.Time
}

// New validates jobs and builds a scheduler.
func New(rdb goredis.UniversalClient, client *asynq.Client, jobs []Job, o Options) (*Scheduler, error) {
	seen := map[string]bool{}
	for _, j := range jobs {
		if j.Name == "" || j.TaskType == "" || j.Every < time.Second {
			return nil, fmt.Errorf("scheduler: invalid job %+v", j)
		}
		if seen[j.Name] {
			return nil, fmt.Errorf("scheduler: duplicate job name %q", j.Name)
		}
		seen[j.Name] = true
	}
	if o.LeaseTTL <= 0 {
		o.LeaseTTL = 15 * time.Second
	}
	if o.Tick <= 0 {
		o.Tick = time.Second
	}
	s := &Scheduler{
		rdb: rdb, client: client, jobs: jobs, o: o, now: time.Now,
		isLeader: prometheus.NewGauge(prometheus.GaugeOpts{Name: "messora_scheduler_is_leader", Help: "1 when this scheduler holds the leader lease."}),
		enqueued: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "messora_scheduler_enqueued_total", Help: "Periodic tasks enqueued by job and result."}, []string{"job", "result"}),
	}
	if o.Registerer != nil {
		o.Registerer.MustRegister(s.isLeader, s.enqueued)
	}
	return s, nil
}

// Run blocks until ctx is cancelled, releasing leadership on exit.
func (s *Scheduler) Run(ctx context.Context) error {
	log := s.o.Logger
	var lease *lock.Lease
	lastSlot := map[string]int64{}
	renewEvery := s.o.LeaseTTL / 3
	var lastRenew time.Time

	ticker := time.NewTicker(s.o.Tick)
	defer ticker.Stop()
	defer func() {
		if lease != nil {
			rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = lease.Release(rctx)
			cancel()
			s.isLeader.Set(0)
			log.Info("scheduler leadership released")
		}
	}()

	for {
		if lease == nil {
			l, ok, err := lock.TryAcquire(ctx, s.rdb, LeaderKey, s.o.LeaseTTL)
			switch {
			case err != nil && ctx.Err() == nil:
				log.Warn("scheduler leader election failed", "err", err)
			case ok:
				lease, lastRenew = l, s.now()
				clear(lastSlot)
				s.isLeader.Set(1)
				log.Info("scheduler became leader")
			}
		} else if s.now().Sub(lastRenew) >= renewEvery {
			if err := lease.Renew(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Warn("scheduler lost leadership", "err", err)
				lease = nil
				s.isLeader.Set(0)
			} else {
				lastRenew = s.now()
			}
		}

		if lease != nil {
			s.tick(ctx, lastSlot)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, lastSlot map[string]int64) {
	now := s.now()
	for _, j := range s.jobs {
		slot := now.Truncate(j.Every).Unix()
		if lastSlot[j.Name] == slot {
			continue
		}
		err := queue.Enqueue(ctx, s.client, j.TaskType, j.Payload,
			asynq.TaskID(SlotTaskID(j.Name, slot)),
			asynq.Retention(2*j.Every),
			asynq.Timeout(j.Every),
		)
		switch {
		case err == nil:
			s.enqueued.WithLabelValues(j.Name, "enqueued").Inc()
			lastSlot[j.Name] = slot
		case errors.Is(err, queue.ErrDuplicate):
			s.enqueued.WithLabelValues(j.Name, "duplicate").Inc()
			lastSlot[j.Name] = slot
		default:
			s.enqueued.WithLabelValues(j.Name, "error").Inc()
			s.o.Logger.Warn("scheduler enqueue failed; will retry next tick", "job", j.Name, "err", err)
		}
	}
}

// SlotTaskID is the deterministic task ID of a job's time slot.
func SlotTaskID(job string, slotUnix int64) string {
	return fmt.Sprintf("sched:%s:%d", job, slotUnix)
}
