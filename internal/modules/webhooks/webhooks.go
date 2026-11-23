// Package webhooks delivers domain events to receivers that integrators
// register (outgoing webhooks).
//
// Flow: a module calls one of the emitters (MessageReceived, …) inside its
// own transaction. If an active endpoint subscribes to the type, the event is
// stored with an outbox task webhook:fanout, so it commits or rolls back with
// the change it describes. Fan-out creates one delivery per (event,
// endpoint), protected by a unique constraint, and schedules webhook:deliver
// for each. Delivery signs the body, sends it through the SSRF-safe client
// and records the outcome; retries are scheduled through the outbox with a
// delay, so the retry plan lives in PostgreSQL and survives a Redis loss.
package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/safehttp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Task types.
const (
	TypeFanOut  = "webhook:fanout"
	TypeDeliver = "webhook:deliver"
	TypeCleanup = "webhook:cleanup"
)

// Limits.
const (
	maxEndpoints     = 50
	requestTimeout   = 10 * time.Second
	leaseSeconds     = 60 // > requestTimeout: a vanished worker's delivery is taken over after this
	storedBodyBytes  = 1024
	responseBodyRead = 16 << 10
	userAgent        = "Messora-Webhooks/1"
	// Retention is how long events and their delivery log are kept.
	Retention = 30 * 24 * time.Hour
)

// Options configures the service.
type Options struct {
	// AllowHTTP accepts http:// endpoint URLs (development only).
	AllowHTTP bool
	// Client replaces the HTTP client (tests).
	Client *safehttp.Client
	Logger *slog.Logger
}

// Service manages endpoints and delivers events.
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	sealer *crypto.Sealer
	client *safehttp.Client
	log    *slog.Logger
}

// New returns the service. The HTTP client refuses private destinations; no
// option here can change that (see safehttp.Options.AllowPrivate).
func New(pool *pgxpool.Pool, sealer *crypto.Sealer, o Options) *Service {
	c := o.Client
	if c == nil {
		c = NewClient(o.AllowHTTP)
	}
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{pool: pool, q: db.New(pool), sealer: sealer, client: c, log: log}
}

// NewClient returns the production delivery client.
func NewClient(allowHTTP bool) *safehttp.Client {
	return safehttp.New(safehttp.Options{Timeout: requestTimeout, MaxResponseBytes: responseBodyRead, AllowHTTP: allowHTTP})
}

// SetClient replaces the HTTP client (tests).
func (s *Service) SetClient(c *safehttp.Client) { s.client = c }

func secretAD(id uuid.UUID) string { return "webhook_secret:" + id.String() }

// EndpointInput holds create/update fields; nil means unchanged.
type EndpointInput struct {
	URL         *string
	Description *string
	EventTypes  *[]string
	Active      *bool
}

func (s *Service) checkURL(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 {
		return "", apperr.Validation("url", "Enter a valid URL.")
	}
	if u.User != nil {
		return "", apperr.Validation("url", "Credentials in the URL are not supported; verify requests with the signature instead.")
	}
	if u.Fragment != "" {
		return "", apperr.Validation("url", "The URL must not contain a fragment (#…).")
	}
	if err := s.client.Check(ctx, raw); err != nil {
		var b *safehttp.BlockedError
		if errors.As(err, &b) {
			return "", apperr.Validation("url", "This destination is not allowed: "+b.Reason+".")
		}
		return "", apperr.Validation("url", "The host name could not be resolved.")
	}
	return raw, nil
}

func eventTypes(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, apperr.Validation("event_types", "Choose at least one event.")
	}
	out := make([]string, 0, len(in))
	for _, t := range in {
		if !Known(t) {
			return nil, apperr.Validation("event_types", fmt.Sprintf("Unknown event type %q.", t))
		}
		out = append(out, t)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

func newSecret() (string, error) { return crypto.NewToken("whsec_") }

func last4(s string) string { return s[len(s)-4:] }

// Create registers an endpoint and returns it with its signing secret.
func (s *Service) Create(ctx context.Context, in EndpointInput, actor *uuid.UUID) (db.WebhookEndpoint, string, error) {
	if in.URL == nil || in.EventTypes == nil {
		return db.WebhookEndpoint{}, "", apperr.Validation("", "url and event_types are required.")
	}
	u, err := s.checkURL(ctx, *in.URL)
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	types, err := eventTypes(*in.EventTypes)
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	desc := ""
	if in.Description != nil {
		desc = strings.TrimSpace(*in.Description)
	}
	active := in.Active == nil || *in.Active
	id, err := uuid.NewV7()
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	secret, err := newSecret()
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	sealed, err := s.sealer.Seal([]byte(secret), secretAD(id))
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	var ep db.WebhookEndpoint
	err = s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		n, err := q.CountWebhookEndpoints(ctx)
		if err != nil {
			return err
		}
		if n >= maxEndpoints {
			return apperr.Conflict("WEBHOOK_LIMIT", fmt.Sprintf("At most %d webhook endpoints can be registered.", maxEndpoints))
		}
		ep, err = q.InsertWebhookEndpoint(ctx, db.InsertWebhookEndpointParams{ID: id, Url: u, Description: desc, EventTypes: types,
			Active: active, SecretEnc: sealed, SecretLast4: last4(secret), CreatedBy: actor})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "webhook.create", EntityType: "webhook_endpoint", EntityID: &id,
			Changes: map[string]any{"url": redactURL(u), "event_types": types, "active": active}})
	})
	return ep, secret, err
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return safehttp.Redact(u)
}

// Get returns an endpoint.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.WebhookEndpoint, error) {
	ep, err := s.q.GetWebhookEndpoint(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ep, apperr.NotFound("Webhook endpoint not found.")
	}
	return ep, err
}

// List returns every endpoint, newest first.
func (s *Service) List(ctx context.Context) ([]db.WebhookEndpoint, error) {
	return s.q.ListWebhookEndpoints(ctx)
}

// Update changes an endpoint. Re-activating clears the disabled reason.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in EndpointInput) (db.WebhookEndpoint, error) {
	p := db.UpdateWebhookEndpointParams{ID: id, Active: in.Active}
	changes := map[string]any{}
	if in.URL != nil {
		u, err := s.checkURL(ctx, *in.URL)
		if err != nil {
			return db.WebhookEndpoint{}, err
		}
		p.Url, changes["url"] = &u, redactURL(u)
	}
	if in.EventTypes != nil {
		types, err := eventTypes(*in.EventTypes)
		if err != nil {
			return db.WebhookEndpoint{}, err
		}
		p.EventTypes, changes["event_types"] = types, types
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		p.Description, changes["description"] = &d, d
	}
	if in.Active != nil {
		changes["active"] = *in.Active
	}
	var ep db.WebhookEndpoint
	err := s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		var err error
		ep, err = q.UpdateWebhookEndpoint(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Webhook endpoint not found.")
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "webhook.update", EntityType: "webhook_endpoint", EntityID: &id, Changes: changes})
	})
	return ep, err
}

// Delete removes an endpoint with its delivery log.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		n, err := q.DeleteWebhookEndpoint(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound("Webhook endpoint not found.")
		}
		return audit.Record(ctx, q, audit.Entry{Action: "webhook.delete", EntityType: "webhook_endpoint", EntityID: &id})
	})
}

// RotateSecret replaces the signing secret and returns the new one.
func (s *Service) RotateSecret(ctx context.Context, id uuid.UUID) (db.WebhookEndpoint, string, error) {
	secret, err := newSecret()
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	sealed, err := s.sealer.Seal([]byte(secret), secretAD(id))
	if err != nil {
		return db.WebhookEndpoint{}, "", err
	}
	var ep db.WebhookEndpoint
	err = s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		var err error
		ep, err = q.SetWebhookEndpointSecret(ctx, db.SetWebhookEndpointSecretParams{ID: id, SecretEnc: sealed, SecretLast4: last4(secret)})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Webhook endpoint not found.")
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "webhook.rotate_secret", EntityType: "webhook_endpoint", EntityID: &id})
	})
	return ep, secret, err
}

// TestData is the payload of webhook.test.
type TestData struct {
	EndpointID uuid.UUID `json:"endpoint_id"`
	Message    string    `json:"message"`
}

// SendTest queues a webhook.test event for one endpoint, active or not.
func (s *Service) SendTest(ctx context.Context, id uuid.UUID) (db.WebhookDelivery, error) {
	var d db.WebhookDelivery
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := q.GetWebhookEndpoint(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Webhook endpoint not found.")
		} else if err != nil {
			return err
		}
		eventID, err := insertEvent(ctx, q, TypeTest, TestData{EndpointID: id, Message: "This is a test event from Messora."})
		if err != nil {
			return err
		}
		d, err = q.InsertWebhookDelivery(ctx, db.InsertWebhookDeliveryParams{EventID: eventID, EndpointID: id})
		if err != nil {
			return err
		}
		return scheduleDelivery(ctx, tx, d.ID, 0)
	})
	return d, err
}

// insertEvent stores an envelope without fan-out.
func insertEvent(ctx context.Context, q *db.Queries, typ string, data any) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	b, err := json.Marshal(Envelope{ID: id, Type: typ, Version: EventVersion, CreatedAt: time.Now().UTC().Truncate(time.Millisecond), Data: data})
	if err != nil {
		return uuid.Nil, err
	}
	return id, q.InsertOutgoingWebhookEvent(ctx, db.InsertOutgoingWebhookEventParams{ID: id, Type: typ, Payload: b})
}

func scheduleDelivery(ctx context.Context, tx pgx.Tx, id uuid.UUID, delay time.Duration) error {
	_, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeDeliver), map[string]string{"delivery_id": id.String()}, delay)
	return err
}

// Redeliver sends a delivery again with a fresh attempt budget.
func (s *Service) Redeliver(ctx context.Context, id uuid.UUID) (db.WebhookDelivery, error) {
	var d db.WebhookDelivery
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		var err error
		d, err = q.ResetWebhookDelivery(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, gerr := q.GetWebhookDelivery(ctx, id); errors.Is(gerr, pgx.ErrNoRows) {
				return apperr.NotFound("Delivery not found.")
			}
			return apperr.Conflict("DELIVERY_IN_PROGRESS", "This delivery is being sent right now.")
		}
		if err != nil {
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: "webhook.redeliver", EntityType: "webhook_endpoint", EntityID: &d.EndpointID,
			Changes: map[string]any{"delivery_id": d.ID}}); err != nil {
			return err
		}
		return scheduleDelivery(ctx, tx, d.ID, 0)
	})
	return d, err
}

// Deliveries lists an endpoint's delivery log.
func (s *Service) Deliveries(ctx context.Context, endpoint uuid.UUID, status *string, before *uuid.UUID, limit int32) ([]db.ListWebhookDeliveriesRow, error) {
	if _, err := s.Get(ctx, endpoint); err != nil {
		return nil, err
	}
	return s.q.ListWebhookDeliveries(ctx, db.ListWebhookDeliveriesParams{EndpointID: endpoint, Status: status, BeforeID: before, Lim: limit})
}

// FanOut creates a delivery for every active endpoint subscribed to the
// event. It is idempotent: a second run for the same event creates nothing.
func (s *Service) FanOut(ctx context.Context, eventID uuid.UUID) (int, error) {
	var n int
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		ev, err := q.GetOutgoingWebhookEvent(ctx, eventID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // removed by retention
		}
		if err != nil {
			return err
		}
		ids, err := q.FanOutWebhookEvent(ctx, db.FanOutWebhookEventParams{EventID: ev.ID, EventType: ev.Type})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := scheduleDelivery(ctx, tx, id, 0); err != nil {
				return err
			}
		}
		n = len(ids)
		return nil
	})
	return n, err
}

// outcome is what a delivery attempt resulted in.
type outcome struct {
	status   string // succeeded, retrying, failed
	code     *int32
	duration *int32
	errMsg   *string
	body     *string
	delay    time.Duration // before the next try (retrying)
	disable  string        // reason to switch the endpoint off
}

// Deliver performs one attempt of a delivery. It returns nil when the
// attempt was recorded (whatever its result); a *queue.RetryAfterError when
// another worker holds the delivery; other errors when the outcome could not
// be stored.
func (s *Service) Deliver(ctx context.Context, id uuid.UUID) error {
	d, err := s.q.ClaimWebhookDelivery(ctx, db.ClaimWebhookDeliveryParams{ID: id, LeaseSeconds: leaseSeconds})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, gerr := s.q.GetWebhookDelivery(ctx, id)
		if errors.Is(gerr, pgx.ErrNoRows) {
			return nil // endpoint deleted
		}
		if gerr != nil {
			return gerr
		}
		if cur.Status == "delivering" && cur.LockedUntil != nil {
			// A worker that died mid-request is taken over once its lease expires.
			return &queue.RetryAfterError{After: time.Until(*cur.LockedUntil) + time.Second, Reason: "delivery leased by another worker"}
		}
		return nil // finished, or not due (a newer task is scheduled)
	}
	if err != nil {
		return err
	}
	o, err := s.attempt(ctx, d)
	if err != nil {
		return err
	}
	return s.finish(context.WithoutCancel(ctx), d, o)
}

func (s *Service) attempt(ctx context.Context, d db.WebhookDelivery) (outcome, error) {
	ep, err := s.q.GetWebhookEndpoint(ctx, d.EndpointID)
	if errors.Is(err, pgx.ErrNoRows) {
		return outcome{status: "failed", errMsg: ptr("endpoint deleted")}, nil
	}
	if err != nil {
		return outcome{}, err
	}
	ev, err := s.q.GetOutgoingWebhookEvent(ctx, d.EventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return outcome{status: "failed", errMsg: ptr("event no longer stored")}, nil
	}
	if err != nil {
		return outcome{}, err
	}
	if !ep.Active && ev.Type != TypeTest {
		return outcome{status: "failed", errMsg: ptr("endpoint is disabled")}, nil
	}
	secret, err := s.sealer.Open(ep.SecretEnc, secretAD(ep.ID))
	if err != nil {
		return outcome{status: "failed", errMsg: ptr("signing secret cannot be decrypted (was ENCRYPTION_KEY changed?)")}, nil
	}
	var body bytes.Buffer
	if err := json.Compact(&body, ev.Payload); err != nil {
		return outcome{}, err
	}
	rctx, cancel := context.WithTimeout(ctx, requestTimeout+2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, ep.Url, bytes.NewReader(body.Bytes()))
	if err != nil {
		return outcome{status: "failed", errMsg: ptr("invalid URL")}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(HeaderSignature, Sign(string(secret), time.Now(), body.Bytes()))
	req.Header.Set(HeaderEventID, ev.ID.String())
	req.Header.Set(HeaderEventType, ev.Type)
	req.Header.Set(HeaderDelivery, d.ID.String())
	req.Header.Set(HeaderAttempt, strconv.Itoa(int(d.Attempts)))
	res, err := s.client.Do(req)
	return classify(res, err, int(d.Attempts), time.Now()), nil
}

// classify applies the retry policy (docs/WEBHOOKS.md):
//   - 2xx: succeeded.
//   - 410 Gone: failed, and the endpoint is switched off.
//   - network errors, timeouts, 408, 429 and 5xx: retried with backoff
//     (Retry-After is honoured on 429 and 503) until MaxAttempts.
//   - a refused destination (SSRF protection), 3xx and other 4xx: failed
//     at once; the receiver must be fixed and the delivery resent.
func classify(res *safehttp.Response, err error, attempt int, now time.Time) outcome {
	var o outcome
	if res != nil {
		code, ms := int32(res.StatusCode), int32(min(res.Duration.Milliseconds(), 1<<31-1)) //nolint:gosec // bounded above
		o.code, o.duration = &code, &ms
		b := res.Body
		if len(b) > storedBodyBytes {
			b = b[:storedBodyBytes]
		}
		if len(b) > 0 {
			o.body = ptr(strings.ToValidUTF8(string(b), string(utf8.RuneError)))
		}
	}
	retry := func(msg string, d time.Duration) outcome {
		if attempt >= MaxAttempts {
			o.status, o.errMsg = "failed", ptr(fmt.Sprintf("gave up after %d attempts: %s", attempt, msg))
			return o
		}
		o.status, o.errMsg, o.delay = "retrying", ptr(msg), d
		return o
	}
	switch {
	case err != nil && safehttp.IsBlocked(err):
		o.status, o.errMsg = "failed", ptr(truncate(err.Error(), 500))
		return o
	case err != nil:
		return retry(truncate(err.Error(), 500), Backoff(attempt))
	}
	switch c := res.StatusCode; {
	case c >= 200 && c < 300:
		o.status = "succeeded"
	case c == http.StatusGone:
		o.status, o.errMsg, o.disable = "failed", ptr("receiver answered 410 Gone"), "The receiver answered 410 Gone."
	case c == http.StatusTooManyRequests || c == http.StatusServiceUnavailable:
		d := Backoff(attempt)
		if ra, ok := retryAfter(res.Header, now); ok {
			d = ra
		}
		return retry(fmt.Sprintf("HTTP %d", c), d)
	case c == http.StatusRequestTimeout || c >= 500:
		return retry(fmt.Sprintf("HTTP %d", c), Backoff(attempt))
	case c >= 300 && c < 400:
		o.status, o.errMsg = "failed", ptr(fmt.Sprintf("HTTP %d: redirects are not followed; register the final URL", c))
	default:
		o.status, o.errMsg = "failed", ptr(fmt.Sprintf("HTTP %d", c))
	}
	return o
}

func (s *Service) finish(ctx context.Context, d db.WebhookDelivery, o outcome) error {
	return s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		p := db.FinishWebhookDeliveryParams{ID: d.ID, Status: o.status, LastStatusCode: o.code, LastDurationMs: o.duration,
			LastError: o.errMsg, LastResponseBody: o.body}
		if o.status == "retrying" {
			next := time.Now().Add(o.delay)
			p.NextAttemptAt = &next
		}
		n, err := q.FinishWebhookDelivery(ctx, p)
		if err != nil || n == 0 {
			return err // n == 0: deleted or reset meanwhile
		}
		if o.status == "retrying" {
			if err := scheduleDelivery(ctx, tx, d.ID, o.delay); err != nil {
				return err
			}
		}
		if o.disable != "" {
			if err := q.DisableWebhookEndpoint(ctx, db.DisableWebhookEndpointParams{ID: d.EndpointID, Reason: &o.disable}); err != nil {
				return err
			}
			s.log.WarnContext(ctx, "webhook endpoint disabled", "endpoint_id", d.EndpointID, "reason", o.disable)
			return audit.Record(ctx, q, audit.Entry{Action: "webhook.disable", EntityType: "webhook_endpoint", EntityID: &d.EndpointID,
				Changes: map[string]any{"reason": o.disable}})
		}
		return nil
	})
}

// Cleanup deletes events (and their deliveries) older than retention.
func (s *Service) Cleanup(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	var total int64
	for {
		n, err := s.q.DeleteOldOutgoingWebhookEvents(ctx, db.DeleteOldOutgoingWebhookEventsParams{Cutoff: cutoff, BatchSize: 5000})
		if err != nil {
			return total, err
		}
		total += n
		if n < 5000 {
			return total, nil
		}
	}
}

func (s *Service) tx(ctx context.Context, fn func(pgx.Tx, *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ptr[T any](v T) *T { return &v }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
