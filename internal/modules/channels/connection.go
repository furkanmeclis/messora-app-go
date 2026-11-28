package channels

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The tenant's Meta connection (docs/PHASE-17.md §3.1, §5.8): its own
// webhook address and verify token. Webhooks are signed with the app
// secrets of the tenant's accounts; nothing is shared between tenants.

// MethodOwnApp is the company's own Meta app (WP5); MethodTechProvider is
// Messora's Tech Provider app with Embedded Signup (WP6, off).
const (
	MethodOwnApp       = "own_app"
	MethodTechProvider = "tech_provider"
)

// WebhookKeyLength is the length of a webhook key and a verify token: 32
// random bytes in lowercase base32 without padding.
const WebhookKeyLength = 52

// MaxKeyGrace bounds how long a rotated-out webhook key keeps answering.
const MaxKeyGrace = 7 * 24 * time.Hour

var keyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// newKey returns 32 random bytes as lowercase base32 (52 characters).
func newKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToLower(keyEncoding.EncodeToString(b)), nil
}

// WellFormedKey reports whether key looks like a webhook key; anything else
// is answered as unknown without a database lookup.
func WellFormedKey(key string) bool {
	if len(key) != WebhookKeyLength {
		return false
	}
	for _, c := range key {
		if (c < 'a' || c > 'z') && (c < '2' || c > '7') {
			return false
		}
	}
	return true
}

// KeyHash is what the routing table stores for a webhook key.
func KeyHash(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

func webhookKeyAD(t uuid.UUID) string  { return "meta_connection.webhook_key:" + t.String() }
func verifyTokenAD(t uuid.UUID) string { return "meta_connection.verify_token:" + t.String() }

// Connection is the tenant's Meta connection with its secrets opened.
type Connection struct {
	Row         db.MetaConnection
	WebhookKey  string
	VerifyToken string
}

// Connection returns the request's tenant's connection, creating it (with
// a fresh webhook key and verify token) the first time.
// A new workspace (sign-up or the admin panel) calls it right away
// (identity.Options.TenantOpened), so the webhook address exists before
// anyone looks at Settings.
func (s *Service) Connection(ctx context.Context) (Connection, error) {
	t, ok := tenant.FromCtx(ctx)
	if !ok {
		return Connection{}, tenant.ErrNoTenant
	}
	row, err := s.q.GetMetaConnection(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.createConnection(ctx, t)
	}
	if err != nil {
		return Connection{}, err
	}
	return s.openConnection(t, row)
}

func (s *Service) createConnection(ctx context.Context, t uuid.UUID) (db.MetaConnection, error) {
	key, err := newKey()
	if err != nil {
		return db.MetaConnection{}, err
	}
	verify, err := newKey()
	if err != nil {
		return db.MetaConnection{}, err
	}
	keyEnc, err := s.sealer.Seal([]byte(key), webhookKeyAD(t))
	if err != nil {
		return db.MetaConnection{}, err
	}
	verifyEnc, err := s.sealer.Seal([]byte(verify), verifyTokenAD(t))
	if err != nil {
		return db.MetaConnection{}, err
	}
	var row db.MetaConnection
	created := false
	err = s.rawTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		row, err = q.InsertMetaConnection(ctx, db.InsertMetaConnectionParams{WebhookKeyEnc: keyEnc, VerifyTokenEnc: verifyEnc})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // another request created it first
		}
		if err != nil {
			return err
		}
		created = true
		return routeKey(ctx, tx, key, 0)
	})
	if err != nil {
		return db.MetaConnection{}, err
	}
	if !created {
		return s.q.GetMetaConnection(ctx)
	}
	return row, nil
}

// routeKey makes key the tenant's current webhook key in the routing
// table; the previous key keeps answering for grace.
func routeKey(ctx context.Context, tx pgx.Tx, key string, grace time.Duration) error {
	_, err := tx.Exec(ctx, `SELECT meta_webhook_key_rotate($1, $2 * interval '1 second')`, KeyHash(key), int64(grace/time.Second))
	return err
}

func (s *Service) openConnection(t uuid.UUID, row db.MetaConnection) (Connection, error) {
	key, err := s.sealer.Open(row.WebhookKeyEnc, webhookKeyAD(t))
	if err != nil {
		return Connection{}, fmt.Errorf("decrypt webhook key: %w", err)
	}
	verify, err := s.sealer.Open(row.VerifyTokenEnc, verifyTokenAD(t))
	if err != nil {
		return Connection{}, fmt.Errorf("decrypt verify token: %w", err)
	}
	return Connection{Row: row, WebhookKey: string(key), VerifyToken: string(verify)}, nil
}

// RotateWebhookKey gives the tenant a new webhook address. The previous
// one keeps working for grace (0 ends it at once), long enough to update
// the callback URL in the Meta app.
func (s *Service) RotateWebhookKey(ctx context.Context, grace time.Duration) (Connection, error) {
	if grace < 0 || grace > MaxKeyGrace {
		return Connection{}, apperr.Validation("grace_hours", "The grace period is 0 to 168 hours.")
	}
	t, ok := tenant.FromCtx(ctx)
	if !ok {
		return Connection{}, tenant.ErrNoTenant
	}
	if _, err := s.Connection(ctx); err != nil {
		return Connection{}, err
	}
	key, err := newKey()
	if err != nil {
		return Connection{}, err
	}
	enc, err := s.sealer.Seal([]byte(key), webhookKeyAD(t))
	if err != nil {
		return Connection{}, err
	}
	var row db.MetaConnection
	err = s.rawTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := routeKey(ctx, tx, key, grace); err != nil {
			return err
		}
		var until *time.Time
		if grace > 0 {
			u := time.Now().Add(grace)
			until = &u
		}
		if row, err = q.SetMetaWebhookKey(ctx, db.SetMetaWebhookKeyParams{WebhookKeyEnc: enc, PreviousKeyExpiresAt: until}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.webhook_key_rotate", EntityType: "meta_connection",
			Changes: map[string]any{"grace_hours": int(grace / time.Hour)}})
	})
	if err != nil {
		return Connection{}, err
	}
	return s.openConnection(t, row)
}

// RotateVerifyToken replaces the verify token. Meta asks for it only when
// the callback URL is (re)verified, so the old one ends at once.
func (s *Service) RotateVerifyToken(ctx context.Context) (Connection, error) {
	t, ok := tenant.FromCtx(ctx)
	if !ok {
		return Connection{}, tenant.ErrNoTenant
	}
	if _, err := s.Connection(ctx); err != nil {
		return Connection{}, err
	}
	verify, err := newKey()
	if err != nil {
		return Connection{}, err
	}
	enc, err := s.sealer.Seal([]byte(verify), verifyTokenAD(t))
	if err != nil {
		return Connection{}, err
	}
	var row db.MetaConnection
	err = s.tx(ctx, func(q *db.Queries) error {
		if row, err = q.SetMetaVerifyToken(ctx, enc); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.verify_token_rotate", EntityType: "meta_connection"})
	})
	if err != nil {
		return Connection{}, err
	}
	return s.openConnection(t, row)
}

// VerifyToken returns the verify token of the context's tenant, "" before
// its connection exists (the handshake then fails). There is no
// platform-wide token.
func (s *Service) VerifyToken(ctx context.Context) (string, error) {
	t, ok := tenant.FromCtx(ctx)
	if !ok {
		return "", tenant.ErrNoTenant
	}
	row, err := s.q.GetMetaConnection(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	b, err := s.sealer.Open(row.VerifyTokenEnc, verifyTokenAD(t))
	if err != nil {
		return "", fmt.Errorf("decrypt verify token: %w", err)
	}
	return string(b), nil
}

// RecordSignatureFailure counts a webhook with a missing or wrong
// signature for the health panel.
func (s *Service) RecordSignatureFailure(ctx context.Context) {
	if err := s.q.RecordMetaSignatureFailure(ctx); err != nil {
		s.log.WarnContext(ctx, "record webhook signature failure", "error", err)
	}
}

// KeyRoute is the tenant a webhook key routes to.
type KeyRoute struct {
	TenantID uuid.UUID
	Status   string // tenant status
}

// WebhookKeyTenant resolves a webhook key to its tenant before any tenant
// is known. It reads the platform routing table by the key's SHA-256 (the
// lookup compares hashes, so its timing says nothing about the key) on the
// application pool without a tenant. Expired and unknown keys are not
// found.
func (s *Service) WebhookKeyTenant(ctx context.Context, key string) (KeyRoute, bool, error) {
	if !WellFormedKey(key) {
		return KeyRoute{}, false, nil
	}
	var r KeyRoute
	err := s.pool.QueryRow(tenant.System(ctx), `
		SELECT k.tenant_id, t.status FROM meta_webhook_keys k JOIN tenants t ON t.id = k.tenant_id
		WHERE k.key_sha256 = $1 AND (k.expires_at IS NULL OR k.expires_at > now())`, KeyHash(key)).Scan(&r.TenantID, &r.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return KeyRoute{}, false, nil
	}
	if err != nil {
		return KeyRoute{}, false, err
	}
	return r, true, nil
}

func (s *Service) rawTx(ctx context.Context, fn func(pgx.Tx, *db.Queries) error) error {
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
