package consent

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Sources of a consent change (consent_events.source).
const (
	SourceManual              = "manual"
	SourceBulk                = "bulk"
	SourceAPI                 = "api"
	SourceImport              = "import"
	SourceKeyword             = "keyword"
	SourceMetaUserPreferences = "meta_user_preferences"
	SourceMetaError           = "meta_error"
	SourceAutomation          = "automation"
	SourceInbound             = "inbound"
)

// OptOutObserver is told, inside the transaction that records it, that a
// contact stopped marketing at a moment: opted out with the business, or
// chose Stop in WhatsApp (campaign analytics attributes it to the campaign
// that reached them last). tx is nil when the caller has none at hand.
type OptOutObserver func(ctx context.Context, tx pgx.Tx, q *db.Queries, contactID uuid.UUID, at time.Time) error

var optOutObserver OptOutObserver

// ObserveOptOuts installs the opt-out observer (one per process).
func ObserveOptOuts(f OptOutObserver) { optOutObserver = f }

func notifyOptOut(ctx context.Context, tx pgx.Tx, q *db.Queries, contactID uuid.UUID, at time.Time) error {
	if optOutObserver == nil {
		return nil
	}
	return optOutObserver(ctx, tx, q, contactID, at)
}

// Change sets the consent of one contact.
type Change struct {
	ContactID uuid.UUID
	// Categories to change; empty = both.
	Categories []string
	Status     Status
	Source     string
	Reason     string
	ActorID    *uuid.UUID
}

// kindOf maps a status to its consent_events kind.
func kindOf(s Status) string {
	switch s {
	case OptedIn:
		return "opt_in"
	case OptedOut:
		return "opt_out"
	case OptOutRequested:
		return "opt_out_requested"
	case Resubscribed:
		return "resubscribed"
	}
	return "unknown"
}

// Result describes what Apply changed.
type Result struct {
	Before, After db.Contact
	// Changed lists the categories whose status changed.
	Changed []string
}

// FullyOptedOut reports whether the legacy "opted out" flag changed and how.
func (r Result) legacyChanged() (changed, optedOut bool) {
	return (r.Before.OptedOutAt != nil) != (r.After.OptedOutAt != nil), r.After.OptedOutAt != nil
}

// Apply records a consent change inside the caller's transaction: the
// contact's columns, one consent event per changed category, and the
// contact.opted_out / contact.opted_in webhook when the legacy flag flips.
// Setting a category to the status it already has records nothing.
func Apply(ctx context.Context, tx pgx.Tx, q *db.Queries, ch Change) (Result, error) {
	if !ch.Status.Valid() {
		return Result{}, apperr.Validation("status", "Unknown consent status.")
	}
	if len(ch.Reason) > 500 {
		return Result{}, apperr.Validation("reason", "Reason is limited to 500 characters.")
	}
	cats := ch.Categories
	if len(cats) == 0 {
		cats = []string{CatMarketing, CatService}
	}
	before, err := q.LockContact(ctx, ch.ContactID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, apperr.NotFound("Contact not found.")
	}
	if err != nil {
		return Result{}, err
	}
	st := StateOf(before)
	res := Result{Before: before, After: before}
	var marketing, service bool
	for _, c := range cats {
		switch c {
		case CatMarketing:
			marketing = marketing || st.Marketing.Status != ch.Status || before.MarketingConsent != string(ch.Status)
		case CatService:
			service = service || st.Service.Status != ch.Status || before.ServiceConsent != string(ch.Status)
		default:
			return Result{}, apperr.Validation("category", "Category is marketing or service.")
		}
	}
	if !marketing && !service {
		return res, nil
	}
	var reason *string
	if ch.Reason != "" {
		reason = &ch.Reason
	}
	after, err := q.SetContactConsent(ctx, db.SetContactConsentParams{ID: ch.ContactID, Marketing: marketing, Service: service,
		Status: string(ch.Status), Source: ch.Source, Reason: reason})
	if err != nil {
		return res, err
	}
	res.After = after
	status := string(ch.Status)
	if marketing && st.Marketing.Status != ch.Status {
		res.Changed = append(res.Changed, CatMarketing)
	}
	if service && st.Service.Status != ch.Status {
		res.Changed = append(res.Changed, CatService)
	}
	// One event per change; a change of both categories is one event
	// without a category (as every record before consent per category).
	var cat *string
	switch len(res.Changed) {
	case 0:
		return res, nil // only the columns caught up with the legacy flag
	case 1:
		cat = &res.Changed[0]
	}
	if _, err := q.InsertConsentEvent(ctx, db.InsertConsentEventParams{ContactID: ch.ContactID, Kind: kindOf(ch.Status),
		Source: ch.Source, Note: ch.Reason, ActorID: ch.ActorID, Category: cat, Status: &status}); err != nil {
		return res, err
	}
	if ch.Status == OptedOut && slices.Contains(res.Changed, CatMarketing) {
		if err := notifyOptOut(ctx, tx, q, ch.ContactID, time.Now()); err != nil {
			return res, err
		}
	}
	if changed, out := res.legacyChanged(); changed {
		typ := webhooks.TypeContactOptedIn
		if out {
			typ = webhooks.TypeContactOptedOut
		}
		if err := webhooks.ContactChanged(ctx, tx, typ, ch.ContactID); err != nil {
			return res, err
		}
	}
	return res, nil
}

// SetMetaMarketingStopped records the customer's "Stop" (true) or "Resume"
// (false) for marketing in WhatsApp. It never touches the contact's own
// marketing consent: a customer who opted out with the business stays opted
// out after resuming in WhatsApp. It reports whether anything changed.
func SetMetaMarketingStopped(ctx context.Context, q *db.Queries, contactID uuid.UUID, stopped bool, at time.Time, source, note string) (bool, error) {
	n, err := q.SetMetaMarketingStopped(ctx, db.SetMetaMarketingStoppedParams{ID: contactID, Stopped: stopped, At: at})
	if err != nil || n == 0 {
		return false, err
	}
	kind := "meta_marketing_resumed"
	if stopped {
		kind = "meta_marketing_stopped"
	}
	cat := CatMarketing
	if _, err = q.InsertConsentEvent(ctx, db.InsertConsentEventParams{ContactID: contactID, Kind: kind, Source: source,
		Note: truncate(note, 500), Category: &cat}); err != nil {
		return false, err
	}
	// A 131050 only reveals an earlier Stop; the customer's own Stop counts.
	if stopped && source != SourceMetaError {
		if err := notifyOptOut(ctx, nil, q, contactID, at); err != nil {
			return false, err
		}
	}
	return true, nil
}

// SetUndeliverable flags a number Meta could not deliver to (131026).
func SetUndeliverable(ctx context.Context, q *db.Queries, contactID uuid.UUID, code string) (bool, error) {
	n, err := q.SetContactUndeliverable(ctx, db.SetContactUndeliverableParams{ID: contactID, Code: &code})
	if err != nil || n == 0 {
		return false, err
	}
	_, err = q.InsertConsentEvent(ctx, db.InsertConsentEventParams{ContactID: contactID, Kind: "undeliverable",
		Source: SourceMetaError, Note: "Meta error " + code})
	return err == nil, err
}

// ClearUndeliverable lifts the flag after the number wrote to the business.
func ClearUndeliverable(ctx context.Context, q *db.Queries, contactID uuid.UUID) (bool, error) {
	n, err := q.ClearContactUndeliverable(ctx, contactID)
	if err != nil || n == 0 {
		return false, err
	}
	_, err = q.InsertConsentEvent(ctx, db.InsertConsentEventParams{ContactID: contactID, Kind: "deliverable",
		Source: SourceInbound, Note: "The customer wrote again."})
	return err == nil, err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
