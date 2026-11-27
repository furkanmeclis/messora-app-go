package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/conversations"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/offers"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ops"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/modules/system"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apidocs"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/fairness"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
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
	offersHandler     = offers.Handler
	consentHandler    = consent.Handler
	aiHandler         = ai.Handler
	opsHandler        = ops.Handler
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
	*offersHandler
	*consentHandler
	*aiHandler
	*opsHandler
}

var _ api.ServerInterface = apiServer{}

// mfaEnrolmentOps are reachable while a mandatory second factor is not set up.
var mfaEnrolmentOps = []string{"getMe", "logout", "reauthenticate", "startTotpSetup", "confirmTotp", "getAuthCapabilities"}

// services holds what the API and CLI commands need.
type services struct {
	cfg        *config.Config
	throttle   *messaging.Throttle
	identity   *identity.Service
	settings   *settings.Store
	mail       *settings.Mail // nil when a test injects its own mailer
	queries    *db.Queries
	channels   *channels.Service
	messaging  *messaging.Service
	contacts   *contacts.Service
	campaigns  *campaigns.Service
	offers     *offers.Service
	inbox      *conversations.Service
	webhooks   *webhooks.Service
	automation *automation.Service
	ai         *ai.Service
	stores     *storage.Stores
	// tenants maps request hosts to tenants (tenant_domains).
	tenants *tenant.Resolver
	// fairness holds the noisy-neighbour limits (platform setting).
	fairness *fairness.Source
}

// openStorage opens the object store the configuration selects.
func openStorage(cfg *config.Config) (*storage.Stores, error) {
	s := cfg.Storage
	return storage.Open(context.Background(), storage.Options{
		Driver: s.Driver, LocalDir: s.LocalDir, AppURL: cfg.PublicAppURL, PublicAssetBaseURL: s.PublicAssetBaseURL,
		SigningKey: s.SigningKey, BucketPrivate: s.S3.BucketPrivate, BucketPublic: s.S3.BucketPublic,
		S3: storage.S3Options{Region: s.S3.Region, Endpoint: s.S3.Endpoint, ForcePathStyle: s.S3.ForcePathStyle,
			AccessKeyID: s.S3.AccessKeyID, SecretAccessKey: s.S3.SecretAccessKey.Reveal(), SSE: s.S3.SSE},
	})
}

func newServices(cfg *config.Config, pool *pgxpool.Pool, rdb goredis.UniversalClient, log *slog.Logger) (*services, error) {
	return newServicesWith(cfg, pool, rdb, log, nil, crypto.DefaultArgon2)
}

// newServicesWith lets tests inject a mailer and cheaper hashing parameters.
// A nil mailer uses SMTP_* from the environment or, without SMTP_HOST, the
// configuration chosen in the panel.
func newServicesWith(cfg *config.Config, pool *pgxpool.Pool, rdb goredis.UniversalClient, log *slog.Logger, mail mailer.Sender, argon crypto.Argon2Params) (*services, error) {
	sealer, err := crypto.NewSealer(cfg.EncryptionKey, nil)
	if err != nil {
		return nil, err
	}
	stores, err := openStorage(cfg)
	if err != nil {
		return nil, err
	}
	q := db.New(pool)
	st := settings.NewStore(q, 15*time.Second)
	if err := st.SetOverrides(cfg.Workspace); err != nil {
		return nil, err
	}
	var mailSettings *settings.Mail
	if mail == nil {
		mailSettings, err = settings.NewMail(q, sealer, mailer.Config{
			Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password.Reveal(), From: cfg.SMTP.From, TLS: cfg.SMTP.TLS,
		})
		if err != nil {
			return nil, err
		}
		mail = mailSettings
	}
	idsvc := identity.New(pool, rdb, sealer, st, mail, identity.Options{PublicAppURL: cfg.PublicAppURL, Logger: log, Argon2: argon})
	wa, err := whatsapp.New(whatsapp.Options{BaseURL: cfg.Meta.GraphBaseURL, Version: cfg.Meta.GraphVersion, DryRun: cfg.Meta.DryRun})
	if err != nil {
		return nil, err
	}
	ch := channels.New(pool, sealer, wa, cfg.Meta.AppSecret.Reveal(), log)
	// Token problems reach the owners and admins by e-mail (when SMTP is
	// set up); the panel shows them regardless.
	ch.SetNotifier(func(ctx context.Context, subject, body string) {
		if !mail.Enabled(ctx) {
			return
		}
		users, err := q.ListUsersWithRole(ctx)
		if err != nil {
			log.ErrorContext(ctx, "list admins for a channel notice", "err", err)
			return
		}
		for _, u := range users {
			if u.Status != "active" || (u.RoleKey != "owner" && u.RoleKey != "admin") {
				continue
			}
			if err := mail.Send(ctx, mailer.Message{To: u.Email, Subject: subject, Body: body}); err != nil {
				log.ErrorContext(ctx, "channel notice e-mail failed", "user_id", u.ID, "err", err)
			}
		}
	})
	media := messaging.NewMediaStore(stores.Private)
	msg := messaging.New(pool, ch, media, log)
	cts := contacts.New(pool, stores, log)
	camp := campaigns.New(pool, cts, log)
	msg.SetCampaignProgress(camp.PublishProgress)
	msg.SetCampaignWithheld(camp.ScheduleResend)
	// Campaign analytics: replies, quick replies, opt-outs and spam signals
	// are attributed in the transactions that record them.
	msg.SetCampaignHooks(messaging.CampaignHooks{Inbound: camp.RecordInbound, MessageError: camp.MessageSignal,
		Account: func(ctx context.Context, tx pgx.Tx, q *db.Queries, sig messaging.AccountSignal) error {
			return camp.AccountSignal(ctx, tx, q, campaigns.AccountSignalInfo(sig))
		}})
	consent.ObserveOptOuts(camp.RecordOptOut)
	camp.SetExportStore(campaigns.NewExportStore(stores))
	// The send scheduler: the worker paces sends with it, the dispatcher
	// and the queue view read capacities and shares from it.
	// Each tenant's campaigns together send at most the fairness setting's
	// rate, whatever numbers they use.
	fair := fairness.NewSource(pool)
	throttle := messaging.NewThrottle(rdb, messaging.DefaultThroughput).WithCeilings(msg.ThroughputCeiling).
		SetReservedShare(float64(cfg.Worker.CampaignReservedPercent) / 100).WithTenantCampaignCap(fair.TenantCampaignMPS)
	camp.SetScheduler(throttle)
	inbox := conversations.New(pool, msg, media, log)
	// Outgoing webhooks: https only outside development; private
	// destinations are always refused (internal/platform/safehttp).
	hooks := webhooks.New(pool, sealer, webhooks.Options{AllowHTTP: cfg.Env == config.EnvDevelopment, Logger: log})
	// AI provider calls go through an SSRF-safe client (https, public
	// addresses only); the fake provider only with AI_FAKE_PROVIDER.
	aiSvc := ai.New(pool, sealer, ai.Options{AllowFake: cfg.AIFakeProvider}, log)
	// The AI reply loop: inbound messages of conversations the AI answers
	// schedule an ai:reply turn (see conversations.HandleAIReply).
	inbox.SetAI(aiSvc)
	msg.SetInboundHook(inbox.OnInbound)
	wireCampaignAI(aiSvc, camp, inbox, q)
	// Automations: AI steps use the AI module, handoff and note steps the
	// conversations module.
	auto := automation.New(pool, msg, mail, automationHTTP(cfg), log)
	auto.SetAI(aiSvc)
	auto.SetInbox(inbox)
	// AI field steps store values with their provenance (never over a
	// person's or verified value).
	auto.SetFieldWriter(inbox)
	return &services{cfg: cfg, ai: aiSvc, identity: idsvc, settings: st, mail: mailSettings, queries: q, channels: ch, messaging: msg, contacts: cts,
		campaigns: camp, throttle: throttle, offers: offers.New(pool, camp, log), inbox: inbox, webhooks: hooks,
		automation: auto, stores: stores, tenants: tenant.NewResolver(pool, 15*time.Second), fairness: fair}, nil
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
		SessionLimit:    ratelimit.PerMinute(apiguard.SessionPerMinute),
		APIKeyLimit:     ratelimit.PerMinute(apiguard.APIKeyPerMinute),
		AnonymousLimit:  ratelimit.PerMinute(apiguard.AnonymousPerMinute),
	})
	if err != nil {
		return err
	}
	rtIssuer, err := realtime.NewIssuer(svc.cfg.Centrifugo.HMACSecret.Reveal())
	if err != nil {
		return err
	}
	sh := settings.NewHandler(pool, svc.settings, svc.mail, svc.stores, log)
	srv := apiServer{
		identityHandler:   identity.NewHandler(svc.identity, log),
		settingsHandler:   sh,
		auditHandler:      audit.NewHandler(svc.queries, log),
		systemHandler:     sys,
		channelsHandler:   channels.NewHandler(svc.channels, svc.cfg.PublicAppURL, svc.cfg.Meta.VerifyToken.Reveal(), log),
		messagesHandler:   messaging.NewHandler(svc.messaging, log),
		contactsHandler:   contacts.NewHandler(svc.contacts, log),
		campaignHandler:   campaigns.NewHandler(svc.campaigns, log),
		realtimeHandler:   realtime.NewHandler(rtIssuer.WithConversationAccess(svc.inbox.CanRead), log).WithSync(svc.inbox.ChangedSince, svc.campaigns.ChangedSince),
		inboxHandler:      conversations.NewHandler(svc.inbox, log),
		webhooksHandler:   webhooks.NewHandler(svc.webhooks, log),
		automationHandler: automation.NewHandler(svc.automation, log),
		consentHandler:    consent.NewHandler(pool, log),
		offersHandler:     offers.NewHandler(svc.offers, log),
		aiHandler:         ai.NewHandler(svc.ai, log),
		opsHandler: ops.NewHandler(ops.New(pool, ops.Deps{Campaigns: svc.campaigns, FailedJobs: sys.FailedJobsTotal,
			Budget: svc.ai.BudgetStatus}), log),
	}
	// The public API description (only what an API key can call), anonymous.
	raw, err := api.GetSpecJSON()
	if err != nil {
		return fmt.Errorf("load embedded spec: %w", err)
	}
	public, err := apidocs.PublicJSON(raw, apidocs.Options{Version: Version, APIKeyPerMinute: apiguard.APIKeyPerMinute})
	if err != nil {
		return err
	}
	r.Method(http.MethodGet, apidocs.Path, apidocs.Handler(public))
	r.Method(http.MethodHead, apidocs.Path, apidocs.Handler(public))
	// Everything below runs as the tenant of the request's host (unknown
	// hosts get 404, suspended tenants 403).
	byHost := tenant.Middleware(svc.tenants, log)
	// Presigned local downloads and public assets without a CDN:
	// authenticated by signature (or public), not by the API guard. The
	// key names its tenant (t/<tenant>/…) and the signature covers it; a
	// signed link answers only at an address of that tenant. Public assets
	// (logos) stay reachable from any host, e.g. in e-mails.
	r.Handle(storage.PublicPrefix+"*", svc.stores.Handler())
	r.With(byHost).Handle(storage.SignedPrefix+"*", svc.stores.Handler())
	// Meta's webhooks: authenticated by signature, not by the API guard.
	r.With(byHost).Handle("/webhooks/whatsapp", svc.messaging.WebhookHandler(svc.cfg.Meta.VerifyToken.Reveal()))
	r.Group(func(g chi.Router) {
		g.Use(byHost, httpx.ClientMeta, guard.Middleware) // the guard applies per-operation body limits
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

func newSystemHandler(pool *pgxpool.Pool, rdb goredis.UniversalClient, insp *asynq.Inspector, cf *centrifugo.Client, log *slog.Logger) *system.Handler {
	return system.NewHandler(Version, pool, rdb, insp, cf, log)
}

// automationHTTP is the client automation HTTP steps use: the same
// SSRF-safe client as outgoing webhooks (plain http only in development).
func automationHTTP(cfg *config.Config) automation.Doer {
	return webhooks.NewClient(cfg.Env == config.EnvDevelopment)
}

// wireCampaignAI connects the AI module and campaign analytics: one
// campaign attribution window (the campaign protection setting) for
// replies, opt-outs and AI usage, and free-text campaign replies classified
// by the AI with the result stored as the recipient's reaction (source ai)
// that counters and analysis read.
func wireCampaignAI(aiSvc *ai.Service, camp *campaigns.Service, inbox *conversations.Service, q *db.Queries) {
	aiSvc.SetAttributionWindow(func(ctx context.Context) (time.Duration, error) {
		st, err := campaigns.CachedAnalyticsSettings(ctx, q)
		return st.AttributionWindow(), err
	})
	camp.SetReplyClassifier(inbox.CampaignReplyClassifier())
	aiSvc.SetClassificationSink(func(ctx context.Context, campaignID, recipientID uuid.UUID, sentiment string) error {
		_, err := camp.SetReaction(ctx, campaignID, recipientID, &sentiment, campaigns.ReactionSourceAI)
		return err
	})
}
