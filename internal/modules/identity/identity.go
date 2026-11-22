// Package identity covers authentication, sessions, two-factor
// authentication, users, invitations, roles and API keys. These share tables
// and invariants (e.g. "never remove the last owner"), so they live in one
// module.
//
// Services return types from the generated internal/api package: the
// OpenAPI contract is the single source of truth for shapes.
package identity

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
)

// Token prefixes make leaked credentials recognisable (and scannable).
const (
	SessionTokenPrefix = "mss_"
	APIKeyPrefix       = "msk_live_"
	inviteTokenPrefix  = "msi_"
	resetTokenPrefix   = "msr_"
	setupTokenPrefix   = "mst_"
	mfaTokenPrefix     = "msm_"
)

// Policy constants.
const (
	ReauthWindow       = 10 * time.Minute
	maxLoginFailures   = 10
	lockMinutes        = 15
	mfaChallengeTTL    = 5 * time.Minute
	mfaMaxAttempts     = 5
	invitationTTL      = 7 * 24 * time.Hour
	passwordResetTTL   = 30 * time.Minute
	setupTokenTTL      = 24 * time.Hour
	maxResetsPerHour   = 3
	sessionTouchPeriod = time.Minute
)

// Options configures the service.
type Options struct {
	PublicAppURL string
	Argon2       crypto.Argon2Params
	Logger       *slog.Logger
}

// Service implements identity use cases.
type Service struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	rdb      goredis.UniversalClient
	sealer   *crypto.Sealer
	settings *settings.Store
	mail     mailer.Sender
	limiter  *ratelimit.Limiter
	o        Options
	now      func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, rdb goredis.UniversalClient, sealer *crypto.Sealer, st *settings.Store, mail mailer.Sender, o Options) *Service {
	if o.Argon2.MemoryKiB == 0 {
		o.Argon2 = crypto.DefaultArgon2
	}
	return &Service{
		pool: pool, q: db.New(pool), rdb: rdb, sealer: sealer, settings: st, mail: mail,
		limiter: ratelimit.New(rdb, "messora:rl"), o: o, now: time.Now,
	}
}

// tx runs fn in a transaction with typed queries.
func (s *Service) tx(ctx context.Context, fn func(q *db.Queries) error) error {
	t, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = t.Rollback(ctx) }()
	if err := fn(s.q.WithTx(t)); err != nil {
		return err
	}
	return t.Commit(ctx)
}

// userView loads a user with its role as the API representation.
func (s *Service) userView(ctx context.Context, q *db.Queries, u db.User) (api.User, error) {
	role, err := q.GetUserRole(ctx, u.ID)
	if err != nil {
		return api.User{}, err
	}
	return toAPIUser(u, role.RoleKey, role.RoleName), nil
}

func toAPIUser(u db.User, roleKey, roleName string) api.User {
	return api.User{
		Id:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		Status:      api.UserStatus(u.Status),
		Role:        api.RoleRef{Key: api.RoleKey(roleKey), Name: roleName},
		MfaEnabled:  u.TotpConfirmedAt != nil,
		Locale:      u.Locale,
		Timezone:    u.Timezone,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }

// Cleanup deletes long-expired sessions and stale idempotency records.
func (s *Service) Cleanup(ctx context.Context) (sessions, idempotency int64, err error) {
	if sessions, err = s.q.DeleteExpiredSessions(ctx); err != nil {
		return
	}
	idempotency, err = s.q.DeleteOldIdempotencyKeys(ctx)
	return
}
