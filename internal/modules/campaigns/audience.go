package campaigns

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Why a recipient of a campaign was not reached (SQL function
// campaign_recipient_class). The first six can be targeted again by the
// "previous campaign's unreached customers" audience; the others are hard
// blocks and never selectable.
const (
	ClassTemporary            = "temporary"
	ClassRejectedByMeta       = "rejected_by_meta"
	ClassEcosystemLimit       = "ecosystem_limit"
	ClassUndeliverable        = "undeliverable"
	ClassPermanent            = "permanent"
	ClassNeverSent            = "never_sent"
	ClassUserStoppedMarketing = "user_stopped_marketing"
	ClassBlockedByConsent     = "blocked_by_consent"
	// ClassReached counts recipients that were reached (or possibly
	// reached: sent, delivered, read, unknown) in summaries.
	ClassReached = "reached"
)

// SelectableClasses can be chosen for a previous-campaign audience.
var SelectableClasses = []string{ClassTemporary, ClassRejectedByMeta, ClassEcosystemLimit, ClassUndeliverable, ClassPermanent, ClassNeverSent}

// PreviousCampaign is the audience "customers campaign CampaignID did not
// reach, for one of Classes". It is computed when the new campaign starts,
// one recipient per contact; consent is checked again as for any audience.
type PreviousCampaign struct {
	CampaignID uuid.UUID `json:"campaign_id"`
	Classes    []string  `json:"classes"`
}

func (p *PreviousCampaign) validate() error {
	if p.CampaignID == uuid.Nil {
		return apperr.Validation("audience.previous_campaign.campaign_id", "Choose the earlier campaign.")
	}
	if len(p.Classes) == 0 {
		return apperr.Validation("audience.previous_campaign.classes", "Choose at least one reason the customers were not reached.")
	}
	seen := map[string]bool{}
	for _, c := range p.Classes {
		if c == ClassUserStoppedMarketing || c == ClassBlockedByConsent {
			return apperr.Validation("audience.previous_campaign.classes",
				fmt.Sprintf("%q cannot be targeted again: the customer stopped marketing or consent blocks them.", c))
		}
		if !slices.Contains(SelectableClasses, c) {
			return apperr.Validation("audience.previous_campaign.classes", fmt.Sprintf("Unknown class %q; use one of %s.", c, strings.Join(SelectableClasses, ", ")))
		}
		if seen[c] {
			return apperr.Validation("audience.previous_campaign.classes", fmt.Sprintf("%q is listed twice.", c))
		}
		seen[c] = true
	}
	return nil
}

// validatePrevious checks that the source campaign exists and is over: an
// active campaign's "never sent" recipients may still be sent to.
func (s *Service) validatePrevious(ctx context.Context, p PreviousCampaign) error {
	c, err := s.q.GetCampaign(ctx, p.CampaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Validation("audience.previous_campaign.campaign_id", "Unknown campaign.")
	}
	if err != nil {
		return err
	}
	if c.Status != "completed" && c.Status != "cancelled" && c.Status != "failed" {
		return apperr.Validation("audience.previous_campaign.campaign_id",
			fmt.Sprintf("The campaign is %s; only finished campaigns (completed, cancelled or failed) can be targeted again.", c.Status))
	}
	return nil
}

// ClassCounts returns a campaign's recipients (unique contacts) by why they
// were not reached, including ClassReached.
func (s *Service) ClassCounts(ctx context.Context, id uuid.UUID) (map[string]int64, error) {
	return classCounts(ctx, s.q, id)
}

func classCounts(ctx context.Context, q *db.Queries, id uuid.UUID) (map[string]int64, error) {
	rows, err := q.CampaignClassCounts(ctx, id)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.Class] = r.N
	}
	return out, nil
}

// Outcome combines a recipient's state, its message's status and why it
// failed into one word for the recipients table:
// pending | retry_scheduled | in_flight | deferred | sent | delivered | read |
// unknown | failed | skipped | cancelled.
func Outcome(state string, messageStatus *string) string {
	switch state {
	case "pending":
		return "pending"
	case "retry":
		return "retry_scheduled"
	case "skipped":
		return "skipped"
	case "cancelled":
		return "cancelled"
	}
	if messageStatus == nil {
		return "pending"
	}
	switch st := *messageStatus; st {
	case "pending", "sending", "held":
		return "in_flight"
	default:
		return st // deferred, sent, delivered, read, unknown, failed, skipped, cancelled
	}
}
