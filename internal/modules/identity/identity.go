// Package identity covers authentication, sessions, two-factor
// authentication, users, memberships, invitations, roles, API keys, self
// sign-up, workspace hand-over and the platform admins' own sign-in. These
// share tables and invariants (e.g. "never remove the last owner"), so they
// live in one module.
//
// Accounts are global (one person, one e-mail address, one password and
// second factor); a membership gives them a role in each company (tenant).
// Sessions and API keys belong to one tenant. Platform admins sign in to the
// admin host with their own sessions.
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
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Token prefixes make leaked credentials recognisable (and scannable).
const (
	SessionTokenPrefix  = "mss_"
	PlatformTokenPrefix = "msp_"
	APIKeyPrefix        = "msk_live_"
	inviteTokenPrefix   = "msi_"
	resetTokenPrefix    = "msr_"
	mfaTokenPrefix      = "msm_"
	verifyTokenPrefix   = "msv_"
	handoffTokenPrefix  = "msh_"
	emailTokenPrefix    = "mse_"
	bootstrapPrefix     = "msb_"
)

// Policy constants.
const (
	ReauthWindow         = 10 * time.Minute
	maxLoginFailures     = 10
	lockMinutes          = 15
	mfaChallengeTTL      = 5 * time.Minute
	mfaMaxAttempts       = 5
	invitationTTL        = 7 * 24 * time.Hour
	passwordResetTTL     = 30 * time.Minute
	verificationTTL      = 24 * time.Hour
	handoffTTL           = 60 * time.Second
	emailChangeTTL       = 24 * time.Hour
	bootstrapTokenTTL    = 24 * time.Hour
	maxResetsPerHour     = 3
	sessionTouchPeriod   = time.Minute
	platformSessionTTL   = 12 * time.Hour
	platformSessionIdle  = 2 * time.Hour
	trialDays            = 14
	resendVerifyInterval = time.Minute
)

// Options configures the service.
type Options struct {
	PublicAppURL string
	// Hosts builds tenant and platform links (scheme and port of
	// PUBLIC_APP_URL, the base domain).
	Hosts  tenant.Hosts
	Argon2 crypto.Argon2Params
	Logger *slog.Logger
	// Mail renders the branded e-mails (templates.Default when nil; links
	// may be http only when PUBLIC_APP_URL is).
	Mail *templates.Renderer
	// InvalidateHosts drops cached host resolutions after a workspace
	// was opened (tenant.Resolver.Invalidate).
	InvalidateHosts func(hosts ...string)
}

// Service implements identity use cases.
type Service struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	rdb      goredis.UniversalClient
	sealer   *crypto.Sealer
	settings *settings.Store
	mail     mailer.Sender
	tpl      *templates.Renderer
	limiter  *ratelimit.Limiter
	o        Options
	now      func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, rdb goredis.UniversalClient, sealer *crypto.Sealer, st *settings.Store, mail mailer.Sender, o Options) *Service {
	if o.Argon2.MemoryKiB == 0 {
		o.Argon2 = crypto.DefaultArgon2
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Hosts == (tenant.Hosts{}) {
		o.Hosts = tenant.NewHosts(o.PublicAppURL, "")
	}
	tpl := o.Mail
	if tpl == nil {
		var err error
		if tpl, err = templates.Default(); err != nil {
			panic(err) // the embedded templates are tested; this cannot fail at run time
		}
	}
	if !o.Hosts.Secure() {
		tpl = tpl.AllowHTTP()
	}
	return &Service{
		pool: pool, q: db.New(pool), rdb: rdb, sealer: sealer, settings: st, mail: mail, tpl: tpl,
		limiter: ratelimit.New(rdb, "messora:rl"), o: o, now: time.Now,
	}
}

// Hosts returns the platform addresses the service builds links with.
func (s *Service) Hosts() tenant.Hosts { return s.o.Hosts }

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

// platformTx runs fn as platform work (messora_platform): across tenants,
// on the registry and the platform tables. ctx's tenant, if any, is
// dropped for the transaction only.
func (s *Service) platformTx(ctx context.Context, fn func(q *db.Queries) error) error {
	return s.tx(tenant.Platform(ctx), fn)
}

// PlatformTx is platformTx for other modules (the platform admin panel).
func (s *Service) PlatformTx(ctx context.Context, fn func(q *db.Queries) error) error {
	return s.platformTx(ctx, fn)
}

// userView loads a member with their role in the connection's tenant as
// the API representation.
func (s *Service) userView(ctx context.Context, q *db.Queries, u db.User) (api.User, error) {
	role, err := q.GetUserRole(ctx, u.ID)
	if err != nil {
		return api.User{}, err
	}
	v := toAPIUser(u, role.RoleKey, role.RoleName)
	if role.MembershipStatus != "active" {
		v.Status = api.UserStatusDisabled
	}
	return v, nil
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

// Cleanup deletes long-expired sessions and stale idempotency records of
// ctx's tenant.
func (s *Service) Cleanup(ctx context.Context) (sessions, idempotency int64, err error) {
	if sessions, err = s.q.DeleteExpiredSessions(ctx); err != nil {
		return
	}
	idempotency, err = s.q.DeleteOldIdempotencyKeys(ctx)
	return
}

// CleanupPlatform deletes long-expired platform sessions.
func (s *Service) CleanupPlatform(ctx context.Context) (int64, error) {
	var n int64
	err := s.platformTx(ctx, func(q *db.Queries) error {
		var err error
		n, err = q.DeleteExpiredPlatformSessions(ctx)
		return err
	})
	return n, err
}
