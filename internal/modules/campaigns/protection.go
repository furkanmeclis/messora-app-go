package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Spam protection (Phase 16C 5.1). Meta reports no individual spam
// complaints (docs/PHASE-16C.md §1.3); what it gives are aggregate signals:
// error 131048 (spam rate limit), 131049 (ecosystem limit) and 131050
// (the user stopped marketing), the number's quality rating, template
// pauses and quality, and account restrictions. Running campaigns are
// checked against the company's thresholds when one of these arrives (and
// every few minutes by the reconciler); a tripped threshold pauses the
// campaign with reason spam_protection and the values that tripped it.

// PauseReasonSpam is the pause reason of spam protection.
const PauseReasonSpam = "spam_protection"

// Protection rules (the "rule" of a spam_protection pause).
const (
	RuleSpamRateLimit      = "error_131048"
	RuleAccountRestriction = "account_restriction"
	RuleEcosystemRate      = "ecosystem_rate"
	RuleOptOutRate         = "opt_out_rate"
	RuleNumberQuality      = "number_quality"
	RuleTemplatePaused     = "template_paused"
	RuleTemplateQuality    = "template_quality"
)

// Quality stop levels.
const (
	QualityStopRed    = "red"
	QualityStopYellow = "yellow"
	QualityStopOff    = "off"
)

const analyticsSettingsKey = "campaign_protection"

// AnalyticsSettings is the company's "Kampanya koruması" configuration:
// spam thresholds and the attribution window.
type AnalyticsSettings struct {
	// Enabled turns the automatic pauses on (the thresholds below).
	Enabled bool `json:"enabled"`
	// StopOn131048: pause at once when Meta reports its spam rate limit.
	StopOn131048 bool `json:"stop_on_131048"`
	// StopOnAccountRestriction: pause when Meta restricts the account.
	StopOnAccountRestriction bool `json:"stop_on_account_restriction"`
	// Window is how many of the campaign's latest attempts the rates are
	// computed over; MinAttempts is how many it needs before a rate counts.
	Window      int `json:"window"`
	MinAttempts int `json:"min_attempts"`
	// EcosystemRatePct: pause when 131049+131050 exceed this share of the
	// window's attempts. OptOutRatePct: when opt-outs do.
	EcosystemRatePct float64 `json:"ecosystem_rate_pct"`
	OptOutRatePct    float64 `json:"opt_out_rate_pct"`
	// QualityStop: pause when the number's (or the template's) quality is
	// at this level or worse: red, yellow (yellow or red) or off.
	QualityStop string `json:"quality_stop"`
	// StopOnTemplatePaused: pause when Meta pauses or disables the template.
	StopOnTemplatePaused bool `json:"stop_on_template_paused"`
	// AttributionDays bounds how long after a campaign reached a contact a
	// reply or opt-out is credited to it (the next campaign ends it sooner).
	AttributionDays int `json:"attribution_days"`
}

// DefaultAnalyticsSettings returns the initial configuration.
func DefaultAnalyticsSettings() AnalyticsSettings {
	return AnalyticsSettings{Enabled: true, StopOn131048: true, StopOnAccountRestriction: true, Window: 200, MinAttempts: 50,
		EcosystemRatePct: 10, OptOutRatePct: 2, QualityStop: QualityStopRed, StopOnTemplatePaused: true, AttributionDays: 7}
}

// AttributionWindow is the attribution window.
func (s AnalyticsSettings) AttributionWindow() time.Duration {
	return time.Duration(s.AttributionDays) * 24 * time.Hour
}

// Validate checks s.
func (s *AnalyticsSettings) Validate() error {
	switch {
	case s.Window < 20 || s.Window > 10000:
		return apperr.Validation("window", "The window is 20–10000 sends.")
	case s.MinAttempts < 1 || s.MinAttempts > s.Window:
		return apperr.Validation("min_attempts", "The minimum is between 1 and the window.")
	case s.EcosystemRatePct <= 0 || s.EcosystemRatePct > 100 || math.IsNaN(s.EcosystemRatePct):
		return apperr.Validation("ecosystem_rate_pct", "The rate is a percentage above 0.")
	case s.OptOutRatePct <= 0 || s.OptOutRatePct > 100 || math.IsNaN(s.OptOutRatePct):
		return apperr.Validation("opt_out_rate_pct", "The rate is a percentage above 0.")
	case s.QualityStop != QualityStopRed && s.QualityStop != QualityStopYellow && s.QualityStop != QualityStopOff:
		return apperr.Validation("quality_stop", "Choose red, yellow or off.")
	case s.AttributionDays < 1 || s.AttributionDays > 30:
		return apperr.Validation("attribution_days", "The attribution window is 1–30 days.")
	}
	return nil
}

// LoadAnalyticsSettings reads the configuration (defaults when never saved).
func LoadAnalyticsSettings(ctx context.Context, q db.Querier) (AnalyticsSettings, error) {
	out := DefaultAnalyticsSettings()
	raw, err := q.GetSetting(ctx, analyticsSettingsKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return DefaultAnalyticsSettings(), fmt.Errorf("campaign protection: corrupt settings: %w", err)
	}
	if out.Validate() != nil {
		return DefaultAnalyticsSettings(), nil
	}
	return out, nil
}

// SaveAnalyticsSettings validates and stores s (audited by the caller).
func SaveAnalyticsSettings(ctx context.Context, q db.Querier, s AnalyticsSettings, by *uuid.UUID) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, _ := json.Marshal(s)
	if err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: analyticsSettingsKey, Value: b, UpdatedBy: by}); err != nil {
		return err
	}
	invalidateAnalytics()
	return nil
}

var analyticsCache = tenant.NewCache[AnalyticsSettings](10 * time.Second)

// CachedAnalyticsSettings is LoadAnalyticsSettings with a 10 s cache per
// tenant (the webhook path reads it for every inbound message).
func CachedAnalyticsSettings(ctx context.Context, q db.Querier) (AnalyticsSettings, error) {
	if v, ok := analyticsCache.Get(ctx); ok {
		return v, nil
	}
	v, err := LoadAnalyticsSettings(ctx, q)
	if err != nil {
		return v, err
	}
	analyticsCache.Set(ctx, v)
	return v, nil
}

func invalidateAnalytics() { analyticsCache.Clear() }

// Signals is what the protection looks at for one campaign.
type Signals struct {
	// Window: the latest attempts, and how many of them hit 131049/131050
	// or opted out.
	Attempts, Ecosystem, OptedOut int64
	// Saw131048: Meta's spam rate limit was just reported.
	Saw131048 bool
	// AccountRestriction is the account_update event just reported (e.g.
	// ACCOUNT_RESTRICTION), "" for none.
	AccountRestriction string
	// NumberQuality is the number's quality rating (GREEN, YELLOW, RED,
	// UNKNOWN); TemplateStatus and TemplateQuality the template's.
	NumberQuality, TemplateStatus, TemplateQuality string
	// StateRules: whether the state-based rules (quality, template) apply;
	// false after an operator resumed a campaign paused by them, until a
	// new event about them arrives.
	StateRules bool
}

// Trip is a tripped threshold: which rule, and the values behind it.
type Trip struct {
	Rule   string
	Detail map[string]any
}

// Evaluate checks the signals against the thresholds; nil = keep going.
func Evaluate(st AnalyticsSettings, sg Signals) *Trip {
	if !st.Enabled {
		return nil
	}
	if st.StopOn131048 && sg.Saw131048 {
		return &Trip{Rule: RuleSpamRateLimit, Detail: map[string]any{"error_code": "131048"}}
	}
	if st.StopOnAccountRestriction && sg.AccountRestriction != "" {
		return &Trip{Rule: RuleAccountRestriction, Detail: map[string]any{"event": sg.AccountRestriction}}
	}
	if sg.Attempts >= int64(st.MinAttempts) && sg.Attempts > 0 {
		rate := func(n int64) float64 { return math.Round(float64(n)*10000/float64(sg.Attempts)) / 100 }
		if r := rate(sg.Ecosystem); r > st.EcosystemRatePct {
			return &Trip{Rule: RuleEcosystemRate, Detail: map[string]any{"rate_pct": r, "threshold_pct": st.EcosystemRatePct,
				"count": sg.Ecosystem, "attempts": sg.Attempts, "window": st.Window}}
		}
		if r := rate(sg.OptedOut); r > st.OptOutRatePct {
			return &Trip{Rule: RuleOptOutRate, Detail: map[string]any{"rate_pct": r, "threshold_pct": st.OptOutRatePct,
				"count": sg.OptedOut, "attempts": sg.Attempts, "window": st.Window}}
		}
	}
	if !sg.StateRules {
		return nil
	}
	if qualityAtOrBelow(sg.NumberQuality, st.QualityStop) {
		return &Trip{Rule: RuleNumberQuality, Detail: map[string]any{"quality": strings.ToUpper(sg.NumberQuality), "threshold": st.QualityStop}}
	}
	if st.StopOnTemplatePaused {
		switch strings.ToUpper(sg.TemplateStatus) {
		case "PAUSED", "DISABLED":
			return &Trip{Rule: RuleTemplatePaused, Detail: map[string]any{"status": strings.ToUpper(sg.TemplateStatus)}}
		}
	}
	if qualityAtOrBelow(sg.TemplateQuality, st.QualityStop) {
		return &Trip{Rule: RuleTemplateQuality, Detail: map[string]any{"quality": strings.ToUpper(sg.TemplateQuality), "threshold": st.QualityStop}}
	}
	return nil
}

// qualityAtOrBelow reports whether a quality rating is as bad as the stop
// level or worse (Meta's GREEN > YELLOW > RED; "LOW"/"FLAGGED" count as red).
func qualityAtOrBelow(quality, stop string) bool {
	q := strings.ToUpper(strings.TrimSpace(quality))
	if q == "LOW" || q == "FLAGGED" {
		q = "RED"
	}
	if q == "MEDIUM" {
		q = "YELLOW"
	}
	switch stop {
	case QualityStopRed:
		return q == "RED"
	case QualityStopYellow:
		return q == "RED" || q == "YELLOW"
	}
	return false
}

// ProtectEvent is what triggered an evaluation.
type ProtectEvent struct {
	// ErrorCode of a campaign message that just failed or was deferred.
	ErrorCode string
	// Inbound / OptOut: a reply or an opt-out was just attributed.
	Inbound, OptOut bool
	// AccountEvent: an account_update event (e.g. ACCOUNT_RESTRICTION).
	AccountEvent string
	// NumberQuality / TemplateStatus / TemplateQuality reported by a webhook
	// just now (they override what is stored, and re-enable the state rules).
	NumberQuality, TemplateStatus, TemplateQuality string
}

func (e ProtectEvent) stateEvent() bool {
	return e.NumberQuality != "" || e.TemplateStatus != "" || e.TemplateQuality != ""
}

// accountRestrictions are the account_update events that stop campaigns.
var accountRestrictions = map[string]bool{"ACCOUNT_RESTRICTION": true, "ACCOUNT_VIOLATION": true, "DISABLED_UPDATE": true,
	"ACCOUNT_DELETED": true, "ACCOUNT_OFFBOARDED": true}

// Protect evaluates campaign id (when it is sending) after ev and pauses it
// with reason spam_protection if a threshold tripped. It runs in the
// caller's transaction.
func (s *Service) Protect(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, ev ProtectEvent) (*Trip, error) {
	c, err := q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if c.Status != "running" && c.Status != "preparing" && c.Status != "paused_limit" {
		return nil, nil
	}
	st, err := CachedAnalyticsSettings(ctx, q)
	if err != nil || !st.Enabled {
		return nil, err
	}
	sg := Signals{Saw131048: ev.ErrorCode == "131048", NumberQuality: ev.NumberQuality,
		TemplateStatus: ev.TemplateStatus, TemplateQuality: ev.TemplateQuality}
	if accountRestrictions[strings.ToUpper(ev.AccountEvent)] {
		sg.AccountRestriction = strings.ToUpper(ev.AccountEvent)
	}
	// Rates only move with sends, replies and opt-outs: read the window
	// then (and on the periodic pass).
	periodic := ev == ProtectEvent{}
	if periodic || ev.ErrorCode == "131049" || ev.ErrorCode == "131050" || ev.OptOut || ev.Inbound {
		w, err := q.SpamWindow(ctx, db.SpamWindowParams{CampaignID: id, N: int32(st.Window), Since: resumedAfterSpam(ctx, q, id)}) //nolint:gosec // bounded setting
		if err != nil {
			return nil, err
		}
		sg.Attempts, sg.Ecosystem, sg.OptedOut = w.Attempts, w.Ecosystem, w.OptedOut
	}
	// State rules: on their own events, and on the periodic pass unless an
	// operator resumed the campaign after such a pause.
	sg.StateRules = ev.stateEvent() || (periodic && resumedAfterSpam(ctx, q, id) == nil)
	if periodic && sg.StateRules {
		if pn, err := q.GetPhoneNumber(ctx, c.PhoneNumberID); err == nil {
			sg.NumberQuality = pn.QualityRating
		}
		if t, err := q.CampaignTemplate(ctx, id); err == nil {
			sg.TemplateStatus, sg.TemplateQuality = t.Status, t.QualityScore
		}
	}
	trip := Evaluate(st, sg)
	if trip == nil {
		return nil, nil
	}
	detail := map[string]any{"reason": PauseReasonSpam, "rule": trip.Rule}
	for k, v := range trip.Detail {
		detail[k] = v
	}
	if _, err := s.transition(ctx, q, tx, id, "paused", []string{"running", "preparing", "paused_limit"}, "paused", detail, nil, nil); err != nil {
		if e, ok := apperr.As(err); ok && e.Code == "INVALID_TRANSITION" {
			return nil, nil
		}
		return nil, err
	}
	s.log.WarnContext(ctx, "campaign paused by spam protection", "campaign_id", id, "rule", trip.Rule)
	return trip, nil
}

// resumedAfterSpam is when an operator last resumed the campaign from a
// spam_protection pause (nil = never): the rates count attempts after it.
func resumedAfterSpam(ctx context.Context, q *db.Queries, id uuid.UUID) *time.Time {
	t, err := q.LastSpamResume(ctx, id)
	if err != nil || t.Unix() <= 0 {
		return nil
	}
	return &t
}

// MessageSignal is told, inside the send or webhook transaction, that a
// campaign message ended with an error (send-time failures, deferrals and
// failed statuses): 131048/131049/131050 are evaluated.
func (s *Service) MessageSignal(ctx context.Context, tx pgx.Tx, q *db.Queries, m db.Message) error {
	if m.CampaignID == nil || m.ErrorCode == nil {
		return nil
	}
	switch *m.ErrorCode {
	case "131048", "131049", "131050":
	default:
		return nil
	}
	_, err := s.Protect(ctx, tx, q, *m.CampaignID, ProtectEvent{ErrorCode: *m.ErrorCode})
	return err
}

// AccountSignal is told, inside the webhook transaction, about an account
// webhook: restrictions stop every sending campaign of the account,
// number quality the number's campaigns, template status and quality the
// template's campaigns.
func (s *Service) AccountSignal(ctx context.Context, tx pgx.Tx, q *db.Queries, sig AccountSignalInfo) error {
	cs, err := q.ActiveCampaignsOfAccount(ctx, sig.AccountID)
	if err != nil {
		return err
	}
	for _, c := range cs {
		ev := ProtectEvent{AccountEvent: sig.AccountEvent}
		if sig.PhoneNumberID == nil || *sig.PhoneNumberID == c.PhoneNumberID {
			ev.NumberQuality = sig.NumberQuality
		}
		if sig.TemplateName != "" && sig.TemplateName == c.TemplateName && (sig.TemplateLanguage == "" || sig.TemplateLanguage == c.TemplateLanguage) {
			ev.TemplateStatus, ev.TemplateQuality = sig.TemplateStatus, sig.TemplateQuality
		}
		if ev == (ProtectEvent{}) {
			continue
		}
		if _, err := s.Protect(ctx, tx, q, c.ID, ev); err != nil {
			return err
		}
	}
	return nil
}

// AccountSignalInfo describes an account webhook for AccountSignal.
type AccountSignalInfo struct {
	AccountID                       uuid.UUID
	PhoneNumberID                   *uuid.UUID
	AccountEvent                    string
	NumberQuality                   string
	TemplateName, TemplateLanguage  string
	TemplateStatus, TemplateQuality string
}

// ProtectAccount evaluates the sending campaigns of an account (after the
// numbers' signals were synced from Meta).
func (s *Service) ProtectAccount(ctx context.Context, accountID uuid.UUID) error {
	cs, err := s.q.ActiveCampaignsOfAccount(ctx, accountID)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
			_, err := s.Protect(ctx, tx, q, c.ID, ProtectEvent{})
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
