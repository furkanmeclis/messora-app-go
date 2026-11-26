package contacts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts/segment"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Filter narrows a contact listing; all parts are combined with AND.
type Filter struct {
	Query    string // name, profile name, phone or e-mail contains
	TagID    *uuid.UUID
	GroupID  *uuid.UUID
	Segment  *segment.Node
	OptedOut *bool
	// Consent status per category (the legacy opted_out_at counts as
	// opted_out for both).
	MarketingConsent *string
	ServiceConsent   *string
	// Reachability: "meta_marketing_stopped" or "undeliverable".
	Reachability *string
	// FromCampaign with FromClasses: recipients of that campaign it did not
	// reach for one of these reasons (campaign_recipient_class). Each
	// contact matches once however many recipient rows it had.
	FromCampaign *uuid.UUID
	FromClasses  []string
	// ContactIDs restricts to an explicit list (a campaign's "selected
	// contacts" audience); nil = no restriction.
	ContactIDs []uuid.UUID
	// AIProfile true: contacts whose profile the AI built from an unknown
	// number (collecting or complete); false: the others.
	AIProfile *bool
}

// consentExpr is a category's effective consent status in SQL.
func consentExpr(column string) string {
	return "(CASE WHEN c.opted_out_at IS NOT NULL THEN 'opted_out' ELSE c." + column + " END)"
}

// where builds the WHERE clause and arguments.
func (f Filter) where() (string, []any, error) {
	var conds []string
	var args []any
	bind := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		if len([]rune(q)) > 100 {
			return "", nil, apperr.Validation("q", "Search is limited to 100 characters.")
		}
		like := bind("%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%")
		conds = append(conds, "(c.name ILIKE "+like+" OR c.profile_name ILIKE "+like+" OR c.phone_e164 LIKE "+like+" OR c.email::text ILIKE "+like+")")
	}
	if f.TagID != nil {
		conds = append(conds, "EXISTS (SELECT 1 FROM contact_tags ct WHERE ct.contact_id = c.id AND ct.tag_id = "+bind(*f.TagID)+")")
	}
	if f.GroupID != nil {
		conds = append(conds, "EXISTS (SELECT 1 FROM contact_group_members gm WHERE gm.contact_id = c.id AND gm.group_id = "+bind(*f.GroupID)+")")
	}
	if f.OptedOut != nil {
		if *f.OptedOut {
			conds = append(conds, "c.opted_out_at IS NOT NULL")
		} else {
			conds = append(conds, "c.opted_out_at IS NULL")
		}
	}
	if f.MarketingConsent != nil {
		conds = append(conds, consentExpr("marketing_consent")+" = "+bind(*f.MarketingConsent))
	}
	if f.ServiceConsent != nil {
		conds = append(conds, consentExpr("service_consent")+" = "+bind(*f.ServiceConsent))
	}
	if f.Reachability != nil {
		switch *f.Reachability {
		case "meta_marketing_stopped":
			conds = append(conds, "c.meta_marketing_stopped_at IS NOT NULL")
		case "undeliverable":
			conds = append(conds, "c.undeliverable_at IS NOT NULL")
		default:
			return "", nil, apperr.Validation("reachability", "Unknown reachability filter.")
		}
	}
	if f.AIProfile != nil {
		if *f.AIProfile {
			conds = append(conds, "c.profile_status IN ('collecting', 'complete')")
		} else {
			conds = append(conds, "(c.profile_status IS NULL OR c.profile_status NOT IN ('collecting', 'complete'))")
		}
	}
	if f.FromCampaign != nil {
		conds = append(conds, "c.id IN (SELECT r.contact_id FROM campaign_recipients r LEFT JOIN messages m ON m.id = r.message_id"+
			" WHERE r.campaign_id = "+bind(*f.FromCampaign)+" AND r.contact_id IS NOT NULL"+
			" AND campaign_recipient_class(r.state, r.skip_reason, m.status, m.failure_class) = ANY("+bind(f.FromClasses)+"::text[]))")
	}
	if f.ContactIDs != nil {
		conds = append(conds, "c.id = ANY("+bind(f.ContactIDs)+"::uuid[])")
	}
	if f.Segment != nil {
		cmp, err := segment.Compile(f.Segment, len(args)+1)
		if err != nil {
			return "", nil, apperr.Validation("definition", err.Error())
		}
		conds = append(conds, cmp.SQL)
		args = append(args, cmp.Args...)
	}
	if len(conds) == 0 {
		return "TRUE", args, nil
	}
	return strings.Join(conds, " AND "), args, nil
}

// List returns up to limit contacts, newest first, before the cursor id.
func (s *Service) List(ctx context.Context, f Filter, before *uuid.UUID, limit int) ([]db.Contact, error) {
	where, args, err := f.where()
	if err != nil {
		return nil, err
	}
	if before != nil {
		args = append(args, *before)
		where += fmt.Sprintf(" AND c.id < $%d", len(args))
	}
	args = append(args, limit)
	sql := fmt.Sprintf("SELECT c.* FROM contacts c WHERE %s ORDER BY c.id DESC LIMIT $%d", where, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[db.Contact])
}

// Count returns how many contacts match (segment preview).
func (s *Service) Count(ctx context.Context, f Filter) (int64, error) {
	where, args, err := f.where()
	if err != nil {
		return 0, err
	}
	var n int64
	err = s.pool.QueryRow(ctx, "SELECT count(*) FROM contacts c WHERE "+where, args...).Scan(&n)
	return n, err
}

// Each streams every matching contact to fn in id order (exports and
// campaign materialisation), fetching in pages to bound memory.
func (s *Service) Each(ctx context.Context, f Filter, fn func(db.Contact) error) error {
	where, args, err := f.where()
	if err != nil {
		return err
	}
	const page = 1000
	var after uuid.UUID
	for {
		a := append(append([]any{}, args...), after, page)
		sql := fmt.Sprintf("SELECT c.* FROM contacts c WHERE %s AND c.id > $%d ORDER BY c.id LIMIT $%d", where, len(args)+1, len(args)+2)
		rows, err := s.pool.Query(ctx, sql, a...)
		if err != nil {
			return err
		}
		batch, err := pgx.CollectRows(rows, pgx.RowToStructByPos[db.Contact])
		if err != nil {
			return err
		}
		for _, c := range batch {
			if err := fn(c); err != nil {
				return err
			}
		}
		if len(batch) < page {
			return nil
		}
		after = batch[len(batch)-1].ID
	}
}

// Page calls fn for up to limit matching contacts after the cursor id, in
// id order (resumable keyset iteration for campaign materialisation).
func (s *Service) Page(ctx context.Context, f Filter, after *uuid.UUID, limit int, fn func(db.Contact)) error {
	where, args, err := f.where()
	if err != nil {
		return err
	}
	start := uuid.Nil
	if after != nil {
		start = *after
	}
	a := append(append([]any{}, args...), start, limit)
	sql := fmt.Sprintf("SELECT c.* FROM contacts c WHERE %s AND c.id > $%d ORDER BY c.id LIMIT $%d", where, len(args)+1, len(args)+2)
	rows, err := s.pool.Query(ctx, sql, a...)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[db.Contact])
	if err != nil {
		return err
	}
	for _, c := range list {
		fn(c)
	}
	return nil
}

// TagsFor returns tags for a page of contacts.
func (s *Service) TagsFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]db.Tag, error) {
	rows, err := s.q.TagsForContacts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]db.Tag{}
	for _, r := range rows {
		out[r.ContactID] = append(out[r.ContactID], db.Tag{ID: r.ID, Name: r.Name, Color: r.Color})
	}
	return out, nil
}

// ConsentGroup counts the matching contacts that share one consent state.
type ConsentGroup struct {
	State consent.State
	N     int64
}

// ConsentGroups breaks the matching contacts down by consent state (campaign
// preflight): one row per distinct combination, so it stays one aggregate
// query however large the audience.
func (s *Service) ConsentGroups(ctx context.Context, f Filter) ([]ConsentGroup, error) {
	where, args, err := f.where()
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT c.marketing_consent, c.service_consent, c.opted_out_at IS NOT NULL,
			c.meta_marketing_stopped_at IS NOT NULL, c.undeliverable_at IS NOT NULL, count(*)
		FROM contacts c WHERE `+where+` GROUP BY 1, 2, 3, 4, 5`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConsentGroup
	set := time.Unix(0, 0).UTC()
	flag := func(on bool) *time.Time {
		if on {
			return &set
		}
		return nil
	}
	for rows.Next() {
		var m, sv string
		var out1, stopped, undeliverable bool
		var n int64
		if err := rows.Scan(&m, &sv, &out1, &stopped, &undeliverable, &n); err != nil {
			return nil, err
		}
		out = append(out, ConsentGroup{N: n, State: consent.StateOf(db.Contact{MarketingConsent: m, ServiceConsent: sv,
			OptedOutAt: flag(out1), MetaMarketingStoppedAt: flag(stopped), UndeliverableAt: flag(undeliverable)})})
	}
	return out, rows.Err()
}

// Where returns the SQL condition (over contacts aliased c) and arguments
// of f, numbered from $1, for queries of other modules that join contacts
// (campaign recipients).
func (f Filter) Where() (string, []any, error) { return f.where() }

// ForecastGroup counts matching contacts that share a consent state, a
// phone prefix (the first five characters of the E.164 number: enough for
// the country, +1 area codes included) and whether their customer service
// window with the sending number is open and whether they already count
// against the portfolio's messaging limit.
type ForecastGroup struct {
	State      consent.State
	Prefix     string
	WindowOpen bool
	Reached    bool
	N          int64
}

// ForecastGroups breaks the matching contacts down for a campaign forecast
// in one aggregate query.
func (s *Service) ForecastGroups(ctx context.Context, f Filter, phoneNumberID, accountID uuid.UUID) ([]ForecastGroup, error) {
	where, args, err := f.where()
	if err != nil {
		return nil, err
	}
	n := len(args)
	args = append(args, phoneNumberID, accountID)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT c.marketing_consent, c.service_consent, c.opted_out_at IS NOT NULL,
			c.meta_marketing_stopped_at IS NOT NULL, c.undeliverable_at IS NOT NULL, left(c.phone_e164, 5),
			EXISTS (SELECT 1 FROM conversations v WHERE v.contact_id = c.id AND v.phone_number_id = $%d
			        AND v.last_inbound_at > now() - interval '24 hours'),
			EXISTS (SELECT 1 FROM portfolio_reach pr WHERE pr.account_id = $%d AND pr.contact_id = c.id
			        AND pr.last_at > now() - interval '24 hours'),
			count(*)
		FROM contacts c WHERE %s GROUP BY 1, 2, 3, 4, 5, 6, 7, 8`, n+1, n+2, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ForecastGroup
	set := time.Unix(0, 0).UTC()
	flag := func(on bool) *time.Time {
		if on {
			return &set
		}
		return nil
	}
	for rows.Next() {
		var m, sv, prefix string
		var optedOut, stopped, undeliverable, open, reached bool
		var cnt int64
		if err := rows.Scan(&m, &sv, &optedOut, &stopped, &undeliverable, &prefix, &open, &reached, &cnt); err != nil {
			return nil, err
		}
		out = append(out, ForecastGroup{N: cnt, Prefix: prefix, WindowOpen: open, Reached: reached,
			State: consent.StateOf(db.Contact{MarketingConsent: m, ServiceConsent: sv, OptedOutAt: flag(optedOut),
				MetaMarketingStoppedAt: flag(stopped), UndeliverableAt: flag(undeliverable)})})
	}
	return out, rows.Err()
}
