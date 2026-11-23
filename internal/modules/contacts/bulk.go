package contacts

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Sort orders a contact list.
type Sort string

// Sort orders.
const (
	SortNewest      Sort = "newest"
	SortOldest      Sort = "oldest"
	SortNameAsc     Sort = "name_asc"
	SortNameDesc    Sort = "name_desc"
	SortLastMessage Sort = "last_message"
)

// Every order ends with the id so pages are stable.
var orderBy = map[Sort]string{
	SortNewest:      "c.id DESC",
	SortOldest:      "c.id ASC",
	SortNameAsc:     "lower(coalesce(nullif(c.name, ''), c.profile_name, c.phone_e164)) ASC, c.id",
	SortNameDesc:    "lower(coalesce(nullif(c.name, ''), c.profile_name, c.phone_e164)) DESC, c.id DESC",
	SortLastMessage: "c.last_inbound_at DESC NULLS LAST, c.id DESC",
}

// ListPage returns one page (by offset) of matching contacts and the total.
func (s *Service) ListPage(ctx context.Context, f Filter, sort Sort, page, limit int) ([]db.Contact, int64, error) {
	order, ok := orderBy[sort]
	if !ok {
		order = orderBy[SortNewest]
	}
	where, args, err := f.where()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM contacts c WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, (page-1)*limit)
	sql := fmt.Sprintf("SELECT c.* FROM contacts c WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d", where, order, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[db.Contact])
	return out, total, err
}

// MaxBulk bounds one bulk action.
const MaxBulk = 200_000

// BulkAction is a change applied to many contacts at once.
type BulkAction string

// Bulk actions.
const (
	BulkAddTag          BulkAction = "add_tag"
	BulkRemoveTag       BulkAction = "remove_tag"
	BulkAddToGroup      BulkAction = "add_to_group"
	BulkRemoveFromGroup BulkAction = "remove_from_group"
	BulkOptOut          BulkAction = "opt_out"
	BulkOptIn           BulkAction = "opt_in"
	BulkDelete          BulkAction = "delete"
)

// BulkRequest selects contacts by ids or by filter (minus Exclude).
type BulkRequest struct {
	Action  BulkAction
	IDs     []uuid.UUID
	Filter  *Filter
	Exclude []uuid.UUID
	TagID   *uuid.UUID
	TagName string
	GroupID *uuid.UUID
	Actor   *uuid.UUID
}

// BulkResult reports how many contacts were selected and changed.
type BulkResult struct {
	Matched  int64
	Affected int64
}

// Bulk applies one action to the selected contacts in a single transaction.
func (s *Service) Bulk(ctx context.Context, req BulkRequest) (BulkResult, error) {
	var res BulkResult
	if (req.Filter == nil) == (len(req.IDs) == 0) {
		return res, apperr.Validation("ids", "Choose contacts either by ids or by filter.")
	}
	// The target set, materialised once into a temporary table.
	var where string
	var args []any
	if req.Filter != nil {
		w, a, err := req.Filter.where()
		if err != nil {
			return res, err
		}
		where, args = w, a
	} else {
		args = []any{req.IDs}
		where = "c.id = ANY($1::uuid[])"
	}
	if len(req.Exclude) > 0 {
		args = append(args, req.Exclude)
		where += fmt.Sprintf(" AND NOT (c.id = ANY($%d::uuid[]))", len(args))
	}

	err := s.txx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := tx.Exec(ctx, "CREATE TEMP TABLE bulk_target (id uuid PRIMARY KEY) ON COMMIT DROP"); err != nil {
			return err
		}
		args := append(args, MaxBulk+1)
		tag, err := tx.Exec(ctx, fmt.Sprintf("INSERT INTO bulk_target SELECT c.id FROM contacts c WHERE %s LIMIT $%d", where, len(args)), args...)
		if err != nil {
			return err
		}
		res.Matched = tag.RowsAffected()
		if res.Matched > MaxBulk {
			return apperr.Validation("", fmt.Sprintf("A bulk action is limited to %d contacts. Narrow the selection.", MaxBulk))
		}
		changes := map[string]any{"matched": res.Matched}
		switch req.Action {
		case BulkAddTag:
			t, err := s.bulkTag(ctx, q, req)
			if err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `INSERT INTO contact_tags (contact_id, tag_id)
				SELECT id, $1 FROM bulk_target ON CONFLICT DO NOTHING RETURNING contact_id`, t.ID)
			if err != nil {
				return err
			}
			added, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			res.Affected = int64(len(added))
			now := time.Now().UnixNano()
			for _, id := range added {
				if err := trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.TagAdded,
					Key: fmt.Sprintf("tag:%s:%s:%d", id, t.ID, now), ContactID: id, Tag: t.Name}); err != nil {
					return err
				}
			}
			changes["tag"] = t.Name
		case BulkRemoveTag:
			if req.TagID == nil {
				return apperr.Validation("tag_id", "Choose a tag.")
			}
			tag, err := tx.Exec(ctx, `DELETE FROM contact_tags WHERE tag_id = $1 AND contact_id IN (SELECT id FROM bulk_target)`, *req.TagID)
			if err != nil {
				return err
			}
			res.Affected = tag.RowsAffected()
			changes["tag_id"] = req.TagID.String()
		case BulkAddToGroup, BulkRemoveFromGroup:
			if req.GroupID == nil {
				return apperr.Validation("group_id", "Choose a group.")
			}
			if _, err := q.GetContactGroup(ctx, *req.GroupID); err != nil {
				if err == pgx.ErrNoRows {
					return apperr.Validation("group_id", "Unknown group.")
				}
				return err
			}
			sql := `INSERT INTO contact_group_members (group_id, contact_id) SELECT $1, id FROM bulk_target ON CONFLICT DO NOTHING`
			if req.Action == BulkRemoveFromGroup {
				sql = `DELETE FROM contact_group_members WHERE group_id = $1 AND contact_id IN (SELECT id FROM bulk_target)`
			}
			tag, err := tx.Exec(ctx, sql, *req.GroupID)
			if err != nil {
				return err
			}
			res.Affected = tag.RowsAffected()
			changes["group_id"] = req.GroupID.String()
		case BulkOptOut, BulkOptIn:
			optOut := req.Action == BulkOptOut
			// Only contacts whose state changes get a consent record.
			rows, err := tx.Query(ctx, `UPDATE contacts c SET
					opted_out_at = CASE WHEN $1 THEN now() ELSE NULL END,
					opt_out_reason = NULL,
					updated_at = now()
				WHERE c.id IN (SELECT id FROM bulk_target) AND (c.opted_out_at IS NULL) = $1
				RETURNING c.id`, optOut)
			if err != nil {
				return err
			}
			changed, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			kind, typ := "opt_in", webhooks.TypeContactOptedIn
			if optOut {
				kind, typ = "opt_out", webhooks.TypeContactOptedOut
			}
			if _, err := tx.Exec(ctx, `INSERT INTO consent_events (contact_id, kind, source, note, actor_id)
				SELECT unnest($1::uuid[]), $2, 'manual', 'bulk action', $3`, changed, kind, req.Actor); err != nil {
				return err
			}
			for _, id := range changed {
				if err := webhooks.ContactChanged(ctx, tx, typ, id); err != nil {
					return err
				}
			}
			res.Affected = int64(len(changed))
		case BulkDelete:
			tag, err := tx.Exec(ctx, `DELETE FROM contacts WHERE id IN (SELECT id FROM bulk_target)`)
			if err != nil {
				return err
			}
			res.Affected = tag.RowsAffected()
		default:
			return apperr.Validation("action", "Unknown action.")
		}
		changes["affected"] = res.Affected
		return audit.Record(ctx, q, audit.Entry{Action: "contact.bulk_" + string(req.Action), EntityType: "contact", Changes: changes})
	})
	return res, err
}

func (s *Service) bulkTag(ctx context.Context, q *db.Queries, req BulkRequest) (db.Tag, error) {
	if req.TagID != nil {
		t, err := q.GetTag(ctx, *req.TagID)
		if err == pgx.ErrNoRows {
			return t, apperr.Validation("tag_id", "Unknown tag.")
		}
		return t, err
	}
	if req.TagName == "" {
		return db.Tag{}, apperr.Validation("tag_id", "Choose a tag.")
	}
	return q.UpsertTag(ctx, db.UpsertTagParams{Name: req.TagName, Color: "gray"})
}
