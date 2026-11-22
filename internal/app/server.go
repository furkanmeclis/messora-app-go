package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/modules/system"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Aliases give each embedded module handler a distinct field name.
type (
	identityHandler = identity.Handler
	settingsHandler = settings.Handler
	auditHandler    = audit.Handler
	systemHandler   = system.Handler
)

// apiServer composes every module handler into api.ServerInterface; the
// compiler guarantees each spec operation has an implementation.
type apiServer struct {
	*identityHandler
	*settingsHandler
	*auditHandler
	*systemHandler
}

var _ api.ServerInterface = apiServer{}

// mfaEnrolmentOps are reachable while a mandatory second factor is not set up.
var mfaEnrolmentOps = []string{"getMe", "logout", "reauthenticate", "startTotpSetup", "confirmTotp", "getAuthCapabilities"}

// services holds what the API and CLI commands need.
type services struct {
	identity *identity.Service
	settings *settings.Store
	queries  *db.Queries
}

func newServices(cfg *config.Config, pool *pgxpool.Pool, rdb goredis.UniversalClient, log *slog.Logger) (*services, error) {
	mail, err := mailer.New(mailer.Config{
		Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password.Reveal(), From: cfg.SMTP.From, TLS: cfg.SMTP.TLS,
	})
	if err != nil {
		return nil, err
	}
	return newServicesWith(cfg, pool, rdb, log, mail, crypto.DefaultArgon2)
}

// newServicesWith lets tests inject a mailer and cheaper hashing parameters.
func newServicesWith(cfg *config.Config, pool *pgxpool.Pool, rdb goredis.UniversalClient, log *slog.Logger, mail mailer.Sender, argon crypto.Argon2Params) (*services, error) {
	sealer, err := crypto.NewSealer(cfg.EncryptionKey, nil)
	if err != nil {
		return nil, err
	}
	q := db.New(pool)
	st := settings.NewStore(q, 15*time.Second)
	idsvc := identity.New(pool, rdb, sealer, st, mail, identity.Options{PublicAppURL: cfg.PublicAppURL, Logger: log, Argon2: argon})
	return &services{identity: idsvc, settings: st, queries: q}, nil
}

// mountAPI registers the guarded API under /api/v1 on r.
func mountAPI(r chi.Router, svc *services, pool *pgxpool.Pool, sys *system.Handler, rdb goredis.UniversalClient, log *slog.Logger) error {
	spec, err := api.GetSpec()
	if err != nil {
		return fmt.Errorf("load embedded spec: %w", err)
	}
	guard, err := apiguard.New(spec, apiguard.Options{
		Auth:            svc.identity,
		Limiter:         ratelimit.New(rdb, "messora:rl"),
		Queries:         svc.queries,
		Logger:          log,
		MFAEnrolmentOps: mfaEnrolmentOps,
		SessionLimit:    ratelimit.PerMinute(600),
		APIKeyLimit:     ratelimit.PerMinute(300),
		AnonymousLimit:  ratelimit.PerMinute(120),
	})
	if err != nil {
		return err
	}
	srv := apiServer{
		identityHandler: identity.NewHandler(svc.identity, log),
		settingsHandler: settings.NewHandler(pool, svc.settings, log),
		auditHandler:    audit.NewHandler(svc.queries, log),
		systemHandler:   sys,
	}
	r.Group(func(g chi.Router) {
		g.Use(httpx.MaxBody(1<<20), httpx.ClientMeta, guard.Middleware)
		api.HandlerWithOptions(srv, api.ChiServerOptions{
			BaseURL:    "/api/v1",
			BaseRouter: g,
			// Malformed path or query parameters the guard did not catch.
			ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				httpx.WriteError(w, r, apperr.Validation("", err.Error()), log)
			},
		})
	})
	return nil
}

func newSystemHandler(pool *pgxpool.Pool, rdb goredis.UniversalClient, insp *asynq.Inspector, cf *centrifugo.Client) *system.Handler {
	return system.NewHandler(Version, pool, rdb, insp, cf)
}
