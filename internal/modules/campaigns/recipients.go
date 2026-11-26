package campaigns

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts/segment"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The recipients table of a campaign: server-side filters, sorting,
// pagination, bulk actions and exports over campaign_recipients joined with
// contacts. Every condition reads the recipient's own (mirrored) columns.

// Recipient status filters. Besides these, every unreached class
// (campaign_recipient_class) can be filtered on.
var recipientStatuses = map[string]string{
	"sent":      "r.sent_at IS NOT NULL",
	"delivered": "r.delivered_at IS NOT NULL",
	"read":      "r.read_at IS NOT NULL",
	"failed":    "campaign_recipient_outcome(r.state, r.msg_status) = 'failed'",
	"pending":   "campaign_recipient_outcome(r.state, r.msg_status) = 'pending'",
	"unknown":   "campaign_recipient_outcome(r.state, r.msg_status) = 'unknown'",
	"blocked":   "campaign_recipient_outcome(r.state, r.msg_status) = 'blocked'",
	"cancelled": "campaign_recipient_outcome(r.state, r.msg_status) = 'cancelled'",
	"rejected":  "r.msg_status = 'failed' AND r.error_code ~ '^[0-9]+$'",
	"replied":   "r.replied_at IS NOT NULL",
	"positive":  "r.reaction = 'positive'",
	"negative":  "r.reaction = 'negative'",
	"neutral":   "r.reaction = 'neutral'",
	"opted_out": "r.opted_out_at IS NOT NULL",
}

// unreachedClasses can be filtered on as statuses.
var unreachedClasses = []string{ClassTemporary, ClassRejectedByMeta, ClassEcosystemLimit, ClassUndeliverable, ClassPermanent,
	ClassNeverSent, ClassUserStoppedMarketing, ClassBlockedByConsent}

// recipientSorts are the sortable columns.
var recipientSorts = map[string]string{
	"created": "r.id", "name": "lower(coalesce(c.name, ''))", "phone": "r.phone_e164", "sent_at": "r.sent_at",
	"delivered_at": "r.delivered_at", "read_at": "r.read_at", "replied_at": "r.replied_at", "cost": "r.cost_amount",
}

// RecipientFilter narrows a campaign's recipients; statuses are ORed, the
// other parts ANDed.
type RecipientFilter struct {
	Query       string      `json:"q,omitempty"`
	Statuses    []string    `json:"status,omitempty"`
	TagID       *uuid.UUID  `json:"tag_id,omitempty"`
	GroupID     *uuid.UUID  `json:"group_id,omitempty"`
	SegmentID   *uuid.UUID  `json:"segment_id,omitempty"`
	ContactType *string     `json:"contact_type,omitempty"`
	AttrKey     string      `json:"attr_key,omitempty"`
	AttrValue   string      `json:"attr_value,omitempty"`
	IDs         []uuid.UUID `json:"ids,omitempty"` // an explicit selection of recipient ids
}

// where builds the condition (aliases r and c) for campaign id.
func (s *Service) recipientWhere(ctx context.Context, id uuid.UUID, f RecipientFilter) (string, []any, error) {
	cf := contacts.Filter{Query: f.Query, TagID: f.TagID, GroupID: f.GroupID}
	if f.SegmentID != nil {
		sg, err := s.q.GetSegment(ctx, *f.SegmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, apperr.Validation("segment_id", "Unknown segment.")
		}
		if err != nil {
			return "", nil, err
		}
		n, err := segment.Parse(sg.Definition)
		if err != nil {
			return "", nil, err
		}
		cf.Segment = n
	}
	where, args, err := cf.Where()
	if err != nil {
		return "", nil, err
	}
	bind := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	conds := []string{"r.campaign_id = " + bind(id)}
	if where != "TRUE" {
		conds = append(conds, where)
	}
	if len(f.Statuses) > 0 {
		var or []string
		for _, st := range f.Statuses {
			if sql, ok := recipientStatuses[st]; ok {
				or = append(or, sql)
				continue
			}
			if slices.Contains(unreachedClasses, st) {
				or = append(or, "campaign_recipient_class(r.state, r.skip_reason, r.msg_status, r.failure_class) = "+bind(st))
				continue
			}
			return "", nil, apperr.Validation("status", fmt.Sprintf("Unknown status filter %q.", st))
		}
		conds = append(conds, "("+strings.Join(or, " OR ")+")")
	}
	if f.ContactType != nil {
		if *f.ContactType == "" {
			conds = append(conds, "c.contact_type IS NULL")
		} else {
			conds = append(conds, "c.contact_type = "+bind(*f.ContactType))
		}
	}
	if f.AttrKey != "" {
		if len(f.AttrKey) > 100 || len(f.AttrValue) > 200 {
			return "", nil, apperr.Validation("attr_key", "Attribute filters are limited to 100/200 characters.")
		}
		conds = append(conds, "c.attributes ->> "+bind(f.AttrKey)+" = "+bind(f.AttrValue))
	}
	if f.IDs != nil {
		conds = append(conds, "r.id = ANY("+bind(f.IDs)+"::uuid[])")
	}
	return strings.Join(conds, " AND "), args, nil
}

// RecipientRow is one row of the recipients table.
type RecipientRow struct {
	db.CampaignRecipient
	Name, ContactType string
	Cost              *string
	UnreachedClass    string
}

const recipientColumns = `r.id, r.campaign_id, r.contact_id, r.phone_e164, r.state, r.skip_reason, r.message_id, r.created_at, r.updated_at,
	r.next_attempt_at, r.resends, r.msg_status, r.attempted_at, r.sent_at, r.delivered_at, r.read_at, r.failed_at, r.error_code,
	r.failure_class, r.cost_amount::text, r.cost_currency, r.replied_at, r.reply_message_id, r.reply_payload, r.reaction,
	r.reaction_source, r.reaction_at, r.opted_out_at, coalesce(c.name, ''), coalesce(c.contact_type, ''),
	coalesce(campaign_recipient_class(r.state, r.skip_reason, r.msg_status, r.failure_class), '')`

func scanRecipient(rows pgx.Rows) (RecipientRow, error) {
	var x RecipientRow
	r := &x.CampaignRecipient
	err := rows.Scan(&r.ID, &r.CampaignID, &r.ContactID, &r.PhoneE164, &r.State, &r.SkipReason, &r.MessageID, &r.CreatedAt, &r.UpdatedAt,
		&r.NextAttemptAt, &r.Resends, &r.MsgStatus, &r.AttemptedAt, &r.SentAt, &r.DeliveredAt, &r.ReadAt, &r.FailedAt, &r.ErrorCode,
		&r.FailureClass, &x.Cost, &r.CostCurrency, &r.RepliedAt, &r.ReplyMessageID, &r.ReplyPayload, &r.Reaction,
		&r.ReactionSource, &r.ReactionAt, &r.OptedOutAt, &x.Name, &x.ContactType, &x.UnreachedClass)
	return x, err
}

// RecipientPage is one page of the recipients table with the total.
type RecipientPage struct {
	Rows  []RecipientRow
	Total int64
}

// SearchRecipients returns a page (offset pagination) sorted by sort
// ("created" by default, "-" prefix = descending) and the matching total.
func (s *Service) SearchRecipients(ctx context.Context, id uuid.UUID, f RecipientFilter, sort string, offset, limit int) (RecipientPage, error) {
	if _, err := s.q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return RecipientPage{}, apperr.NotFound("Campaign not found.")
	}
	where, args, err := s.recipientWhere(ctx, id, f)
	if err != nil {
		return RecipientPage{}, err
	}
	order, err := recipientOrder(sort)
	if err != nil {
		return RecipientPage{}, err
	}
	var out RecipientPage
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM campaign_recipients r LEFT JOIN contacts c ON c.id = r.contact_id WHERE "+where,
		args...).Scan(&out.Total); err != nil {
		return out, err
	}
	n := len(args)
	rows, err := s.pool.Query(ctx, fmt.Sprintf("SELECT %s FROM campaign_recipients r LEFT JOIN contacts c ON c.id = r.contact_id WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d",
		recipientColumns, where, order, n+1, n+2), append(args, limit, offset)...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Rows = []RecipientRow{}
	for rows.Next() {
		x, err := scanRecipient(rows)
		if err != nil {
			return out, err
		}
		out.Rows = append(out.Rows, x)
	}
	return out, rows.Err()
}

func recipientOrder(sort string) (string, error) {
	desc := strings.HasPrefix(sort, "-")
	key := strings.TrimPrefix(sort, "-")
	if key == "" {
		key, desc = "created", true
	}
	col, ok := recipientSorts[key]
	if !ok {
		return "", apperr.Validation("sort", fmt.Sprintf("Cannot sort by %q.", key))
	}
	dir := "ASC NULLS LAST"
	if desc {
		dir = "DESC NULLS LAST"
	}
	return col + " " + dir + ", r.id " + strings.Fields(dir)[0], nil
}

// EachRecipient streams the matching recipients in id order, in pages of
// 1000 (exports).
func (s *Service) EachRecipient(ctx context.Context, id uuid.UUID, f RecipientFilter, fn func(RecipientRow) error) error {
	where, args, err := s.recipientWhere(ctx, id, f)
	if err != nil {
		return err
	}
	after := uuid.Nil
	n := len(args)
	for {
		rows, err := s.pool.Query(ctx, fmt.Sprintf("SELECT %s FROM campaign_recipients r LEFT JOIN contacts c ON c.id = r.contact_id WHERE %s AND r.id > $%d ORDER BY r.id LIMIT 1000",
			recipientColumns, where, n+1), append(args, after)...)
		if err != nil {
			return err
		}
		var batch []RecipientRow
		for rows.Next() {
			x, err := scanRecipient(rows)
			if err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, x)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, x := range batch {
			if err := fn(x); err != nil {
				return err
			}
		}
		if len(batch) < 1000 {
			return nil
		}
		after = batch[len(batch)-1].ID
	}
}

// CountRecipients counts the matching recipients.
func (s *Service) CountRecipients(ctx context.Context, id uuid.UUID, f RecipientFilter) (int64, error) {
	where, args, err := s.recipientWhere(ctx, id, f)
	if err != nil {
		return 0, err
	}
	var n int64
	err = s.pool.QueryRow(ctx, "SELECT count(*) FROM campaign_recipients r LEFT JOIN contacts c ON c.id = r.contact_id WHERE "+where, args...).Scan(&n)
	return n, err
}

// Bulk actions on a selection of recipients.
const (
	BulkAddTag     = "add_tag"
	BulkAddToGroup = "add_to_group"
	BulkFollowUp   = "follow_up"
)

// BulkResult is what a bulk action did.
type BulkResult struct {
	Affected int64
	// Campaign is the follow-up draft (follow_up).
	Campaign *db.Campaign
}

// BulkRecipients applies action to the matching recipients' contacts: tag
// them, add them to a group, or create a draft follow-up campaign (same
// number and template) whose audience is exactly these contacts.
func (s *Service) BulkRecipients(ctx context.Context, id uuid.UUID, f RecipientFilter, action string, target *uuid.UUID, actor *uuid.UUID) (BulkResult, error) {
	c, err := s.q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return BulkResult{}, apperr.NotFound("Campaign not found.")
	}
	if err != nil {
		return BulkResult{}, err
	}
	where, args, err := s.recipientWhere(ctx, id, f)
	if err != nil {
		return BulkResult{}, err
	}
	sel := "SELECT DISTINCT r.contact_id FROM campaign_recipients r LEFT JOIN contacts c ON c.id = r.contact_id WHERE r.contact_id IS NOT NULL AND " + where
	var out BulkResult
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		switch action {
		case BulkAddTag, BulkAddToGroup:
			if target == nil {
				return apperr.Validation("target_id", "Choose the tag or group.")
			}
			var sql string
			if action == BulkAddTag {
				if _, err := q.GetTag(ctx, *target); errors.Is(err, pgx.ErrNoRows) {
					return apperr.Validation("target_id", "Unknown tag.")
				}
				sql = fmt.Sprintf("INSERT INTO contact_tags (contact_id, tag_id) SELECT x.contact_id, $%d FROM (%s) x ON CONFLICT DO NOTHING", len(args)+1, sel)
			} else {
				if _, err := q.GetContactGroup(ctx, *target); errors.Is(err, pgx.ErrNoRows) {
					return apperr.Validation("target_id", "Unknown group.")
				}
				sql = fmt.Sprintf("INSERT INTO contact_group_members (group_id, contact_id) SELECT $%d, x.contact_id FROM (%s) x ON CONFLICT DO NOTHING", len(args)+1, sel)
			}
			tag, err := tx.Exec(ctx, sql, append(args, *target)...)
			if err != nil {
				return err
			}
			out.Affected = tag.RowsAffected()
		case BulkFollowUp:
			rows, err := tx.Query(ctx, sel+fmt.Sprintf(" LIMIT %d", MaxAudienceContacts+1), args...)
			if err != nil {
				return err
			}
			ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return apperr.Validation("selection", "The selection has no contacts.")
			}
			if len(ids) > MaxAudienceContacts {
				return apperr.Validation("selection", fmt.Sprintf("A follow-up campaign takes at most %d contacts; narrow the selection.", MaxAudienceContacts))
			}
			slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
			aj, _ := jsonMarshal(Audience{ContactIDs: ids})
			name := c.Name
			if len([]rune(name)) > 105 {
				name = string([]rune(name)[:105])
			}
			fc, err := q.InsertCampaign(ctx, db.InsertCampaignParams{Name: name + " — takip", PhoneNumberID: c.PhoneNumberID,
				TemplateName: c.TemplateName, TemplateLanguage: c.TemplateLanguage, TemplateParams: c.TemplateParams, Audience: aj,
				CreatedBy: actor, OfferID: c.OfferID, Priority: c.Priority, MaxMps: c.MaxMps})
			if err != nil {
				return err
			}
			if _, err := q.SetCampaignReactionMap(ctx, db.SetCampaignReactionMapParams{ID: fc.ID, ReactionMap: c.ReactionMap}); err != nil {
				return err
			}
			out.Campaign, out.Affected = &fc, int64(len(ids))
			if err := audit.Record(ctx, q, audit.Entry{Action: "campaign.create", EntityType: "campaign", EntityID: &fc.ID,
				Changes: map[string]any{"follow_up_of": id, "contacts": len(ids)}}); err != nil {
				return err
			}
		default:
			return apperr.Validation("action", "Unknown action.")
		}
		return audit.Record(ctx, q, audit.Entry{Action: "campaign.recipients_bulk", EntityType: "campaign", EntityID: &id,
			Changes: map[string]any{"action": action, "target_id": target, "affected": out.Affected}})
	})
	return out, err
}

// ── Breakdowns ───────────────────────────────────────────────────────────

// Breakdown dimensions.
const (
	DimGroup       = "group"
	DimSegment     = "segment"
	DimTag         = "tag"
	DimContactType = "contact_type"
	DimAttribute   = "attribute"
)

// Bucket is one row of a breakdown.
type Bucket struct {
	Key, Label                                                               string
	Recipients, Sent, Delivered, Read, Replied, Positive, Negative, OptedOut int64
	Cost                                                                     string
}

const bucketMetrics = `count(*), count(r.sent_at), count(r.delivered_at), count(r.read_at), count(r.replied_at),
	count(*) FILTER (WHERE r.reaction = 'positive'), count(*) FILTER (WHERE r.reaction = 'negative'), count(r.opted_out_at),
	coalesce(sum(r.cost_amount), 0)::text`

// Breakdown computes a campaign's figures per group, segment, tag, contact
// type or value of a custom attribute in one query over its recipients. A
// contact in two tags (groups, segments) counts in both.
func (s *Service) Breakdown(ctx context.Context, id uuid.UUID, dim, attrKey string) ([]Bucket, error) {
	if _, err := s.q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Campaign not found.")
	}
	var sql string
	args := []any{id}
	switch dim {
	case DimTag:
		sql = `SELECT t.id::text, t.name, ` + bucketMetrics + ` FROM campaign_recipients r
			JOIN contact_tags ct ON ct.contact_id = r.contact_id JOIN tags t ON t.id = ct.tag_id
			WHERE r.campaign_id = $1 GROUP BY t.id, t.name ORDER BY count(*) DESC, t.name LIMIT 100`
	case DimGroup:
		sql = `SELECT g.id::text, g.name, ` + bucketMetrics + ` FROM campaign_recipients r
			JOIN contact_group_members gm ON gm.contact_id = r.contact_id JOIN contact_groups g ON g.id = gm.group_id
			WHERE r.campaign_id = $1 GROUP BY g.id, g.name ORDER BY count(*) DESC, g.name LIMIT 100`
	case DimContactType:
		sql = `SELECT coalesce(c.contact_type, ''), coalesce(c.contact_type, ''), ` + bucketMetrics + ` FROM campaign_recipients r
			LEFT JOIN contacts c ON c.id = r.contact_id
			WHERE r.campaign_id = $1 GROUP BY 1 ORDER BY count(*) DESC, 1 LIMIT 100`
	case DimAttribute:
		if strings.TrimSpace(attrKey) == "" || len(attrKey) > 100 {
			return nil, apperr.Validation("attr_key", "Choose the attribute.")
		}
		args = append(args, attrKey)
		sql = `SELECT coalesce(c.attributes ->> $2, ''), coalesce(c.attributes ->> $2, ''), ` + bucketMetrics + ` FROM campaign_recipients r
			LEFT JOIN contacts c ON c.id = r.contact_id
			WHERE r.campaign_id = $1 GROUP BY 1 ORDER BY count(*) DESC, 1 LIMIT 100`
	case DimSegment:
		return s.segmentBreakdown(ctx, id)
	default:
		return nil, apperr.Validation("dimension", "Break down by group, segment, tag, contact_type or attribute.")
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bucket{}
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Key, &b.Label, &b.Recipients, &b.Sent, &b.Delivered, &b.Read, &b.Replied, &b.Positive, &b.Negative,
			&b.OptedOut, &b.Cost); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// segmentBreakdown evaluates every segment (at most 30) as a FILTER over
// the campaign's recipients, all in one query.
func (s *Service) segmentBreakdown(ctx context.Context, id uuid.UUID) ([]Bucket, error) {
	segs, err := s.q.ListSegments(ctx)
	if err != nil {
		return nil, err
	}
	if len(segs) > 30 {
		segs = segs[:30]
	}
	args := []any{id}
	var cols []string
	var kept []db.Segment
	for _, sg := range segs {
		n, err := segment.Parse(sg.Definition)
		if err != nil {
			continue
		}
		cmp, err := segment.Compile(n, len(args)+1)
		if err != nil {
			continue
		}
		args = append(args, cmp.Args...)
		f := "FILTER (WHERE " + cmp.SQL + ")"
		cols = append(cols, "count(*) "+f, "count(r.sent_at) "+f, "count(r.delivered_at) "+f, "count(r.read_at) "+f,
			"count(r.replied_at) "+f, "count(*) FILTER (WHERE r.reaction = 'positive' AND ("+cmp.SQL+"))",
			"count(*) FILTER (WHERE r.reaction = 'negative' AND ("+cmp.SQL+"))", "count(r.opted_out_at) "+f,
			"coalesce(sum(r.cost_amount) "+f+", 0)::text")
		kept = append(kept, sg)
	}
	out := []Bucket{}
	if len(kept) == 0 {
		return out, nil
	}
	row := s.pool.QueryRow(ctx, "SELECT "+strings.Join(cols, ", ")+
		" FROM campaign_recipients r JOIN contacts c ON c.id = r.contact_id WHERE r.campaign_id = $1", args...)
	vals := make([]any, 0, len(kept)*9)
	buckets := make([]Bucket, len(kept))
	for i := range kept {
		b := &buckets[i]
		b.Key, b.Label = kept[i].ID.String(), kept[i].Name
		vals = append(vals, &b.Recipients, &b.Sent, &b.Delivered, &b.Read, &b.Replied, &b.Positive, &b.Negative, &b.OptedOut, &b.Cost)
	}
	if err := row.Scan(vals...); err != nil {
		return nil, err
	}
	for _, b := range buckets {
		if b.Recipients > 0 {
			out = append(out, b)
		}
	}
	slices.SortStableFunc(out, func(a, b Bucket) int { return int(b.Recipients - a.Recipients) })
	return out, nil
}

// HourlyPoint is one hour of a campaign's activity.
type HourlyPoint struct {
	Hour                                   time.Time
	Sent, Delivered, Read, Replied, Failed int64
}

// Hourly returns the campaign's sends, deliveries, reads, replies and
// failures per hour.
func (s *Service) Hourly(ctx context.Context, id uuid.UUID) ([]HourlyPoint, error) {
	rows, err := s.q.CampaignHourly(ctx, id)
	if err != nil {
		return nil, err
	}
	out := []HourlyPoint{}
	idx := map[time.Time]int{}
	for _, r := range rows {
		i, ok := idx[r.Hour]
		if !ok {
			out = append(out, HourlyPoint{Hour: r.Hour})
			i = len(out) - 1
			idx[r.Hour] = i
		}
		p := &out[i]
		switch r.Kind {
		case "sent":
			p.Sent = r.N
		case "delivered":
			p.Delivered = r.N
		case "read":
			p.Read = r.N
		case "replied":
			p.Replied = r.N
		case "failed":
			p.Failed = r.N
		}
	}
	slices.SortFunc(out, func(a, b HourlyPoint) int { return a.Hour.Compare(b.Hour) })
	return out, nil
}
