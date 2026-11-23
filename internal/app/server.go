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
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/conversations"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/modules/system"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
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
	identityHandler   = identity.Handler
	settingsHandler   = settings.Handler
	auditHandler      = audit.Handler
	systemHandler     = system.Handler
	channelsHandler   = channels.Handler
	messagesHandler   = messaging.Handler
	contactsHandler   = contacts.Handler
	campaignHandler   = campaigns.Handler
	realtimeHandler   = realtime.Handler
	inboxHandler      = conversations.Handler
	webhooksHandler   = webhooks.Handler
	automationHandler = automation.Handler
)

// apiServer composes every module handler into api.ServerInterface; the
// compiler guarantees each spec operation has an implementation.
type apiServer struct {
	*identityHandler
	*settingsHandler
	*auditHandler
	*systemHandler
	*channelsHandler
	*messagesHandler
	*contactsHandler
	*campaignHandler
	*realtimeHandler
	*inboxHandler
	*webhooksHandler
	*automationHandler
}

var _ api.ServerInterface = apiServer{}

// mfaEnrolmentOps are reachable while a mandatory second factor is not set up.
var mfaEnrolmentOps = []string{"getMe", "logout", "reauthenticate", "startTotpSetup", "confirmTotp", "getAuthCapabilities"}

// services holds what the API and CLI commands need.
type services struct {
	cfg        *config.Config
	identity   *identity.Service
	settings   *settings.Store
	queries    *db.Queries
	channels   *channels.Service
	messaging  *messaging.Service
	contacts   *contacts.Service
	campaigns  *campaigns.Service
	inbox      *conversations.Service
	webhooks   *webhooks.Service
	automation *automation.Service
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
	wa, err := whatsapp.New(whatsapp.Options{BaseURL: cfg.Meta.GraphBaseURL, Version: cfg.Meta.GraphVersion, DryRun: cfg.Meta.DryRun})
	if err != nil {
		return nil, err
	}
	ch := channels.New(pool, sealer, wa, cfg.Meta.AppSecret.Reveal(), log)
	media := messaging.NewMediaStore(cfg.MediaDir)
	msg := messaging.New(pool, ch, media, log)
	cts := contacts.New(pool, cfg.MediaDir, log)
	camp := campaigns.New(pool, cts, log)
	inbox := conversations.New(pool, msg, media, log)
	// Outgoing webhooks: https only outside development; private
	// destinations are always refused (internal/platform/safehttp).
	hooks := webhooks.New(pool, sealer, webhooks.Options{AllowHTTP: cfg.Env == config.EnvDevelopment, Logger: log})
	return &services{cfg: cfg, identity: idsvc, settings: st, queries: q, channels: ch, messaging: msg, contacts: cts,
		campaigns: camp, inbox: inbox, webhooks: hooks,
		automation: automation.New(pool, msg, mail, automationHTTP(cfg), log)}, nil
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
	rtIssuer, err := realtime.NewIssuer(svc.cfg.Centrifugo.HMACSecret.Reveal())
	if err != nil {
		return err
	}
	srv := apiServer{
		identityHandler:   identity.NewHandler(svc.identity, log),
		settingsHandler:   settings.NewHandler(pool, svc.settings, log),
		auditHandler:      audit.NewHandler(svc.queries, log),
		systemHandler:     sys,
		channelsHandler:   channels.NewHandler(svc.channels, svc.cfg.PublicAppURL, svc.cfg.Meta.VerifyToken.Reveal(), log),
		messagesHandler:   messaging.NewHandler(svc.messaging, log),
		contactsHandler:   contacts.NewHandler(svc.contacts, log),
		campaignHandler:   campaigns.NewHandler(svc.campaigns, log),
		realtimeHandler:   realtime.NewHandler(rtIssuer.WithConversationAccess(svc.inbox.CanRead), log),
		inboxHandler:      conversations.NewHandler(svc.inbox, log),
		webhooksHandler:   webhooks.NewHandler(svc.webhooks, log),
		automationHandler: automation.NewHandler(svc.automation, log),
	}
	// Meta's webhooks: authenticated by signature, not by the API guard.
	r.Handle("/webhooks/whatsapp", svc.messaging.WebhookHandler(svc.cfg.Meta.VerifyToken.Reveal()))
	r.Group(func(g chi.Router) {
		g.Use(httpx.ClientMeta, guard.Middleware) // the guard applies per-operation body limits
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

// automationHTTP is the client automation HTTP steps use: the same
// SSRF-safe client as outgoing webhooks (plain http only in development).
func automationHTTP(cfg *config.Config) automation.Doer {
	return webhooks.NewClient(cfg.Env == config.EnvDevelopment)
}
