package contacts

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
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
