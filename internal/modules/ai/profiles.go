package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
)

func profileFromRow(r db.AiProfile, phone string) Profile {
	p := Profile{ID: r.ID, Name: r.Name, PhoneNumberID: r.PhoneNumberID, PhoneDisplay: phone, SystemPrompt: r.SystemPrompt,
		ProviderID: coalesceID(r.ProviderID, r.PlatformProviderID), Model: r.Model, AutoReply: r.AutoReply, DisclosureMode: r.DisclosureMode, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	_ = json.Unmarshal(r.Handoff, &p.Handoff)
	p.Handoff.OnRequest = true
	if p.Handoff.OnCriticalIntents == nil {
		p.Handoff.OnCriticalIntents = []string{}
	}
	_ = json.Unmarshal(r.DataCollection, &p.DataCollection)
	if p.DataCollection.Fields == nil {
		p.DataCollection.Fields = []DataField{}
	}
	_ = json.Unmarshal(r.AllowedActions, &p.AllowedActions)
	if p.AllowedActions == nil {
		p.AllowedActions = []string{}
	}
	return p
}

// ListProfiles returns every AI profile.
func (s *Service) ListProfiles(ctx context.Context) ([]Profile, error) {
	rows, err := s.q.ListAIProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Profile, len(rows))
	for i, r := range rows {
		out[i] = profileFromRow(db.AiProfile{ID: r.ID, Name: r.Name, PhoneNumberID: r.PhoneNumberID, SystemPrompt: r.SystemPrompt,
			ProviderID: r.ProviderID, Model: r.Model, AutoReply: r.AutoReply, Handoff: r.Handoff, DataCollection: r.DataCollection,
			AllowedActions: r.AllowedActions, DisclosureMode: r.DisclosureMode, Status: r.Status, CreatedBy: r.CreatedBy,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}, deref(r.PhoneDisplay))
	}
	return out, nil
}

// GetProfile returns one profile.
func (s *Service) GetProfile(ctx context.Context, id uuid.UUID) (Profile, error) {
	r, err := s.q.GetAIProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	return profileFromRow(r, ""), nil
}

// ProfileForNumber returns the profile bound to a phone number (by its
// Messora id), or ErrNotFound.
func (s *Service) ProfileForNumber(ctx context.Context, phoneNumberID uuid.UUID) (Profile, error) {
	r, err := s.q.GetAIProfileForNumber(ctx, &phoneNumberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	return profileFromRow(r, ""), nil
}

// checkRefs verifies the number, provider and automations a profile names.
func (s *Service) checkRefs(ctx context.Context, q *db.Queries, p Profile) error {
	if p.PhoneNumberID != nil {
		ok, err := q.PhoneNumberExists(ctx, *p.PhoneNumberID)
		if err != nil {
			return err
		}
		if !ok {
			return &FieldError{"phone_number_id", "Unknown phone number."}
		}
	}
	if p.ProviderID != nil {
		kind, err := s.providerKind(ctx, q, *p.ProviderID)
		if err != nil {
			return err
		}
		if kind == "" {
			return &FieldError{"provider_id", "Unknown provider."}
		}
		if kind == KindFake && !s.opts.AllowFake {
			return &FieldError{"provider_id", "The fake provider is not available here."}
		}
	} else if p.Model != nil {
		st, err := s.Settings(ctx)
		if err != nil {
			return err
		}
		if st.DefaultProviderID == nil {
			return &FieldError{"model", "Choose the provider of the model, or set a default provider first."}
		}
	}
	for _, a := range p.AllowedActions {
		if id, ok := strings.CutPrefix(a, "automation:"); ok {
			exists, err := q.AutomationExists(ctx, uuid.MustParse(id))
			if err != nil {
				return err
			}
			if !exists {
				return &FieldError{"allowed_actions", "Unknown automation " + id + "."}
			}
		}
	}
	return nil
}

func profileJSON(p Profile) (handoff, data, actions []byte) {
	handoff, _ = json.Marshal(p.Handoff)
	data, _ = json.Marshal(p.DataCollection)
	actions, _ = json.Marshal(p.AllowedActions)
	return
}

// CreateProfile stores a new profile. A number has at most one profile.
func (s *Service) CreateProfile(ctx context.Context, p Profile, actor *uuid.UUID) (Profile, error) {
	if p.DisclosureMode == "" {
		p.DisclosureMode = DiscloseFirstMessage
	}
	if p.Status == "" {
		p.Status = ProfileActive
	}
	if err := p.normalize(); err != nil {
		return Profile{}, err
	}
	var out Profile
	err := s.tx(ctx, func(q *db.Queries) error {
		// AI agents are a plan feature with a count (Phase 17 WP8).
		plan, err := s.plan(ctx)
		if err != nil {
			return err
		}
		if err := plan.Require(entitlements.AIAgents); err != nil {
			return err
		}
		if err := entitlements.Check(ctx, q, s.plans, entitlements.AIAgentCount, func() (int64, error) { return q.CountAIProfilesForPlan(ctx) }); err != nil {
			return err
		}
		if err := s.checkRefs(ctx, q, p); err != nil {
			return err
		}
		h, d, a := profileJSON(p)
		own, platform, err := s.splitProvider(ctx, q, p.ProviderID)
		if err != nil {
			return err
		}
		r, err := q.InsertAIProfile(ctx, db.InsertAIProfileParams{Name: p.Name, PhoneNumberID: p.PhoneNumberID, SystemPrompt: p.SystemPrompt,
			ProviderID: own, PlatformProviderID: platform, Model: p.Model, AutoReply: p.AutoReply, Handoff: h, DataCollection: d, AllowedActions: a,
			DisclosureMode: p.DisclosureMode, Status: p.Status, CreatedBy: actor})
		if isUnique(err) {
			return &FieldError{"phone_number_id", "This number already has an AI profile."}
		}
		if err != nil {
			return err
		}
		out = profileFromRow(r, "")
		return audit.Record(ctx, q, audit.Entry{Action: "ai.profile.create", EntityType: "ai_profile", EntityID: &r.ID,
			Changes: profileAudit(Profile{}, out)})
	})
	return out, err
}

// UpdateProfile replaces a profile's configuration.
func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, p Profile) (Profile, error) {
	cur, err := s.GetProfile(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	if err := p.normalize(); err != nil {
		return Profile{}, err
	}
	var out Profile
	err = s.tx(ctx, func(q *db.Queries) error {
		if err := s.checkRefs(ctx, q, p); err != nil {
			return err
		}
		h, d, a := profileJSON(p)
		own, platform, err := s.splitProvider(ctx, q, p.ProviderID)
		if err != nil {
			return err
		}
		r, err := q.UpdateAIProfile(ctx, db.UpdateAIProfileParams{ID: id, Name: p.Name, PhoneNumberID: p.PhoneNumberID,
			SystemPrompt: p.SystemPrompt, ProviderID: own, PlatformProviderID: platform, Model: p.Model, AutoReply: p.AutoReply, Handoff: h,
			DataCollection: d, AllowedActions: a, DisclosureMode: p.DisclosureMode, Status: p.Status})
		if isUnique(err) {
			return &FieldError{"phone_number_id", "This number already has an AI profile."}
		}
		if err != nil {
			return err
		}
		out = profileFromRow(r, "")
		return audit.Record(ctx, q, audit.Entry{Action: "ai.profile.update", EntityType: "ai_profile", EntityID: &id,
			Changes: profileAudit(cur, out)})
	})
	return out, err
}

// DeleteProfile removes a profile.
func (s *Service) DeleteProfile(ctx context.Context, id uuid.UUID) error {
	cur, err := s.GetProfile(ctx, id)
	if err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.DeleteAIProfile(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.profile.delete", EntityType: "ai_profile", EntityID: &id,
			Changes: map[string]any{"name": cur.Name}})
	})
}

// profileAudit lists changed settings; the prompt is summarised by length.
func profileAudit(a, b Profile) map[string]any {
	ch := map[string]any{}
	set := func(k string, x, y any) {
		xb, _ := json.Marshal(x)
		yb, _ := json.Marshal(y)
		if string(xb) != string(yb) {
			ch[k] = []any{x, y}
		}
	}
	set("name", a.Name, b.Name)
	set("phone_number_id", a.PhoneNumberID, b.PhoneNumberID)
	set("provider_id", a.ProviderID, b.ProviderID)
	set("model", a.Model, b.Model)
	set("auto_reply", a.AutoReply, b.AutoReply)
	set("handoff", a.Handoff, b.Handoff)
	set("data_collection", a.DataCollection, b.DataCollection)
	set("allowed_actions", a.AllowedActions, b.AllowedActions)
	set("disclosure_mode", a.DisclosureMode, b.DisclosureMode)
	set("status", a.Status, b.Status)
	if a.SystemPrompt != b.SystemPrompt {
		ch["system_prompt_chars"] = []any{len([]rune(a.SystemPrompt)), len([]rune(b.SystemPrompt))}
	}
	return ch
}

// TestResult is the answer of the profile test console.
type TestResult struct {
	Result
	Reply        *Reply
	Instructions string
}

// TestProfile runs a sample conversation through a profile exactly as a
// live AI turn would (instructions, reply schema), recorded with purpose
// "test". Nothing is sent to any customer.
func (s *Service) TestProfile(ctx context.Context, id uuid.UUID, msgs []Message, lang Language) (TestResult, error) {
	p, err := s.GetProfile(ctx, id)
	if err != nil {
		return TestResult{}, err
	}
	st, err := s.Settings(ctx)
	if err != nil {
		return TestResult{}, err
	}
	var fields []DataField
	if p.DataCollection.Enabled {
		fields = p.DataCollection.Fields
	}
	instr := Instructions(p, st, lang)
	res, err := s.Generate(ctx, Call{Purpose: PurposeTest, ProfileID: &p.ID, Request: Request{System: instr, Messages: msgs,
		Output: &OutputSchema{Name: ReplySchemaName, Schema: ReplySchema(fields)}, MaxTokens: 2048}})
	out := TestResult{Result: res, Instructions: instr}
	if err != nil {
		return out, err
	}
	if len(res.Structured) > 0 {
		if r, err := DecodeReply(res.Structured); err == nil {
			out.Reply = &r
		}
	}
	return out, nil
}

func coalesceID(a, b *uuid.UUID) *uuid.UUID {
	if a != nil {
		return a
	}
	return b
}

// splitProvider tells a company connection (own) from one of Messora's keys
// (platform); checkRefs has verified that id is one of the two.
func (s *Service) splitProvider(ctx context.Context, q *db.Queries, id *uuid.UUID) (own, platform *uuid.UUID, err error) {
	if id == nil {
		return nil, nil, nil
	}
	if _, err := q.GetAIProvider(ctx, *id); err == nil {
		return id, nil, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, err
	}
	return nil, id, nil
}
