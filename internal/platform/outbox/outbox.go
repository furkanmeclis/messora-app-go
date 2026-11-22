// Package outbox implements the transactional outbox. Producers call Write
// inside the same transaction as their business change; the Relay running in
// the worker publishes committed events at least once. Consumers must be
// idempotent; for queue tasks the TaskID "outbox:<id>" makes re-publication a
// no-op.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox/store"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
)

// TaskTopicPrefix marks events that become queue tasks: "task:<task_type>".
const TaskTopicPrefix = "task:"

// TaskTopic returns the outbox topic for an asynq task type.
func TaskTopic(taskType string) string { return TaskTopicPrefix + taskType }

// DBTX is satisfied by pgx.Tx, *pgxpool.Pool and *pgx.Conn.
type DBTX = store.DBTX

// Write records an event, publishable after delay (0 = immediately). Pass the
// caller's transaction so the event commits or rolls back together with the
// business change.
func Write(ctx context.Context, db DBTX, topic string, payload any, delay time.Duration) (int64, error) {
	if topic == "" {
		return 0, errors.New("outbox: empty topic")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("outbox: marshal payload: %w", err)
	}
	if delay < 0 {
		delay = 0
	}
	return store.New(db).InsertOutboxEvent(ctx, store.InsertOutboxEventParams{
		Topic:        topic,
		Payload:      b,
		DelaySeconds: delay.Seconds(),
	})
}

// Event is a claimed outbox row handed to a Publisher.
type Event struct {
	ID       int64
	Topic    string
	Payload  []byte
	Attempts int32
}

// Publisher delivers events of one topic family.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// PublisherFunc adapts a function to Publisher.
type PublisherFunc func(ctx context.Context, e Event) error

// Publish implements Publisher.
func (f PublisherFunc) Publish(ctx context.Context, e Event) error { return f(ctx, e) }

// TaskPublisher enqueues "task:<type>" events into asynq with TaskID
// "outbox:<id>", so publishing the same event twice is harmless.
func TaskPublisher(c *asynq.Client) Publisher {
	return PublisherFunc(func(ctx context.Context, e Event) error {
		taskType := strings.TrimPrefix(e.Topic, TaskTopicPrefix)
		err := queue.Enqueue(ctx, c, taskType, e.Payload,
			asynq.TaskID(fmt.Sprintf("outbox:%d", e.ID)),
			asynq.Retention(24*time.Hour), // keep the ID reserved after completion
		)
		if errors.Is(err, queue.ErrDuplicate) {
			return nil
		}
		return err
	})
}

// Router picks a Publisher by topic prefix (longest match wins).
type Router struct {
	routes map[string]Publisher
}

// NewRouter returns an empty router.
func NewRouter() *Router { return &Router{routes: map[string]Publisher{}} }

// Handle registers p for topics starting with prefix.
func (r *Router) Handle(prefix string, p Publisher) { r.routes[prefix] = p }

func (r *Router) lookup(topic string) (Publisher, bool) {
	var best string
	var pub Publisher
	for prefix, p := range r.routes {
		if strings.HasPrefix(topic, prefix) && len(prefix) >= len(best) {
			best, pub = prefix, p
		}
	}
	return pub, pub != nil
}

// RelayOptions configures a Relay.
type RelayOptions struct {
	BatchSize    int32
	PollInterval time.Duration // fallback when no NOTIFY arrives
	Logger       *slog.Logger
	Registerer   prometheus.Registerer
}

// Relay drains the outbox.
type Relay struct {
	pool   *pgxpool.Pool
	router *Router
	o      RelayOptions

	published prometheus.Counter
	failed    prometheus.Counter
}

// NewRelay builds a relay; call Run in its own goroutine.
func NewRelay(pool *pgxpool.Pool, router *Router, o RelayOptions) *Relay {
	if o.BatchSize <= 0 {
		o.BatchSize = 200
	}
	if o.PollInterval <= 0 {
		o.PollInterval = time.Second
	}
	r := &Relay{
		pool: pool, router: router, o: o,
		published: prometheus.NewCounter(prometheus.CounterOpts{Name: "messora_outbox_published_total", Help: "Outbox events published."}),
		failed:    prometheus.NewCounter(prometheus.CounterOpts{Name: "messora_outbox_publish_failures_total", Help: "Outbox publish attempts that failed and were rescheduled."}),
	}
	if o.Registerer != nil {
		o.Registerer.MustRegister(r.published, r.failed)
		o.Registerer.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "messora_outbox_backlog", Help: "Unpublished outbox events.",
		}, func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			n, err := store.New(pool).CountUnpublishedOutbox(ctx)
			if err != nil {
				return -1
			}
			return float64(n)
		}))
	}
	return r
}

// Run drains until ctx is cancelled. It listens for NOTIFY 'outbox' and also
// polls, so a lost notification only delays publication by PollInterval.
func (r *Relay) Run(ctx context.Context) error {
	log := r.o.Logger
	for ctx.Err() == nil {
		if err := r.listenLoop(ctx); err != nil && ctx.Err() == nil {
			log.Warn("outbox listener restarting", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil
}

func (r *Relay) listenLoop(ctx context.Context) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN outbox"); err != nil {
		return err
	}
	for ctx.Err() == nil {
		for {
			n, err := r.DrainOnce(ctx)
			if err != nil {
				return err
			}
			if n < int(r.o.BatchSize) {
				break
			}
		}
		waitCtx, cancel := context.WithTimeout(ctx, r.o.PollInterval)
		_, err := conn.Conn().WaitForNotification(waitCtx)
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return err
		}
	}
	return nil
}

// DrainOnce claims and publishes a single batch. It returns how many events
// were claimed.
func (r *Relay) DrainOnce(ctx context.Context) (int, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := store.New(tx)
	rows, err := q.ClaimOutboxBatch(ctx, r.o.BatchSize)
	if err != nil {
		return 0, err
	}
	var done []int64
	for _, row := range rows {
		e := Event{ID: row.ID, Topic: row.Topic, Payload: row.Payload, Attempts: row.Attempts}
		pub, ok := r.router.lookup(e.Topic)
		var perr error
		if !ok {
			perr = fmt.Errorf("no publisher for topic %q", e.Topic)
		} else {
			perr = pub.Publish(ctx, e)
		}
		if perr == nil {
			done = append(done, e.ID)
			continue
		}
		r.failed.Inc()
		msg := perr.Error()
		delay := retryDelaySeconds(e.Attempts)
		r.o.Logger.Error("outbox publish failed", "outbox_id", e.ID, "topic", e.Topic, "attempt", e.Attempts+1, "retry_in_s", delay, "err", perr)
		if err := q.MarkOutboxFailed(ctx, store.MarkOutboxFailedParams{ID: e.ID, LastError: &msg, RetryInSeconds: delay}); err != nil {
			return 0, err
		}
	}
	if len(done) > 0 {
		if err := q.MarkOutboxPublished(ctx, done); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	r.published.Add(float64(len(done)))
	return len(rows), nil
}

// retryDelaySeconds: 1, 2, 4 … capped at 300 s. Events are never dropped.
func retryDelaySeconds(attempts int32) float64 {
	d := 1.0
	for i := int32(0); i < attempts && d < 300; i++ {
		d *= 2
	}
	if d > 300 {
		d = 300
	}
	return d
}

// CleanupHandler deletes events published more than retention ago, in
// batches, so a large backlog never holds a long lock.
func CleanupHandler(pool *pgxpool.Pool, retention time.Duration, log *slog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		cutoff := pgtype.Timestamptz{Time: time.Now().Add(-retention).UTC(), Valid: true}
		var total int64
		for {
			n, err := store.New(pool).DeletePublishedOutboxBefore(ctx, store.DeletePublishedOutboxBeforeParams{Cutoff: cutoff, BatchSize: 5000})
			if err != nil {
				return err
			}
			total += n
			if n < 5000 {
				break
			}
		}
		log.InfoContext(ctx, "outbox cleanup finished", "deleted", total)
		return nil
	}
}
