// Package billing is subscriptions and billing (Phase 17 WP9,
// docs/BILLING.md): the subscription state machine (trial, active,
// past_due, read_only, canceled) driven by a per-tenant scheduled job,
// proformas paid by bank transfer with a unique reference, the platform
// admin's payments inbox that matches incoming transfers, invoice records
// for the official e-Fatura/e-Arşiv invoice (invoicing.Issuer), the AI
// credit packs (credits.Grant, source purchase) and the TL message balance
// of companies that send through Messora's Tech Provider connection.
//
// Every write happens in a platform transaction scoped to the company
// (app.tenant_id set locally): the application role only reads billing
// rows (migration 00035), except the message wallet, which the pricing
// webhook debits under row-level security.
package billing

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/billing/invoicing"
	"github.com/furkanmeclis/messora-app-go/internal/modules/billing/payments"
	"github.com/furkanmeclis/messora-app-go/internal/modules/credits"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Subscription statuses (tenants.subscription_status). Trial is the
// "trialing" state: the stored value predates billing (WP2).
const (
	StatusTrial    = "trial"
	StatusActive   = "active"
	StatusPastDue  = "past_due"
	StatusReadOnly = "read_only"
	StatusCanceled = "canceled"
)

// Proforma kinds.
const (
	KindSubscription   = "subscription"
	KindUpgrade        = "upgrade"
	KindAICredits      = "ai_credits"
	KindMessageBalance = "message_balance"
)

// Error codes.
const (
	CodeReadOnly         = "SUBSCRIPTION_READ_ONLY"
	CodeBalanceExhausted = "MESSAGE_BALANCE_EXHAUSTED"
	CodeProfileRequired  = "BILLING_PROFILE_REQUIRED"
	CodeNotConfigured    = "BILLING_NOT_CONFIGURED"
)

// ErrReadOnly is the API answer while the subscription is read-only.
func ErrReadOnly() *apperr.Error {
	return apperr.PaymentRequired(CodeReadOnly,
		"The workspace is read-only until its subscription is paid: reading and exports work, sending and creating do not.")
}

// ErrBalanceExhausted is the answer to a template send through Messora's
// Tech Provider connection when the message balance is used up.
func ErrBalanceExhausted() *apperr.Error {
	return apperr.PaymentRequired(CodeBalanceExhausted, "The message balance is used up; top it up in Billing to send templates again.")
}

// Options wires the service.
type Options struct {
	// InvalidatePlan drops the cached plan of tenants whose plan changed.
	InvalidatePlan func(ids ...uuid.UUID)
	Credits        *credits.Service
	// Notify sends a billing e-mail of the context's workspace (identity).
	Notify func(ctx context.Context, n identity.BillingNotice) int
	// Issuer issues the official invoice (invoicing.Manual by default).
	Issuer invoicing.Issuer
	// Store keeps uploaded invoice PDFs (the private store).
	Store  storage.Store
	Logger *slog.Logger
}

// Service implements billing.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	o    Options
	now  func() time.Time
	// payments are the methods companies can pay with: bank transfer,
	// and the disabled card stub until a card provider is integrated.
	payments []payments.Provider

	cache settingsCache
	gate  gateCache
}

// New returns the service.
func New(pool *pgxpool.Pool, o Options) *Service {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.InvalidatePlan == nil {
		o.InvalidatePlan = func(...uuid.UUID) {}
	}
	if o.Issuer == nil {
		o.Issuer = invoicing.Manual{}
	}
	s := &Service{pool: pool, q: db.New(pool), o: o, now: time.Now, gate: gateCache{m: map[uuid.UUID]gateEntry{}}}
	s.payments = []payments.Provider{payments.BankTransfer{Accounts: s.accounts}, payments.Card{}}
	return s
}

// SetClock replaces the clock (tests).
func (s *Service) SetClock(now func() time.Time) {
	s.now = now
	s.InvalidateGate()
	s.InvalidateSettings()
}

// SetNotify replaces the e-mail sender.
func (s *Service) SetNotify(fn func(ctx context.Context, n identity.BillingNotice) int) {
	s.o.Notify = fn
}

func (s *Service) accounts(ctx context.Context) ([]payments.Account, error) {
	seller, _, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]payments.Account, 0, len(seller.BankAccounts))
	for _, b := range seller.BankAccounts {
		out = append(out, payments.Account{BankName: b.BankName, AccountHolder: b.AccountHolder, IBAN: b.Iban})
	}
	return out, nil
}

// effects are what a transaction does after it committed: e-mails and
// cache invalidations.
type effects struct {
	after []func(ctx context.Context)
	// invalidate names tenants whose plan or status changed.
	invalidate []uuid.UUID
}

func (e *effects) later(fn func(ctx context.Context)) { e.after = append(e.after, fn) }

// tenantTx runs fn in a platform transaction scoped to tenant id: billing
// rows are written by the platform role (the application role only reads
// them). fn gets the platform context.
func (s *Service) tenantTx(ctx context.Context, id uuid.UUID, fn func(ctx context.Context, q *db.Queries, fx *effects) error) error {
	return s.platformTx(ctx, func(ctx context.Context, q *db.Queries, fx *effects) error {
		if err := q.SetTenantLocal(ctx, id.String()); err != nil {
			return err
		}
		return fn(ctx, q, fx)
	})
}

// platformTx runs fn in a platform transaction without a tenant (transfers
// and lists across companies); fn may scope itself with SetTenantLocal.
func (s *Service) platformTx(ctx context.Context, fn func(ctx context.Context, q *db.Queries, fx *effects) error) error {
	pctx := tenant.Platform(ctx)
	tx, err := s.pool.Begin(pctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(pctx) }()
	fx := &effects{}
	if err := fn(pctx, s.q.WithTx(tx), fx); err != nil {
		return err
	}
	if err := tx.Commit(pctx); err != nil {
		return err
	}
	if len(fx.invalidate) > 0 {
		s.o.InvalidatePlan(fx.invalidate...)
		s.invalidateGate(fx.invalidate...)
	}
	for _, f := range fx.after {
		f(context.WithoutCancel(ctx))
	}
	return nil
}

// ── The read-only gate ─────────────────────────────────────────────────────

type gateEntry struct {
	readOnly bool
	expires  time.Time
}

type gateCache struct {
	mu sync.Mutex
	m  map[uuid.UUID]gateEntry
}

// GateTTL is how long a tenant's read-only state is used before it is read
// again on another replica (a change made on this replica applies at once).
const GateTTL = 5 * time.Second

// ReadOnly reports whether the context tenant's subscription blocks
// sending, AI, automations and creating: read_only, or canceled with the
// paid period over. A read error answers false (fail open: a database
// hiccup must not stop a paying company; the API and jobs check again).
func (s *Service) ReadOnly(ctx context.Context) bool {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return false
	}
	return s.ReadOnlyTenant(ctx, id)
}

// ReadOnlyTenant is ReadOnly for tenant id.
func (s *Service) ReadOnlyTenant(ctx context.Context, id uuid.UUID) bool {
	now := s.now()
	s.gate.mu.Lock()
	e, ok := s.gate.m[id]
	s.gate.mu.Unlock()
	if ok && now.Before(e.expires) {
		return e.readOnly
	}
	ro, err := s.readOnly(ctx, id, now)
	if err != nil {
		s.o.Logger.WarnContext(ctx, "read the subscription state", "tenant_id", id, "err", err)
		return ok && e.readOnly
	}
	s.gate.mu.Lock()
	s.gate.m[id] = gateEntry{readOnly: ro, expires: now.Add(GateTTL)}
	s.gate.mu.Unlock()
	return ro
}

func (s *Service) readOnly(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	t, err := s.q.GetTenant(tenant.System(ctx), id)
	if err != nil {
		return false, err
	}
	switch t.SubscriptionStatus {
	case StatusReadOnly:
		return true, nil
	case StatusCanceled:
		sub, err := s.q.GetSubscription(tenant.WithID(ctx, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return sub.PeriodEnd == nil || !now.Before(*sub.PeriodEnd), nil
	}
	return false, nil
}

func (s *Service) invalidateGate(ids ...uuid.UUID) {
	s.gate.mu.Lock()
	defer s.gate.mu.Unlock()
	for _, id := range ids {
		delete(s.gate.m, id)
	}
}

// InvalidateGate drops every cached read-only state (tests, status changes
// made outside the service).
func (s *Service) InvalidateGate() {
	s.gate.mu.Lock()
	s.gate.m = map[uuid.UUID]gateEntry{}
	s.gate.mu.Unlock()
}

// CheckWritable answers 402 SUBSCRIPTION_READ_ONLY while ctx's tenant is
// read-only (engines call it before sending or starting work).
func (s *Service) CheckWritable(ctx context.Context) error {
	if s.ReadOnly(ctx) {
		return ErrReadOnly()
	}
	return nil
}
