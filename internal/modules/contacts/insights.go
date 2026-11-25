package contacts

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AudienceKind is what an audience insight is about.
type AudienceKind string

// Audience kinds.
const (
	AudienceGroup   AudienceKind = "group"
	AudienceTag     AudienceKind = "tag"
	AudienceSegment AudienceKind = "segment"
)

// Insights describes how an audience (a group, tag or segment) behaves:
// who is in it, how messages to it perform, and every campaign that reached
// any of its contacts, whether or not the campaign targeted it.
type Insights struct {
	Summary   InsightSummary    `json:"summary"`
	Messaging InsightMessaging  `json:"messaging"`
	Campaigns []InsightCampaign `json:"campaigns"`
	Daily     []InsightDay      `json:"daily"`
	Engaged   []InsightContact  `json:"engaged"`
}

// InsightSummary counts the audience.
type InsightSummary struct {
	Contacts  int64 `json:"contacts"`
	Reachable int64 `json:"reachable"` // not opted out
	OptedOut  int64 `json:"opted_out"`
	Active7   int64 `json:"active_7d"` // wrote in the last 7 days
	Active30  int64 `json:"active_30d"`
	New30     int64 `json:"new_30d"` // joined the audience in the last 30 days
	WithEmail int64 `json:"with_email"`
}

// InsightMessaging is the last 90 days of messages with the audience.
type InsightMessaging struct {
	Outbound     int64 `json:"outbound"`
	Sent         int64 `json:"sent"`
	Delivered    int64 `json:"delivered"`
	Read         int64 `json:"read"`
	Failed       int64 `json:"failed"`
	Inbound      int64 `json:"inbound"`
	Messaged     int64 `json:"contacts_messaged"`
	Replied      int64 `json:"contacts_replied"` // wrote within 72 h of a message
	FromCampaign int64 `json:"from_campaigns"`
}

// InsightCampaign is one campaign that reached the audience.
type InsightCampaign struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Template   string     `json:"template_name"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Targeted   bool       `json:"targeted"` // the campaign's audience was this one
	Total      int64      `json:"total_recipients"`
	Reached    int64      `json:"reached"` // recipients in this audience
	Skipped    int64      `json:"skipped"`
	Sent       int64      `json:"sent"`
	Delivered  int64      `json:"delivered"`
	Read       int64      `json:"read"`
	Failed     int64      `json:"failed"`
	Replied    int64      `json:"replied"`
	OptedOut   int64      `json:"opted_out_after"`
}

// InsightDay is one day of activity.
type InsightDay struct {
	Day       string `json:"day"` // YYYY-MM-DD in the requested time zone
	Sent      int64  `json:"sent"`
	Delivered int64  `json:"delivered"`
	Read      int64  `json:"read"`
	Inbound   int64  `json:"inbound"`
}

// InsightContact is one of the most engaged contacts.
type InsightContact struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Phone       string     `json:"phone"`
	Inbound     int64      `json:"inbound_30d"`
	LastInbound *time.Time `json:"last_inbound_at"`
}

// Insights computes the insights of a group, tag or segment. tz names the
// time zone of the daily series (IANA, e.g. Europe/Istanbul).
func (s *Service) Insights(ctx context.Context, kind AudienceKind, id uuid.UUID, f Filter, tz string) (Insights, error) {
	var out Insights
	if _, err := time.LoadLocation(tz); err != nil || tz == "" {
		tz = "UTC"
	}
	where, args, err := f.where()
	if err != nil {
		return out, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite}) // temp table
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout = '20s'"); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, "CREATE TEMP TABLE aud (id uuid PRIMARY KEY) ON COMMIT DROP"); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO aud SELECT c.id FROM contacts c WHERE "+where, args...); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, "ANALYZE aud"); err != nil {
		return out, err
	}

	joined := "c.created_at"
	switch kind {
	case AudienceGroup:
		joined = "(SELECT gm.added_at FROM contact_group_members gm WHERE gm.group_id = $1 AND gm.contact_id = c.id)"
	case AudienceTag:
		joined = "(SELECT ct.created_at FROM contact_tags ct WHERE ct.tag_id = $1 AND ct.contact_id = c.id)"
	}
	sm := &out.Summary
	if err := tx.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE c.opted_out_at IS NULL),
		       count(*) FILTER (WHERE c.opted_out_at IS NOT NULL),
		       count(*) FILTER (WHERE c.last_inbound_at > now() - interval '7 days'),
		       count(*) FILTER (WHERE c.last_inbound_at > now() - interval '30 days'),
		       count(*) FILTER (WHERE `+joined+` > now() - interval '30 days'),
		       count(*) FILTER (WHERE c.email IS NOT NULL AND c.email::text <> '')
		FROM contacts c JOIN aud a ON a.id = c.id
		WHERE $1::uuid IS NOT NULL`, id).Scan(&sm.Contacts, &sm.Reachable, &sm.OptedOut, &sm.Active7, &sm.Active30, &sm.New30, &sm.WithEmail); err != nil {
		return out, fmt.Errorf("insights summary: %w", err)
	}

	mg := &out.Messaging
	if err := tx.QueryRow(ctx, `
		WITH m AS (
		    SELECT m.* FROM messages m JOIN aud a ON a.id = m.contact_id
		    WHERE m.created_at > now() - interval '90 days')
		SELECT count(*) FILTER (WHERE direction = 'outbound'),
		       count(*) FILTER (WHERE direction = 'outbound' AND sent_at IS NOT NULL),
		       count(*) FILTER (WHERE direction = 'outbound' AND delivered_at IS NOT NULL),
		       count(*) FILTER (WHERE direction = 'outbound' AND read_at IS NOT NULL),
		       count(*) FILTER (WHERE direction = 'outbound' AND status = 'failed'),
		       count(*) FILTER (WHERE direction = 'inbound'),
		       count(DISTINCT contact_id) FILTER (WHERE direction = 'outbound' AND sent_at IS NOT NULL),
		       count(DISTINCT o.contact_id) FILTER (WHERE o.direction = 'outbound' AND o.sent_at IS NOT NULL AND EXISTS (
		           SELECT 1 FROM messages i WHERE i.contact_id = o.contact_id AND i.direction = 'inbound'
		             AND i.occurred_at > o.sent_at AND i.occurred_at <= o.sent_at + interval '72 hours')),
		       count(*) FILTER (WHERE direction = 'outbound' AND campaign_id IS NOT NULL)
		FROM m o`).Scan(&mg.Outbound, &mg.Sent, &mg.Delivered, &mg.Read, &mg.Failed, &mg.Inbound, &mg.Messaged, &mg.Replied, &mg.FromCampaign); err != nil {
		return out, fmt.Errorf("insights messaging: %w", err)
	}

	rows, err := tx.Query(ctx, `
		WITH r AS (
		    SELECT cr.campaign_id, count(*) AS reached, count(*) FILTER (WHERE cr.state = 'skipped') AS skipped
		    FROM campaign_recipients cr JOIN aud a ON a.id = cr.contact_id
		    GROUP BY cr.campaign_id),
		mm AS (
		    SELECT m.campaign_id,
		           count(*) FILTER (WHERE m.sent_at IS NOT NULL) AS sent,
		           count(*) FILTER (WHERE m.delivered_at IS NOT NULL) AS delivered,
		           count(*) FILTER (WHERE m.read_at IS NOT NULL) AS read,
		           count(*) FILTER (WHERE m.status = 'failed') AS failed,
		           count(DISTINCT m.contact_id) FILTER (WHERE m.sent_at IS NOT NULL AND EXISTS (
		               SELECT 1 FROM messages i WHERE i.contact_id = m.contact_id AND i.direction = 'inbound'
		                 AND i.occurred_at > m.sent_at AND i.occurred_at <= m.sent_at + interval '72 hours')) AS replied
		    FROM messages m JOIN aud a ON a.id = m.contact_id
		    WHERE m.campaign_id IS NOT NULL
		    GROUP BY m.campaign_id),
		oo AS (
		    SELECT cr.campaign_id, count(*) AS opted_out
		    FROM campaign_recipients cr JOIN aud a ON a.id = cr.contact_id
		    JOIN contacts c ON c.id = cr.contact_id
		    JOIN campaigns k ON k.id = cr.campaign_id
		    WHERE c.opted_out_at IS NOT NULL AND k.started_at IS NOT NULL AND c.opted_out_at > k.started_at
		    GROUP BY cr.campaign_id)
		SELECT k.id, k.name, k.status, k.template_name, k.started_at, k.finished_at, k.audience, k.total_recipients,
		       r.reached, r.skipped, coalesce(mm.sent, 0), coalesce(mm.delivered, 0), coalesce(mm.read, 0),
		       coalesce(mm.failed, 0), coalesce(mm.replied, 0), coalesce(oo.opted_out, 0)
		FROM r JOIN campaigns k ON k.id = r.campaign_id
		LEFT JOIN mm ON mm.campaign_id = k.id
		LEFT JOIN oo ON oo.campaign_id = k.id
		ORDER BY coalesce(k.started_at, k.created_at) DESC
		LIMIT 100`)
	if err != nil {
		return out, fmt.Errorf("insights campaigns: %w", err)
	}
	for rows.Next() {
		var c InsightCampaign
		var aud []byte
		if err := rows.Scan(&c.ID, &c.Name, &c.Status, &c.Template, &c.StartedAt, &c.FinishedAt, &aud, &c.Total,
			&c.Reached, &c.Skipped, &c.Sent, &c.Delivered, &c.Read, &c.Failed, &c.Replied, &c.OptedOut); err != nil {
			rows.Close()
			return out, err
		}
		var a struct {
			GroupID   *uuid.UUID `json:"group_id"`
			TagID     *uuid.UUID `json:"tag_id"`
			SegmentID *uuid.UUID `json:"segment_id"`
		}
		_ = json.Unmarshal(aud, &a)
		switch kind {
		case AudienceGroup:
			c.Targeted = a.GroupID != nil && *a.GroupID == id
		case AudienceTag:
			c.Targeted = a.TagID != nil && *a.TagID == id
		case AudienceSegment:
			c.Targeted = a.SegmentID != nil && *a.SegmentID == id
		}
		out.Campaigns = append(out.Campaigns, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = tx.Query(ctx, `
		WITH days AS (
		    SELECT generate_series((now() AT TIME ZONE $1)::date - 29, (now() AT TIME ZONE $1)::date, interval '1 day')::date AS day),
		m AS (
		    SELECT m.* FROM messages m JOIN aud a ON a.id = m.contact_id
		    WHERE m.created_at > now() - interval '31 days')
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       (SELECT count(*) FROM m WHERE m.direction = 'outbound' AND (m.sent_at AT TIME ZONE $1)::date = d.day),
		       (SELECT count(*) FROM m WHERE m.direction = 'outbound' AND (m.delivered_at AT TIME ZONE $1)::date = d.day),
		       (SELECT count(*) FROM m WHERE m.direction = 'outbound' AND (m.read_at AT TIME ZONE $1)::date = d.day),
		       (SELECT count(*) FROM m WHERE m.direction = 'inbound' AND (m.occurred_at AT TIME ZONE $1)::date = d.day)
		FROM days d ORDER BY d.day`, tz)
	if err != nil {
		return out, fmt.Errorf("insights daily: %w", err)
	}
	for rows.Next() {
		var d InsightDay
		if err := rows.Scan(&d.Day, &d.Sent, &d.Delivered, &d.Read, &d.Inbound); err != nil {
			rows.Close()
			return out, err
		}
		out.Daily = append(out.Daily, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = tx.Query(ctx, `
		SELECT c.id, coalesce(nullif(c.name, ''), c.profile_name), c.phone_e164, count(m.id), c.last_inbound_at
		FROM aud a JOIN contacts c ON c.id = a.id
		JOIN messages m ON m.contact_id = c.id AND m.direction = 'inbound' AND m.occurred_at > now() - interval '30 days'
		GROUP BY c.id
		ORDER BY count(m.id) DESC, c.last_inbound_at DESC NULLS LAST
		LIMIT 5`)
	if err != nil {
		return out, fmt.Errorf("insights engaged: %w", err)
	}
	for rows.Next() {
		var c InsightContact
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Inbound, &c.LastInbound); err != nil {
			rows.Close()
			return out, err
		}
		out.Engaged = append(out.Engaged, c)
	}
	rows.Close()
	if out.Campaigns == nil {
		out.Campaigns = []InsightCampaign{}
	}
	if out.Engaged == nil {
		out.Engaged = []InsightContact{}
	}
	return out, rows.Err()
}
