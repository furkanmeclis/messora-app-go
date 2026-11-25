package campaigns

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// sendRules is what the consent check needs for one campaign: the message
// category of its template and the company policy.
type sendRules struct {
	category string
	policy   consent.Policy
}

// rulesFor reads the template category and the consent policy. A template
// that is gone counts as marketing (the strictest).
func rulesFor(ctx context.Context, q *db.Queries, phoneNumberID uuid.UUID, name, language string) (sendRules, error) {
	r := sendRules{category: consent.Marketing}
	pn, err := q.GetPhoneNumber(ctx, phoneNumberID)
	if err == nil {
		t, terr := q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: name, Language: language})
		switch {
		case terr == nil:
			r.category = consent.MessageCategory(t.Category)
		case !errors.Is(terr, pgx.ErrNoRows):
			return r, terr
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return r, err
	}
	st, err := consent.CachedSettings(ctx, q)
	if err != nil {
		return r, err
	}
	r.policy = st.Policy()
	return r, nil
}

func (r sendRules) decide(c db.Contact) consent.Decision {
	return consent.Decide(consent.StateOf(c), r.category, r.policy)
}

// PreflightReason counts one reason.
type PreflightReason struct {
	Reason string
	Count  int64
	Hard   bool
}

// Preflight is what the consent rules decide for an audience.
type Preflight struct {
	Category         string
	UnknownMarketing string
	Total            int64
	Eligible         int64 // allowed, including warned
	Warned           int64
	Blocked          int64
	HardBlocked      int64
	BlockedReasons   []PreflightReason
	WarnedReasons    []PreflightReason
	// ByFailureClass: for a previous-campaign audience, the source
	// campaign's customers by why they were not reached.
	ByFailureClass map[string]int64
}

// Preflight counts an audience by consent class for a template, before a
// campaign is created or started. Nothing is stored.
func (s *Service) Preflight(ctx context.Context, a Audience, phoneNumberID *uuid.UUID, name, language string) (Preflight, error) {
	if err := s.validate(ctx, Input{Audience: &a, PhoneNumberID: phoneNumberID}); err != nil {
		return Preflight{}, err
	}
	var pn db.PhoneNumber
	var err error
	if phoneNumberID != nil {
		pn, err = s.q.GetPhoneNumber(ctx, *phoneNumberID)
	} else {
		pn, err = s.q.GetDefaultPhoneNumber(ctx)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Preflight{}, apperr.Conflict("NO_PHONE_NUMBER", "Connect a WhatsApp account before creating campaigns.")
	}
	if err != nil {
		return Preflight{}, err
	}
	if _, err := s.q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: name, Language: language}); errors.Is(err, pgx.ErrNoRows) {
		return Preflight{}, apperr.Validation("template_name", "The template does not exist on the sender's account; sync templates first.")
	} else if err != nil {
		return Preflight{}, err
	}
	rules, err := rulesFor(ctx, s.q, pn.ID, name, language)
	if err != nil {
		return Preflight{}, err
	}
	f, err := s.filter(ctx, a)
	if err != nil {
		return Preflight{}, err
	}
	groups, err := s.contacts.ConsentGroups(ctx, f)
	if err != nil {
		return Preflight{}, err
	}
	out := Preflight{Category: rules.category, UnknownMarketing: consent.UnknownAllow}
	if rules.policy.BlockUnknownMarketing {
		out.UnknownMarketing = consent.UnknownBlock
	}
	blocked := map[string]*PreflightReason{}
	warned := map[string]*PreflightReason{}
	for _, g := range groups {
		out.Total += g.N
		d := consent.Decide(g.State, rules.category, rules.policy)
		switch d.Verdict {
		case consent.Block:
			out.Blocked += g.N
			if d.Hard {
				out.HardBlocked += g.N
			}
			add(blocked, d, g.N)
		case consent.Warn:
			out.Eligible += g.N
			out.Warned += g.N
			add(warned, d, g.N)
		default:
			out.Eligible += g.N
		}
	}
	out.BlockedReasons, out.WarnedReasons = sorted(blocked), sorted(warned)
	if a.PreviousCampaign != nil {
		if out.ByFailureClass, err = classCounts(ctx, s.q, a.PreviousCampaign.CampaignID); err != nil {
			return out, err
		}
	}
	return out, nil
}

func add(m map[string]*PreflightReason, d consent.Decision, n int64) {
	r, ok := m[d.Reason]
	if !ok {
		r = &PreflightReason{Reason: d.Reason, Hard: d.Hard}
		m[d.Reason] = r
	}
	r.Count += n
}

// sorted lists the reasons, largest first.
func sorted(m map[string]*PreflightReason) []PreflightReason {
	out := make([]PreflightReason, 0, len(m))
	for _, r := range m {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}
