package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/safehttp"
)

// TypeModelSync is the daily model catalogue sync (empty payload = every
// enabled provider).
const TypeModelSync = "ai:model_sync"

// requestTimeout bounds one Generate call (a DeepSeek repair included).
const requestTimeout = 90 * time.Second

// Options configures the service.
type Options struct {
	// AllowFake enables the fake provider kind (AI_FAKE_PROVIDER).
	AllowFake bool
	// HTTP is the client for provider calls. nil = an SSRF-safe client
	// (internal/platform/safehttp), https only.
	HTTP *http.Client
	// BaseURLs replaces the default endpoint of a kind (tests point them at
	// local servers). A connection's own base_url still wins.
	BaseURLs map[Kind]string
	// Now is the clock (tests); nil = time.Now.
	Now func() time.Time
}

// Service is the AI module.
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	sealer *crypto.Sealer
	log    *slog.Logger
	opts   Options

	mu   sync.RWMutex
	fake FakeScript
}

// New returns the service.
func New(pool *pgxpool.Pool, sealer *crypto.Sealer, opts Options, log *slog.Logger) *Service {
	if opts.HTTP == nil {
		opts.HTTP = safehttp.New(safehttp.Options{Timeout: requestTimeout}).HTTPClient()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Service{pool: pool, q: db.New(pool), sealer: sealer, log: log, opts: opts}
}

// FakeAllowed reports whether the fake provider kind is available.
func (s *Service) FakeAllowed() bool { return s.opts.AllowFake }

// SetFakeScript scripts the fake provider (tests). nil restores its
// default answers.
func (s *Service) SetFakeScript(f FakeScript) {
	s.mu.Lock()
	s.fake = f
	s.mu.Unlock()
}

func (s *Service) tx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ErrNotFound is returned for unknown ids.
var ErrNotFound = errors.New("ai: not found")

func keyAD(id uuid.UUID) string { return "ai_provider.key:" + id.String() }

// ---------------------------------------------------------------- providers

// ProviderInput creates or changes a connection; nil fields are unchanged.
type ProviderInput struct {
	Kind    Kind
	Name    *string
	APIKey  *string // a new key rotates the stored one
	BaseURL *string // "" removes the override
	Enabled *bool
}

// ListProviders returns every connection (keys stay sealed).
func (s *Service) ListProviders(ctx context.Context) ([]db.AiProvider, error) {
	return s.q.ListAIProviders(ctx)
}

// GetProvider returns one connection.
func (s *Service) GetProvider(ctx context.Context, id uuid.UUID) (db.AiProvider, error) {
	p, err := s.q.GetAIProvider(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func validKey(k string) error {
	if strings.TrimSpace(k) != k || len(k) < 8 || len(k) > 500 || strings.ContainsAny(k, " \t\r\n") {
		return &FieldError{"api_key", "The API key looks wrong (8–500 characters, no spaces)."}
	}
	return nil
}

func (s *Service) validKind(k Kind) error {
	if slices.Contains(Kinds, k) || (k == KindFake && s.opts.AllowFake) {
		return nil
	}
	return &FieldError{"kind", "Unknown provider."}
}

func validName(n string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" || len([]rune(n)) > 120 {
		return "", &FieldError{"name", "Name is required (at most 120 characters)."}
	}
	return n, nil
}

// CreateProvider seals the key, stores the connection and checks the key
// right away (a failed check is stored as the status, not an error).
func (s *Service) CreateProvider(ctx context.Context, in ProviderInput, actor *uuid.UUID) (db.AiProvider, error) {
	if err := s.validKind(in.Kind); err != nil {
		return db.AiProvider{}, err
	}
	name, err := validName(deref(in.Name))
	if err != nil {
		return db.AiProvider{}, err
	}
	key := deref(in.APIKey)
	if err := validKey(key); err != nil {
		return db.AiProvider{}, err
	}
	var base *string
	if in.BaseURL != nil {
		b, err := ValidateBaseURL(in.Kind, *in.BaseURL)
		if err != nil {
			return db.AiProvider{}, &FieldError{"base_url", err.Error()}
		}
		if b != "" {
			base = &b
		}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	id := uuid.Must(uuid.NewV7())
	sealed, err := s.sealer.Seal([]byte(key), keyAD(id))
	if err != nil {
		return db.AiProvider{}, err
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		_, err := q.InsertAIProvider(ctx, db.InsertAIProviderParams{ID: id, Kind: string(in.Kind), Name: name, ApiKeyEnc: sealed,
			KeyLast4: Last4(key), BaseUrl: base, Enabled: enabled, CreatedBy: actor})
		if isUnique(err) {
			return &FieldError{"name", "A provider with this name already exists."}
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.provider.create", EntityType: "ai_provider", EntityID: &id,
			Changes: map[string]any{"kind": in.Kind, "name": name, "base_url": base, "enabled": enabled}})
	})
	if err != nil {
		return db.AiProvider{}, err
	}
	return s.CheckProvider(ctx, id)
}

// UpdateProvider renames, rotates the key, changes the endpoint or
// enables/disables a connection. A new key or endpoint is checked.
func (s *Service) UpdateProvider(ctx context.Context, id uuid.UUID, in ProviderInput) (db.AiProvider, error) {
	cur, err := s.GetProvider(ctx, id)
	if err != nil {
		return cur, err
	}
	params := db.UpdateAIProviderParams{ID: id, Enabled: in.Enabled}
	changes := map[string]any{}
	if in.Name != nil {
		n, err := validName(*in.Name)
		if err != nil {
			return cur, err
		}
		params.Name = &n
		changes["name"] = []any{cur.Name, n}
	}
	if in.APIKey != nil {
		if err := validKey(*in.APIKey); err != nil {
			return cur, err
		}
		sealed, err := s.sealer.Seal([]byte(*in.APIKey), keyAD(id))
		if err != nil {
			return cur, err
		}
		params.ApiKeyEnc = sealed
		params.KeyLast4 = ptr(Last4(*in.APIKey))
		changes["api_key_rotated"] = true
	}
	if in.BaseURL != nil {
		b, err := ValidateBaseURL(Kind(cur.Kind), *in.BaseURL)
		if err != nil {
			return cur, &FieldError{"base_url", err.Error()}
		}
		params.SetBaseUrl = true
		if b != "" {
			params.BaseUrl = &b
		}
		changes["base_url"] = []any{cur.BaseUrl, params.BaseUrl}
	}
	if in.Enabled != nil {
		changes["enabled"] = []any{cur.Enabled, *in.Enabled}
	}
	var p db.AiProvider
	err = s.tx(ctx, func(q *db.Queries) error {
		p, err = q.UpdateAIProvider(ctx, params)
		if isUnique(err) {
			return &FieldError{"name", "A provider with this name already exists."}
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.provider.update", EntityType: "ai_provider", EntityID: &id, Changes: changes})
	})
	if err != nil {
		return p, err
	}
	if in.APIKey != nil || in.BaseURL != nil {
		return s.CheckProvider(ctx, id)
	}
	return p, nil
}

// DeleteProvider removes a connection and its models. Profiles fall back to
// the company default; a deleted default is cleared.
func (s *Service) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	cur, err := s.GetProvider(ctx, id)
	if err != nil {
		return err
	}
	st, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.DeleteAIProvider(ctx, id); err != nil {
			return err
		}
		if st.DefaultProviderID != nil && *st.DefaultProviderID == id {
			st.DefaultProviderID, st.DefaultModel = nil, ""
			raw, _ := json.Marshal(st)
			if err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: settingsKey, Value: raw}); err != nil {
				return err
			}
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.provider.delete", EntityType: "ai_provider", EntityID: &id,
			Changes: map[string]any{"kind": cur.Kind, "name": cur.Name}})
	})
}

// config opens the key and resolves the endpoint of a connection.
func (s *Service) config(p db.AiProvider) (ProviderConfig, error) {
	key, err := s.sealer.Open(p.ApiKeyEnc, keyAD(p.ID))
	if err != nil {
		return ProviderConfig{}, fmt.Errorf("ai: open the key of provider %s: %w", p.ID, err)
	}
	cfg := ProviderConfig{Kind: Kind(p.Kind), APIKey: string(key), HTTP: s.opts.HTTP, BaseURL: s.opts.BaseURLs[Kind(p.Kind)]}
	if p.BaseUrl != nil {
		cfg.BaseURL = *p.BaseUrl
	}
	if cfg.Kind == KindFake {
		s.mu.RLock()
		cfg.Fake = s.fake
		s.mu.RUnlock()
	}
	return cfg, nil
}

// CheckProvider validates the stored key and records the outcome.
func (s *Service) CheckProvider(ctx context.Context, id uuid.UUID) (db.AiProvider, error) {
	p, err := s.GetProvider(ctx, id)
	if err != nil {
		return p, err
	}
	if Kind(p.Kind) == KindFake && !s.opts.AllowFake {
		return s.q.SetAIProviderStatus(ctx, db.SetAIProviderStatusParams{ID: id, Status: string(StatusError),
			StatusDetail: "The fake provider is not available here (AI_FAKE_PROVIDER)."})
	}
	cfg, err := s.config(p)
	if err != nil {
		return p, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	status, detail := CheckKey(cctx, cfg)
	return s.q.SetAIProviderStatus(ctx, db.SetAIProviderStatusParams{ID: id, Status: string(status), StatusDetail: scrub(shorten(detail), cfg.APIKey)})
}

// --------------------------------------------------------------- catalogue

// SyncModels refreshes the model list of one connection.
func (s *Service) SyncModels(ctx context.Context, id uuid.UUID) (int, error) {
	p, err := s.GetProvider(ctx, id)
	if err != nil {
		return 0, err
	}
	if Kind(p.Kind) == KindFake && !s.opts.AllowFake {
		return 0, &Error{Kind: KindNotConfigured, Provider: KindFake, Message: "the fake provider is not available here"}
	}
	cfg, err := s.config(p)
	if err != nil {
		return 0, err
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	models, err := ListModels(cctx, cfg)
	if err != nil {
		return 0, err
	}
	keep := make([]string, 0, len(models))
	err = s.tx(ctx, func(q *db.Queries) error {
		for _, m := range models {
			if m.ID == "" || len(m.ID) > 200 || slices.Contains(keep, m.ID) {
				continue
			}
			caps, _ := json.Marshal(m.Capabilities)
			name := m.DisplayName
			if name == "" {
				name = m.ID
			}
			if err := q.UpsertAIModel(ctx, db.UpsertAIModelParams{ProviderID: id, ModelID: m.ID, DisplayName: name,
				ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutput, Capabilities: caps, Lifecycle: m.Lifecycle}); err != nil {
				return err
			}
			keep = append(keep, m.ID)
		}
		if _, err := q.DeleteStaleAIModels(ctx, db.DeleteStaleAIModelsParams{ProviderID: id, Keep: keep}); err != nil {
			return err
		}
		return q.MarkAIModelsSynced(ctx, id)
	})
	return len(keep), err
}

// SyncAll refreshes every enabled connection (the daily job). Failures are
// logged per provider and do not stop the others.
func (s *Service) SyncAll(ctx context.Context) error {
	ps, err := s.q.ListAIProviders(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if !p.Enabled || (Kind(p.Kind) == KindFake && !s.opts.AllowFake) {
			continue
		}
		if _, err := s.SyncModels(ctx, p.ID); err != nil {
			s.log.WarnContext(ctx, "ai model sync failed", "provider_id", p.ID, "kind", p.Kind, "error_kind", KindOf(err))
		}
	}
	return nil
}

// ModelView is a catalogue entry with the price valid now.
type ModelView struct {
	Model        db.ListAIModelsRow
	Capabilities Capabilities
	Price        *db.GetAIPriceRow
}

// ListModels returns the catalogue with today's prices.
func (s *Service) ListModels(ctx context.Context) ([]ModelView, error) {
	rows, err := s.q.ListAIModels(ctx)
	if err != nil {
		return nil, err
	}
	prices, err := s.q.ListAIPrices(ctx)
	if err != nil {
		return nil, err
	}
	domain := make([]Price, 0, len(prices))
	byID := map[uuid.UUID]db.GetAIPriceRow{}
	for _, p := range prices {
		r := db.GetAIPriceRow(p)
		domain = append(domain, priceFromRow(r))
		byID[r.ID] = r
	}
	now := s.opts.Now()
	out := make([]ModelView, len(rows))
	for i, m := range rows {
		var caps Capabilities
		_ = json.Unmarshal(m.Capabilities, &caps)
		out[i] = ModelView{Model: m, Capabilities: caps}
		if p, ok := SelectPrice(domain, Kind(m.ProviderKind), m.ModelID, now); ok {
			r := byID[p.ID]
			out[i].Price = &r
		}
	}
	return out, nil
}

// ------------------------------------------------------------------ prices

func priceFromRow(r db.GetAIPriceRow) Price {
	rules, _ := ParseRules(r.Rules)
	p := Price{ID: r.ID, Kind: Kind(r.ProviderKind), Pattern: r.ModelPattern, Currency: r.Currency,
		Input: ratOf(&r.Input), Output: ratOf(&r.Output), CachedInput: ratOf(nonEmpty(r.CachedInput)),
		CacheWrite5m: ratOf(nonEmpty(r.CacheWrite5m)), CacheWrite1h: ratOf(nonEmpty(r.CacheWrite1h)), Rules: rules,
		EffectiveFrom: r.EffectiveFrom.Time.UTC()}
	if r.EffectiveTo.Valid {
		t := r.EffectiveTo.Time.UTC()
		p.EffectiveTo = &t
	}
	return p
}

// ListPrices returns every price row.
func (s *Service) ListPrices(ctx context.Context) ([]db.GetAIPriceRow, error) {
	rows, err := s.q.ListAIPrices(ctx)
	out := make([]db.GetAIPriceRow, len(rows))
	for i, r := range rows {
		out[i] = db.GetAIPriceRow(r)
	}
	return out, err
}

func rulesJSON(r Rules) []byte {
	b, _ := json.Marshal(r)
	return b
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func dateP(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return date(*t)
}

func upsertParams(in PriceInput) db.UpsertAIPriceParams {
	return db.UpsertAIPriceParams{ProviderKind: string(in.Kind), ModelPattern: in.Pattern, Currency: in.Currency, Input: in.Input,
		CachedInput: in.CachedInput, CacheWrite5m: in.CacheWrite5m, CacheWrite1h: in.CacheWrite1h, Output: in.Output,
		Rules: rulesJSON(in.Rules), EffectiveFrom: date(in.EffectiveFrom), EffectiveTo: dateP(in.EffectiveTo),
		Source: in.Source, SourceUrl: in.SourceURL, Notes: in.Notes}
}

// CreatePrice adds a price row entered by an admin; it counts as confirmed
// by them.
func (s *Service) CreatePrice(ctx context.Context, in PriceInput, actor *uuid.UUID) (db.GetAIPriceRow, error) {
	in.Source = "manual"
	if err := in.Validate(); err != nil {
		return db.GetAIPriceRow{}, err
	}
	var out db.GetAIPriceRow
	err := s.tx(ctx, func(q *db.Queries) error {
		row, err := q.InsertAIPrice(ctx, db.InsertAIPriceParams(upsertParams(in)))
		if isUnique(err) {
			return &FieldError{"effective_from", "A price for this model pattern and date already exists; edit it instead."}
		}
		if err != nil {
			return err
		}
		now := s.opts.Now()
		u, err := q.UpdateAIPrice(ctx, updateParams(db.GetAIPriceRow(row), in, actor, &now))
		if err != nil {
			return err
		}
		out = db.GetAIPriceRow(u)
		return audit.Record(ctx, q, audit.Entry{Action: "ai.price.create", EntityType: "ai_model_price", EntityID: &out.ID,
			Changes: map[string]any{"provider_kind": in.Kind, "model_pattern": in.Pattern, "input": in.Input, "output": in.Output,
				"effective_from": in.EffectiveFrom.Format(time.DateOnly)}})
	})
	return out, err
}

func updateParams(cur db.GetAIPriceRow, in PriceInput, by *uuid.UUID, at *time.Time) db.UpdateAIPriceParams {
	return db.UpdateAIPriceParams{ID: cur.ID, ModelPattern: in.Pattern, Currency: in.Currency, Input: in.Input,
		CachedInput: in.CachedInput, CacheWrite5m: in.CacheWrite5m, CacheWrite1h: in.CacheWrite1h, Output: in.Output,
		Rules: rulesJSON(in.Rules), EffectiveFrom: date(in.EffectiveFrom), EffectiveTo: dateP(in.EffectiveTo),
		SourceUrl: in.SourceURL, Notes: in.Notes, ConfirmedBy: by, ConfirmedAt: at}
}

// PriceFromRow turns a stored row back into an input (for partial edits).
func PriceFromRow(r db.GetAIPriceRow) PriceInput {
	rules, _ := ParseRules(r.Rules)
	in := PriceInput{Kind: Kind(r.ProviderKind), Pattern: r.ModelPattern, Currency: r.Currency, Input: r.Input, Output: r.Output,
		CachedInput: nonEmpty(r.CachedInput), CacheWrite5m: nonEmpty(r.CacheWrite5m), CacheWrite1h: nonEmpty(r.CacheWrite1h),
		Rules: rules, EffectiveFrom: r.EffectiveFrom.Time, Source: r.Source, SourceURL: r.SourceUrl, Notes: r.Notes}
	if r.EffectiveTo.Valid {
		t := r.EffectiveTo.Time
		in.EffectiveTo = &t
	}
	return in
}

// GetPrice returns one row.
func (s *Service) GetPrice(ctx context.Context, id uuid.UUID) (db.GetAIPriceRow, error) {
	r, err := s.q.GetAIPrice(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// UpdatePrice stores an edited row. confirm=true records the actor as
// having checked it against the source; any edit without confirm clears a
// previous confirmation. Costs already recorded keep their amount.
func (s *Service) UpdatePrice(ctx context.Context, id uuid.UUID, in PriceInput, confirm bool, actor *uuid.UUID) (db.GetAIPriceRow, error) {
	cur, err := s.GetPrice(ctx, id)
	if err != nil {
		return cur, err
	}
	in.Kind = Kind(cur.ProviderKind)
	if err := in.Validate(); err != nil {
		return cur, err
	}
	var by *uuid.UUID
	var at *time.Time
	if confirm {
		now := s.opts.Now()
		by, at = actor, &now
	} else if old := PriceFromRow(cur); old.Validate() == nil && old.sameFigures(in) {
		by, at = cur.ConfirmedBy, cur.ConfirmedAt // unchanged figures keep the confirmation
	}
	var out db.GetAIPriceRow
	err = s.tx(ctx, func(q *db.Queries) error {
		u, err := q.UpdateAIPrice(ctx, updateParams(cur, in, by, at))
		if isUnique(err) {
			return &FieldError{"effective_from", "A price for this model pattern and date already exists."}
		}
		if err != nil {
			return err
		}
		out = db.GetAIPriceRow(u)
		return audit.Record(ctx, q, audit.Entry{Action: "ai.price.update", EntityType: "ai_model_price", EntityID: &id,
			Changes: map[string]any{"model_pattern": in.Pattern, "input": []any{cur.Input, in.Input}, "output": []any{cur.Output, in.Output},
				"confirmed": confirm}})
	})
	return out, err
}

func (a PriceInput) sameFigures(b PriceInput) bool {
	ra, _ := json.Marshal(a.Rules)
	rb, _ := json.Marshal(b.Rules)
	return a.Pattern == b.Pattern && a.Currency == b.Currency && a.Input == b.Input && a.Output == b.Output &&
		deref(a.CachedInput) == deref(b.CachedInput) && deref(a.CacheWrite5m) == deref(b.CacheWrite5m) &&
		deref(a.CacheWrite1h) == deref(b.CacheWrite1h) && string(ra) == string(rb) &&
		a.EffectiveFrom.Equal(b.EffectiveFrom) && timePtrEq(a.EffectiveTo, b.EffectiveTo)
}

func timePtrEq(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// DeletePrice removes a row (recorded costs keep their amount).
func (s *Service) DeletePrice(ctx context.Context, id uuid.UUID) error {
	cur, err := s.GetPrice(ctx, id)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.DeleteAIPrice(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.price.delete", EntityType: "ai_model_price", EntityID: &id,
			Changes: map[string]any{"provider_kind": cur.ProviderKind, "model_pattern": cur.ModelPattern}})
	})
}

// ImportPrices stores rows parsed from LiteLLM's list, replacing rows of
// the same kind, pattern and date (which then need a new confirmation).
func (s *Service) ImportPrices(ctx context.Context, res ImportResult) error {
	return s.tx(ctx, func(q *db.Queries) error {
		for _, p := range res.Prices {
			if _, err := q.UpsertAIPrice(ctx, upsertParams(p)); err != nil {
				return err
			}
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.price.import", EntityType: "ai_model_price",
			Changes: map[string]any{"source": "litellm", "prices": len(res.Prices), "skipped": res.Skipped}})
	})
}

const seededKey = "ai_prices_seeded"

// EnsureSeed stores the shipped seed prices once per installation; rows an
// admin deleted later do not come back.
func (s *Service) EnsureSeed(ctx context.Context) error {
	seed, err := SeedPrices()
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		n, err := q.InsertSettingIfAbsent(ctx, db.InsertSettingIfAbsentParams{Key: seededKey, Value: []byte(`{"date":"` + SeedDate.Format(time.DateOnly) + `"}`)})
		if err != nil || n == 0 {
			return err
		}
		for _, p := range seed {
			if _, err := q.InsertAIPriceIfAbsent(ctx, db.InsertAIPriceIfAbsentParams(upsertParams(p))); err != nil {
				return err
			}
		}
		return nil
	})
}

// ----------------------------------------------------------------- generate

// Call is one AI request with what it is for. Zero ids are unset.
type Call struct {
	Purpose         string
	ProviderID      *uuid.UUID // nil = the profile's, else the company default
	Model           string     // "" = the profile's, else the company default
	ProfileID       *uuid.UUID
	ConversationID  *uuid.UUID
	ContactID       *uuid.UUID
	CampaignID      *uuid.UUID
	AutomationRunID *uuid.UUID
	Request         Request
}

// Result is a completed (or failed) request with its recorded cost.
type Result struct {
	Response
	UsageID      uuid.UUID
	ProviderID   *uuid.UUID
	ProviderName string
	ProviderKind Kind
	Cost         *string // nil = no price known for the model
	Currency     string
	Estimated    bool
	// CampaignID is the campaign the request was attributed to (nil = none).
	CampaignID *uuid.UUID
}

var purposes = []string{PurposeReply, PurposeSuggest, PurposeClassify, PurposeExtract, PurposeSummarize, PurposeAutomation, PurposeTest}

// Generate runs a request through the configured provider and records
// exactly one ai_usage row, whether it succeeds or not. While AI is off it
// returns ErrAIDisabled (and records the refused call).
func (s *Service) Generate(ctx context.Context, c Call) (Result, error) {
	if !slices.Contains(purposes, c.Purpose) {
		return Result{}, fmt.Errorf("ai: unknown purpose %q", c.Purpose)
	}
	start := s.opts.Now()
	res := Result{}
	prov, model, err := s.resolve(ctx, c)
	if err == nil {
		res.ProviderID, res.ProviderName, res.ProviderKind = &prov.ID, prov.Name, Kind(prov.Kind)
		var cfg ProviderConfig
		if cfg, err = s.config(prov); err == nil {
			var client Client
			if client, err = NewClient(cfg); err == nil {
				req := c.Request
				req.Model = model
				cctx, cancel := context.WithTimeout(ctx, requestTimeout)
				res.Response, err = client.Generate(cctx, req)
				cancel()
			}
		}
	}
	if res.Model == "" {
		res.Model = model
	}
	if res.Latency == 0 {
		res.Latency = s.opts.Now().Sub(start)
	}
	s.record(ctx, c, model, start, &res, err)
	return res, err
}

// resolve picks the connection and model: the call's, the profile's, then
// the company default. A connection must be enabled; AI must be on.
func (s *Service) resolve(ctx context.Context, c Call) (db.AiProvider, string, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return db.AiProvider{}, c.Model, err
	}
	if !st.Enabled {
		return db.AiProvider{}, c.Model, &Error{Kind: KindDisabled, Message: "AI is turned off for the company"}
	}
	pid, model := c.ProviderID, c.Model
	if c.ProfileID != nil {
		pr, err := s.q.GetAIProfile(ctx, *c.ProfileID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return db.AiProvider{}, model, err
		}
		// The profile's override applies unless the call names its own provider.
		if err == nil && c.ProviderID == nil {
			pid = pr.ProviderID
			if model == "" && pr.Model != nil {
				model = *pr.Model
			}
		}
	}
	if pid == nil {
		pid = st.DefaultProviderID
	}
	if pid == nil {
		return db.AiProvider{}, model, &Error{Kind: KindNotConfigured, Message: "no AI provider is chosen (set a default in the AI settings)"}
	}
	if model == "" && st.DefaultProviderID != nil && *pid == *st.DefaultProviderID {
		model = st.DefaultModel
	}
	p, err := s.q.GetAIProvider(ctx, *pid)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, model, &Error{Kind: KindNotConfigured, Message: "the chosen AI provider no longer exists"}
	}
	if err != nil {
		return p, model, err
	}
	switch {
	case !p.Enabled:
		return p, model, &Error{Kind: KindNotConfigured, Provider: Kind(p.Kind), Message: "the AI provider is disabled"}
	case Kind(p.Kind) == KindFake && !s.opts.AllowFake:
		return p, model, &Error{Kind: KindNotConfigured, Provider: KindFake, Message: "the fake provider is not available here"}
	case model == "":
		return p, model, &Error{Kind: KindNotConfigured, Provider: Kind(p.Kind), Message: "no model is chosen for the AI provider"}
	}
	return p, model, nil
}

// record writes the usage row and fills the cost into res. A failure to
// record is logged, never returned: the answer already exists.
func (s *Service) record(ctx context.Context, c Call, model string, at time.Time, res *Result, callErr error) {
	params := db.InsertAIUsageParams{ProviderID: res.ProviderID, ProviderKind: string(res.ProviderKind), Model: model,
		Purpose: c.Purpose, ProfileID: c.ProfileID, ConversationID: c.ConversationID, ContactID: c.ContactID,
		CampaignID: c.CampaignID, AutomationRunID: c.AutomationRunID,
		InputTokens: res.Usage.InputTokens, CachedTokens: res.Usage.CachedInputTokens, CacheWriteTokens: res.Usage.CacheWriteTokens,
		CacheWrite1hTokens: res.Usage.CacheWrite1hTokens, OutputTokens: res.Usage.OutputTokens, ReasoningTokens: res.Usage.ReasoningTokens,
		LatencyMs: res.Latency.Milliseconds(), Status: "ok"}
	if callErr != nil {
		params.Status = "error"
		k := string(KindOf(callErr))
		if k == "" {
			k = "internal"
		}
		params.ErrorKind = &k
	}
	if res.ProviderKind != "" && model != "" {
		if prices, err := s.q.ListAIPricesForKind(ctx, string(res.ProviderKind)); err == nil {
			domain := make([]Price, len(prices))
			for i, p := range prices {
				domain[i] = priceFromRow(db.GetAIPriceRow(p))
			}
			p, ok := SelectPrice(domain, res.ProviderKind, model, at)
			if !ok && res.Model != "" && res.Model != model {
				p, ok = SelectPrice(domain, res.ProviderKind, res.Model, at) // e.g. a dated snapshot id
			}
			if ok {
				if cost, err := PriceUsage(p, res.Usage, at); err == nil {
					amount := cost.String()
					res.Cost, res.Currency, res.Estimated = &amount, cost.Currency, cost.Estimated
					params.Cost, params.Currency, params.PriceID = &amount, &cost.Currency, &p.ID
				}
			}
		}
	}
	if params.CampaignID == nil && c.ConversationID != nil {
		// Campaign attribution at insert time; the reconcile job retries
		// rows left without one.
		if st, err := s.Settings(ctx); err == nil {
			if cid, err := attributeCampaign(ctx, s.q, *c.ConversationID, at, st.AttributionWindow()); err == nil {
				params.CampaignID = cid
			} else {
				s.log.WarnContext(ctx, "ai usage campaign attribution", "err", err)
			}
		}
	}
	res.CampaignID = params.CampaignID
	id, err := s.q.InsertAIUsage(ctx, params)
	if err != nil {
		s.log.ErrorContext(ctx, "record ai usage", "err", err, "purpose", c.Purpose)
		return
	}
	res.UsageID = id
}

// ------------------------------------------------------------------- utils

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isUnique(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}
