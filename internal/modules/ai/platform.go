package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Phase 17 WP8: the platform's own AI keys and AI credits.
//
// Messora holds one key per provider kind (platform_ai_providers, managed
// in the platform admin panel). A company uses them by default; every such
// call is paid with its AI credits (internal/modules/credits). A company
// whose plan includes byo_ai_keys (enterprise) may add its own keys
// (ai_providers): calls with them consume no credits, and a kind without
// an own key falls back to Messora's key and credits. On other plans the
// company's stored own keys are kept but never used.

// Meter meters the AI calls made with the platform's keys (credits.Service).
type Meter interface {
	// Available reports whether the context's tenant has credits left.
	Available(ctx context.Context) (bool, error)
	// Charge converts a call's USD cost ("" = no price) into millicredits.
	Charge(ctx context.Context, cost string, ok bool) int64
	// Consume books millicredits inside q's transaction.
	Consume(ctx context.Context, q *db.Queries, amount int64) error
}

// SetPlans installs the plan provider (features ai_agents, byo_ai_keys).
func (s *Service) SetPlans(p entitlements.Provider) { s.plans = p }

// SetMeter installs the credit meter. Without one, calls with the
// platform's keys are not metered.
func (s *Service) SetMeter(m Meter) { s.meter = m }

// Key sources of a call (ai_usage.key_source).
const (
	KeySourceTenant   = "tenant"
	KeySourcePlatform = "platform"
)

func platformKeyAD(id uuid.UUID) string { return "platform_ai_provider.key:" + id.String() }

// PlatformSettingKey holds the default provider and model of the platform's
// keys (used when a company chose none).
const PlatformSettingKey = "ai_platform"

// PlatformSettings is the platform's AI default.
type PlatformSettings struct {
	DefaultProviderID *uuid.UUID `json:"default_provider_id"`
	DefaultModel      string     `json:"default_model"`
}

// PlatformDefaults reads the platform's AI default.
func (s *Service) PlatformDefaults(ctx context.Context) (PlatformSettings, error) {
	var out PlatformSettings
	raw, err := s.q.GetPlatformSetting(tenant.System(ctx), PlatformSettingKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

// SavePlatformDefaults stores the platform's AI default with q (a platform
// transaction).
func (s *Service) SavePlatformDefaults(ctx context.Context, q db.Querier, st PlatformSettings) error {
	st.DefaultModel = strings.TrimSpace(st.DefaultModel)
	if len(st.DefaultModel) > 200 {
		return &FieldError{"default_model", "Model id is too long."}
	}
	if st.DefaultProviderID != nil {
		if _, err := q.GetPlatformAIProvider(ctx, *st.DefaultProviderID); errors.Is(err, pgx.ErrNoRows) {
			return &FieldError{"default_provider_id", "Unknown provider."}
		} else if err != nil {
			return err
		}
		if st.DefaultModel == "" {
			return &FieldError{"default_model", "Choose the default model of the provider."}
		}
	} else if st.DefaultModel != "" {
		return &FieldError{"default_provider_id", "Choose the provider of the default model."}
	}
	raw, _ := json.Marshal(st)
	return q.UpsertPlatformSetting(ctx, db.UpsertPlatformSettingParams{Key: PlatformSettingKey, Value: raw})
}

// ── Platform keys (platform admin panel; tenant.Platform contexts) ─────────

// ListPlatformProviders returns Messora's keys (sealed).
func (s *Service) ListPlatformProviders(ctx context.Context) ([]db.PlatformAiProvider, error) {
	return s.q.ListPlatformAIProviders(ctx)
}

// PlatformModelCounts counts the catalogue entries of each platform key.
func (s *Service) PlatformModelCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	rows, err := s.q.ListPlatformAIModels(ctx, true)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]int64{}
	for _, r := range rows {
		out[r.ProviderID]++
	}
	return out, nil
}

// GetPlatformProvider returns one of Messora's keys.
func (s *Service) GetPlatformProvider(ctx context.Context, id uuid.UUID) (db.PlatformAiProvider, error) {
	p, err := s.q.GetPlatformAIProvider(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// ErrProviderExists is returned for a second key of the same kind.
var ErrProviderExists = errors.New("ai: the platform already has a key for this provider")

// CreatePlatformProvider stores Messora's key for a kind and checks it.
func (s *Service) CreatePlatformProvider(ctx context.Context, in ProviderInput) (db.PlatformAiProvider, error) {
	if err := s.validKind(in.Kind); err != nil {
		return db.PlatformAiProvider{}, err
	}
	name, err := validName(deref(in.Name))
	if err != nil {
		return db.PlatformAiProvider{}, err
	}
	key := deref(in.APIKey)
	if err := validKey(key); err != nil {
		return db.PlatformAiProvider{}, err
	}
	var base *string
	if in.BaseURL != nil {
		b, err := ValidateBaseURL(in.Kind, *in.BaseURL)
		if err != nil {
			return db.PlatformAiProvider{}, &FieldError{"base_url", err.Error()}
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
	sealed, err := s.sealer.Seal([]byte(key), platformKeyAD(id))
	if err != nil {
		return db.PlatformAiProvider{}, err
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		_, err := q.InsertPlatformAIProvider(ctx, db.InsertPlatformAIProviderParams{ID: id, Kind: string(in.Kind), Name: name,
			ApiKeyEnc: sealed, KeyLast4: Last4(key), BaseUrl: base, Enabled: enabled})
		if isUnique(err) {
			return ErrProviderExists
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.ai_provider_create", EntityType: "platform_ai_provider", EntityID: &id,
			Changes: map[string]any{"kind": in.Kind, "name": name, "base_url": base, "enabled": enabled}})
	})
	if err != nil {
		return db.PlatformAiProvider{}, err
	}
	return s.TestPlatformProvider(ctx, id)
}

// UpdatePlatformProvider renames, rotates, re-points or switches a key.
func (s *Service) UpdatePlatformProvider(ctx context.Context, id uuid.UUID, in ProviderInput) (db.PlatformAiProvider, error) {
	cur, err := s.GetPlatformProvider(ctx, id)
	if err != nil {
		return cur, err
	}
	params := db.UpdatePlatformAIProviderParams{ID: id, Enabled: in.Enabled}
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
		sealed, err := s.sealer.Seal([]byte(*in.APIKey), platformKeyAD(id))
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
	var p db.PlatformAiProvider
	err = s.tx(ctx, func(q *db.Queries) error {
		if p, err = q.UpdatePlatformAIProvider(ctx, params); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.ai_provider_update", EntityType: "platform_ai_provider", EntityID: &id, Changes: changes})
	})
	if err != nil {
		return p, err
	}
	if in.APIKey != nil || in.BaseURL != nil {
		return s.TestPlatformProvider(ctx, id)
	}
	return p, nil
}

// DeletePlatformProvider removes Messora's key for a kind (and its models).
// Companies using it get AI_NOT_CONFIGURED until a key is added again.
func (s *Service) DeletePlatformProvider(ctx context.Context, id uuid.UUID) error {
	cur, err := s.GetPlatformProvider(ctx, id)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.DeletePlatformAIProvider(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.ai_provider_delete", EntityType: "platform_ai_provider", EntityID: &id,
			Changes: map[string]any{"kind": cur.Kind, "name": cur.Name}})
	})
}

func (s *Service) platformConfig(p db.PlatformAiProvider) (ProviderConfig, error) {
	key, err := s.sealer.Open(p.ApiKeyEnc, platformKeyAD(p.ID))
	if err != nil {
		return ProviderConfig{}, fmt.Errorf("ai: open the key of platform provider %s: %w", p.ID, err)
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

// TestPlatformProvider checks a key and refreshes its model list; the
// outcome is the provider's status (a failed check is not an error).
func (s *Service) TestPlatformProvider(ctx context.Context, id uuid.UUID) (db.PlatformAiProvider, error) {
	p, err := s.GetPlatformProvider(ctx, id)
	if err != nil {
		return p, err
	}
	if Kind(p.Kind) == KindFake && !s.opts.AllowFake {
		return s.q.SetPlatformAIProviderStatus(ctx, db.SetPlatformAIProviderStatusParams{ID: id, Status: string(StatusError),
			StatusDetail: "The fake provider is not available here (AI_FAKE_PROVIDER)."})
	}
	cfg, err := s.platformConfig(p)
	if err != nil {
		return p, err
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	status, detail := CheckKey(cctx, cfg)
	p, err = s.q.SetPlatformAIProviderStatus(ctx, db.SetPlatformAIProviderStatusParams{ID: id, Status: string(status),
		StatusDetail: scrub(shorten(detail), cfg.APIKey)})
	if err != nil || status != StatusOK {
		return p, err
	}
	mctx, mcancel := context.WithTimeout(ctx, 60*time.Second)
	defer mcancel()
	models, lerr := ListModels(mctx, cfg)
	if lerr != nil {
		s.log.WarnContext(ctx, "platform ai model sync failed", "provider_id", id, "error_kind", KindOf(lerr))
		return p, nil
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
			if err := q.UpsertPlatformAIModel(ctx, db.UpsertPlatformAIModelParams{ProviderID: id, ModelID: m.ID, DisplayName: name,
				ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutput, Capabilities: caps, Lifecycle: m.Lifecycle}); err != nil {
				return err
			}
			keep = append(keep, m.ID)
		}
		if _, err := q.DeleteStalePlatformAIModels(ctx, db.DeleteStalePlatformAIModelsParams{ProviderID: id, Keep: keep}); err != nil {
			return err
		}
		return q.MarkPlatformAIModelsSynced(ctx, id)
	})
	if err != nil {
		return p, err
	}
	return s.GetPlatformProvider(ctx, id)
}

// PlatformModels lists the models of Messora's enabled keys with today's
// prices (also what companies see with key_source platform).
func (s *Service) PlatformModels(ctx context.Context) ([]ModelView, error) {
	rows, err := s.q.ListPlatformAIModels(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]ModelView, len(rows))
	for i, m := range rows {
		out[i] = ModelView{KeySource: KeySourcePlatform, Model: db.ListAIModelsRow{ProviderID: m.ProviderID, ModelID: m.ModelID,
			DisplayName: m.DisplayName, ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutput, Capabilities: m.Capabilities,
			Lifecycle: m.Lifecycle, SyncedAt: m.SyncedAt, ProviderKind: m.ProviderKind, ProviderName: m.ProviderName}}
		_ = json.Unmarshal(m.Capabilities, &out[i].Capabilities)
	}
	return out, s.attachPrices(ctx, out)
}

// ── Resolution of a call ───────────────────────────────────────────────────

// route is the key a call uses: the company's own (tenant) or Messora's
// (platform).
type route struct {
	tenant   *db.AiProvider
	platform *db.PlatformAiProvider
	model    string
}

func (r route) kind() Kind {
	switch {
	case r.tenant != nil:
		return Kind(r.tenant.Kind)
	case r.platform != nil:
		return Kind(r.platform.Kind)
	}
	return ""
}

func (r route) keySource() *string {
	switch {
	case r.tenant != nil:
		return ptr(KeySourceTenant)
	case r.platform != nil:
		return ptr(KeySourcePlatform)
	}
	return nil
}

// plan returns the context tenant's plan (Default without a provider).
func (s *Service) plan(ctx context.Context) (entitlements.Plan, error) {
	if s.plans == nil {
		return entitlements.Default(), nil
	}
	return entitlements.For(ctx, s.plans)
}

// OwnKeysAllowed reports whether the context's company may use its own AI
// keys (byo_ai_keys).
func (s *Service) OwnKeysAllowed(ctx context.Context) bool {
	p, err := s.plan(ctx)
	return err == nil && p.Has(entitlements.BYOAIKeys)
}

// errNotInPlan is a call of a company whose plan has no AI.
var errNotInPlan = &Error{Kind: KindNotInPlan, Message: "AI is not included in the workspace's plan"}

// errCreditsExhausted is a call with the platform's keys without credits.
var errCreditsExhausted = &Error{Kind: KindCreditsExhausted, Message: "the workspace's AI credits are used up"}

// platformRoute finds Messora's key of kind (an enabled one), and checks the
// credits.
func (s *Service) platformRoute(ctx context.Context, kind Kind, byID *uuid.UUID, model string, why string) (route, error) {
	var (
		p   db.PlatformAiProvider
		err error
	)
	if byID != nil {
		p, err = s.q.GetPlatformAIProvider(ctx, *byID)
	} else {
		p, err = s.q.GetPlatformAIProviderByKind(ctx, string(kind))
	}
	if errors.Is(err, pgx.ErrNoRows) {
		msg := "Messora has no AI key for " + string(kind)
		if kind == "" {
			msg = "no AI provider is chosen (set a default in the AI settings)"
		}
		if why != "" {
			msg += " (" + why + ")"
		}
		return route{model: model}, &Error{Kind: KindNotConfigured, Provider: kind, Message: msg}
	}
	if err != nil {
		return route{model: model}, err
	}
	rt := route{platform: &p, model: model}
	switch {
	case !p.Enabled:
		return rt, &Error{Kind: KindNotConfigured, Provider: Kind(p.Kind), Message: "Messora's key for this provider is switched off"}
	case Kind(p.Kind) == KindFake && !s.opts.AllowFake:
		return rt, &Error{Kind: KindNotConfigured, Provider: KindFake, Message: "the fake provider is not available here"}
	case model == "":
		return rt, &Error{Kind: KindNotConfigured, Provider: Kind(p.Kind), Message: "no model is chosen for the AI provider"}
	}
	if s.meter != nil {
		ok, err := s.meter.Available(ctx)
		if err != nil {
			return rt, err
		}
		if !ok {
			return rt, errCreditsExhausted
		}
	}
	return rt, nil
}

// resolve picks the key and model of a call: the call's provider, the
// profile's, then the company default (each may be one of the company's
// own keys or one of Messora's); with nothing chosen, the platform's
// default. A company's own key is used only when its plan includes
// byo_ai_keys and the key is enabled; otherwise the call goes to Messora's
// key of the same kind and is paid with credits.
func (s *Service) resolve(ctx context.Context, c Call) (route, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return route{model: c.Model}, err
	}
	if !st.Enabled {
		return route{model: c.Model}, &Error{Kind: KindDisabled, Message: "AI is turned off for the company"}
	}
	plan, err := s.plan(ctx)
	if err != nil {
		return route{model: c.Model}, err
	}
	if !plan.Has(entitlements.AIAgents) {
		return route{model: c.Model}, errNotInPlan
	}
	pid, model := c.ProviderID, c.Model
	if c.ProfileID != nil {
		pr, err := s.q.GetAIProfile(ctx, *c.ProfileID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return route{model: model}, err
		}
		// The profile's override applies unless the call names its own provider.
		if err == nil && c.ProviderID == nil {
			pid = pr.ProviderID
			if pid == nil {
				pid = pr.PlatformProviderID
			}
			if model == "" && pr.Model != nil {
				model = *pr.Model
			}
		}
	}
	if pid == nil {
		pid = st.DefaultProviderID
	}
	if pid == nil {
		// Nothing chosen: the platform's default key and model.
		def, err := s.PlatformDefaults(ctx)
		if err != nil {
			return route{model: model}, err
		}
		if model == "" {
			model = def.DefaultModel
		}
		return s.platformRoute(ctx, "", def.DefaultProviderID, model, "")
	}
	if model == "" && st.DefaultProviderID != nil && *pid == *st.DefaultProviderID {
		model = st.DefaultModel
	}
	p, err := s.q.GetAIProvider(ctx, *pid)
	if errors.Is(err, pgx.ErrNoRows) {
		// One of Messora's keys, or a connection that no longer exists.
		if _, perr := s.q.GetPlatformAIProvider(ctx, *pid); perr == nil {
			return s.platformRoute(ctx, "", pid, model, "")
		}
		return route{model: model}, &Error{Kind: KindNotConfigured, Message: "the chosen AI provider no longer exists"}
	}
	if err != nil {
		return route{model: model}, err
	}
	own := Kind(p.Kind) != KindFake || s.opts.AllowFake
	if plan.Has(entitlements.BYOAIKeys) && p.Enabled && own {
		if model == "" {
			return route{tenant: &p, model: model}, &Error{Kind: KindNotConfigured, Provider: Kind(p.Kind), Message: "no model is chosen for the AI provider"}
		}
		return route{tenant: &p, model: model}, nil
	}
	why := "the company's own key is switched off"
	if !plan.Has(entitlements.BYOAIKeys) {
		why = "own AI keys need a plan with byo_ai_keys"
	}
	return s.platformRoute(ctx, Kind(p.Kind), nil, model, why)
}

// routeConfig opens the route's key.
func (s *Service) routeConfig(rt route) (ProviderConfig, error) {
	if rt.tenant != nil {
		return s.config(*rt.tenant)
	}
	if rt.platform != nil {
		return s.platformConfig(*rt.platform)
	}
	return ProviderConfig{}, &Error{Kind: KindNotConfigured, Message: "no AI provider"}
}

// attachPrices fills each view's price valid now.
func (s *Service) attachPrices(ctx context.Context, out []ModelView) error {
	prices, err := s.q.ListAIPrices(ctx)
	if err != nil {
		return err
	}
	domain := make([]Price, 0, len(prices))
	byID := map[uuid.UUID]db.GetAIPriceRow{}
	for _, p := range prices {
		r := db.GetAIPriceRow(p)
		domain = append(domain, priceFromRow(r))
		byID[r.ID] = r
	}
	now := s.opts.Now()
	for i := range out {
		m := out[i].Model
		if p, ok := SelectPrice(domain, Kind(m.ProviderKind), m.ModelID, now); ok {
			r := byID[p.ID]
			out[i].Price = &r
		}
	}
	return nil
}
