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
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// PausedStatuses are the statuses a campaign can be resumed from. Every
// status starting with "paused" is a pause; paused_limit waits for the
// portfolio's messaging limit (its queued messages still go out), the
// others hold queued messages. Pause reasons (manual, offer_expired,
// messaging_limit, and later ones such as spam protection) are kept in
// paused_reason and the pause history. Campaigns paused by the former
// licence check keep their stored reason, license_restricted.
var PausedStatuses = []string{"paused", "paused_limit"}

// IsPaused reports whether status is a pause.
func IsPaused(status string) bool { return strings.HasPrefix(status, "paused") }

// Pause reasons set by the engine itself.
const (
	PauseReasonManual = "manual"
	PauseReasonSystem = "system"
	PauseReasonLimit  = "messaging_limit"
)

// audited are the campaign events written to the audit log (R10).
var audited = map[string]bool{"started": true, "scheduled": true, "paused": true, "paused_limit": true,
	"resumed": true, "cancelled": true}

// recordPauseChange keeps the pause fields and history in step with a
// status change made by transition: entering a pause stores the reason,
// who, the automatic resume time and a snapshot of the counts; leaving it
// closes the history row (with what the operator changed) and clears the
// fields.
func (s *Service) recordPauseChange(ctx context.Context, q *db.Queries, c db.Campaign, kind string, detail map[string]any, actor *uuid.UUID) (db.Campaign, error) {
	if IsPaused(c.Status) {
		reason, _ := detail["reason"].(string)
		if reason == "" {
			reason = PauseReasonSystem
			if actor != nil {
				reason = PauseReasonManual
			}
		}
		var resumeAt *time.Time
		if t, ok := detail["resume_at"].(time.Time); ok {
			resumeAt = &t
		}
		c, err := q.SetCampaignPauseInfo(ctx, db.SetCampaignPauseInfoParams{ID: c.ID, Reason: &reason, PausedBy: actor, ResumeAt: resumeAt})
		if err != nil {
			return c, err
		}
		snap, err := snapshot(ctx, q, c.ID)
		if err != nil {
			return c, err
		}
		// A pause replacing another (manual pause while waiting for the
		// limit) ends the earlier one.
		if err := q.CloseCampaignPause(ctx, db.CloseCampaignPauseParams{CampaignID: c.ID, ResumedBy: actor,
			Changes: []byte(`{"superseded_by":"` + c.Status + `"}`)}); err != nil {
			return c, err
		}
		sj, _ := json.Marshal(snap)
		dj, _ := json.Marshal(detail)
		if dj == nil || string(dj) == "null" {
			dj = []byte("{}")
		}
		_, err = q.InsertCampaignPause(ctx, db.InsertCampaignPauseParams{CampaignID: c.ID, Status: c.Status, Reason: reason, Detail: dj,
			PausedBy: actor, ResumeAt: resumeAt, Snapshot: sj})
		return c, err
	}
	if c.PausedAt == nil && c.ResumeAt == nil {
		return c, nil
	}
	if kind == "resumed" {
		var changes []byte
		if ch, ok := detail["changes"]; ok {
			changes, _ = json.Marshal(ch)
		}
		if err := q.CloseCampaignPause(ctx, db.CloseCampaignPauseParams{CampaignID: c.ID, ResumedBy: actor, Changes: changes}); err != nil {
			return c, err
		}
	}
	return q.ClearCampaignPauseInfo(ctx, c.ID)
}

// snapshot is a campaign's counts at a moment: recipients by state,
// messages by status and recipients by unreached class (class_<name>).
func snapshot(ctx context.Context, q *db.Queries, id uuid.UUID) (map[string]int64, error) {
	out, err := stats(ctx, q, id)
	if err != nil {
		return nil, err
	}
	cls, err := classCounts(ctx, q, id)
	if err != nil {
		return nil, err
	}
	for k, v := range cls {
		out["class_"+k] = v
	}
	return out, nil
}

// Settings are what may change while a campaign is in progress.
type Settings struct {
	Name        *string
	Priority    *string
	MaxMPS      *float64
	ClearMaxMPS bool
}

func (st Settings) validate() error {
	if st.Name != nil && (strings.TrimSpace(*st.Name) == "" || len([]rune(*st.Name)) > 120) {
		return apperr.Validation("name", "Name is 1–120 characters.")
	}
	return validateSettings(st.Priority, st.MaxMPS)
}

func validateSettings(priority *string, maxMPS *float64) error {
	if priority != nil {
		if _, ok := map[string]bool{"high": true, "normal": true, "low": true}[*priority]; !ok {
			return apperr.Validation("priority", "Priority is high, normal or low.")
		}
	}
	if maxMPS != nil && (*maxMPS <= 0 || *maxMPS > 1000 || math.IsNaN(*maxMPS)) {
		return apperr.Validation("max_mps", "The rate cap is between 0 and 1000 messages per second.")
	}
	return nil
}

// applySettings stores changed settings and returns them as field → [old,
// new] (empty when nothing changed).
func (s *Service) applySettings(ctx context.Context, q *db.Queries, cur db.Campaign, st Settings) (map[string]any, error) {
	changes := map[string]any{}
	if st.Name != nil && strings.TrimSpace(*st.Name) != cur.Name {
		n := strings.TrimSpace(*st.Name)
		st.Name = &n
		changes["name"] = []any{cur.Name, n}
	} else {
		st.Name = nil
	}
	if st.Priority != nil && *st.Priority != cur.Priority {
		changes["priority"] = []any{cur.Priority, *st.Priority}
	} else {
		st.Priority = nil
	}
	switch {
	case st.ClearMaxMPS && cur.MaxMps != nil:
		changes["max_mps"] = []any{*cur.MaxMps, nil}
	case st.MaxMPS != nil && (cur.MaxMps == nil || *cur.MaxMps != *st.MaxMPS):
		changes["max_mps"] = []any{cur.MaxMps, *st.MaxMPS}
	default:
		st.MaxMPS, st.ClearMaxMPS = nil, false
	}
	if len(changes) == 0 {
		return changes, nil
	}
	if _, err := q.UpdateCampaignSettings(ctx, db.UpdateCampaignSettingsParams{ID: cur.ID, Name: st.Name, Priority: st.Priority,
		MaxMps: st.MaxMPS, ClearMaxMps: st.ClearMaxMPS}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.Conflict("CAMPAIGN_FINISHED", "A finished campaign cannot be changed.")
		}
		return nil, err
	}
	return changes, nil
}

// UpdateSettings changes name, priority or rate cap of a campaign that has
// not finished (also while it runs: the scheduler picks the new weight up on
// its next tick). Priority changes are audited.
func (s *Service) UpdateSettings(ctx context.Context, id uuid.UUID, st Settings, actor *uuid.UUID) (db.Campaign, error) {
	if err := st.validate(); err != nil {
		return db.Campaign{}, err
	}
	var c db.Campaign
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		cur, err := q.LockCampaign(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Campaign not found.")
		}
		if err != nil {
			return err
		}
		if cur.Status == "completed" || cur.Status == "cancelled" || cur.Status == "failed" {
			return apperr.Conflict("CAMPAIGN_FINISHED", "A finished campaign cannot be changed.")
		}
		changes, err := s.applySettings(ctx, q, cur, st)
		if err != nil {
			return err
		}
		if c, err = q.GetCampaign(ctx, id); err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		dj, _ := json.Marshal(changes)
		if err := q.InsertCampaignEvent(ctx, db.InsertCampaignEventParams{CampaignID: id, Kind: "settings_changed", Detail: dj, ActorID: actor}); err != nil {
			return err
		}
		action := "campaign.settings_update"
		if _, ok := changes["priority"]; ok {
			action = "campaign.priority_change"
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: action, EntityType: "campaign", EntityID: &id, Changes: changes}); err != nil {
			return err
		}
		return s.publish(ctx, tx, q, c)
	})
	return c, err
}

// Pauses lists a campaign's pause history, newest first.
func (s *Service) Pauses(ctx context.Context, id uuid.UUID) ([]db.CampaignPause, error) {
	if _, err := s.q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Campaign not found.")
	}
	return s.q.ListCampaignPauses(ctx, id)
}

// ResumePreview is what resuming a paused campaign would do.
type ResumePreview struct {
	Campaign db.Campaign
	// Remaining recipients (pending, waiting for a resend, held) and what
	// the consent rules decide for them now.
	Remaining, Eligible, Blocked, Warned int64
	BlockedReasons, WarnedReasons        []PreflightReason
	// ShareMPS is the rate the campaign would get now; EstimatedSeconds the
	// time to send the eligible recipients at that rate (nil when the rate
	// is 0).
	ShareMPS         float64
	EstimatedSeconds *int64
	Capacity         NumberCapacity
	// Limit is the portfolio's messaging limit and its use (Limit 0 = none).
	Limit limitState
}

// PreviewResume computes the pre-resume summary of a paused campaign with
// its current settings, or with the ones in st.
func (s *Service) PreviewResume(ctx context.Context, id uuid.UUID, st Settings) (ResumePreview, error) {
	if err := st.validate(); err != nil {
		return ResumePreview{}, err
	}
	c, err := s.q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResumePreview{}, apperr.NotFound("Campaign not found.")
	}
	if err != nil {
		return ResumePreview{}, err
	}
	if !IsPaused(c.Status) {
		return ResumePreview{}, apperr.Conflict("INVALID_TRANSITION", fmt.Sprintf("A %s campaign cannot be resumed.", c.Status))
	}
	if st.Priority != nil {
		c.Priority = *st.Priority
	}
	if st.ClearMaxMPS {
		c.MaxMps = nil
	} else if st.MaxMPS != nil {
		c.MaxMps = st.MaxMPS
	}
	rules, err := rulesFor(ctx, s.q, c.PhoneNumberID, c.TemplateName, c.TemplateLanguage)
	if err != nil {
		return ResumePreview{}, err
	}
	groups, err := s.q.RemainingConsentGroups(ctx, id)
	if err != nil {
		return ResumePreview{}, err
	}
	out := ResumePreview{Campaign: c}
	blocked, warned := map[string]*PreflightReason{}, map[string]*PreflightReason{}
	set := time.Unix(0, 0).UTC()
	flag := func(on bool) *time.Time {
		if on {
			return &set
		}
		return nil
	}
	for _, g := range groups {
		out.Remaining += g.N
		d := consent.Decide(consent.StateOf(db.Contact{MarketingConsent: g.MarketingConsent, ServiceConsent: g.ServiceConsent,
			OptedOutAt: flag(g.OptedOut), MetaMarketingStoppedAt: flag(g.Stopped), UndeliverableAt: flag(g.Undeliverable)}), rules.category, rules.policy)
		switch d.Verdict {
		case consent.Block:
			out.Blocked += g.N
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
	capa, err := s.numberCapacity(ctx, s.q, c.PhoneNumberID)
	if err != nil {
		return out, err
	}
	out.Capacity = capa
	cs, err := s.contenders(ctx, s.q, c)
	if err != nil {
		return out, err
	}
	out.ShareMPS = shareOf(capa.Campaigns, cs, c)
	if out.ShareMPS > 0 {
		sec := int64(math.Ceil(float64(out.Eligible) / out.ShareMPS))
		out.EstimatedSeconds = &sec
	}
	lim, err := s.limitState(ctx, s.q, c.PhoneNumberID, false)
	if err != nil {
		return out, err
	}
	out.Limit = lim
	return out, nil
}
