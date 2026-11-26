package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// fieldValues are a contact's field values by provenance key (name,
// email, var1…var5, attr:<key>).
func fieldValues(c db.Contact) map[string]string {
	v := map[string]string{"name": c.Name, "email": deref(c.Email)}
	for i, p := range []*string{c.Var1, c.Var2, c.Var3, c.Var4, c.Var5} {
		v[fmt.Sprintf("var%d", i+1)] = deref(p)
	}
	var attrs map[string]any
	if len(c.Attributes) > 0 && json.Unmarshal(c.Attributes, &attrs) == nil {
		for k, x := range attrs {
			if x != nil {
				v["attr:"+k] = fmt.Sprint(x)
			}
		}
	}
	return v
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// recordEdits marks the fields a person (or an API key) changed as theirs
// and verified, and forgets the provenance of cleared fields. A value the
// AI collected and a person then edited is the person's.
func recordEdits(ctx context.Context, q *db.Queries, before, after db.Contact) error {
	source, actor := "user", (*uuid.UUID)(nil)
	if p := principal.From(ctx); p != nil {
		if p.Kind == principal.KindAPIKey {
			source = "api"
		} else {
			id := p.ID
			actor = &id
		}
	}
	old, cur := fieldValues(before), fieldValues(after)
	keys := make([]string, 0, len(old)+len(cur))
	for k := range old {
		keys = append(keys, k)
	}
	for k := range cur {
		if _, ok := old[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		if old[k] == cur[k] {
			continue
		}
		if cur[k] == "" {
			if err := q.DeleteContactFieldSource(ctx, db.DeleteContactFieldSourceParams{ContactID: after.ID, Field: k}); err != nil {
				return err
			}
			continue
		}
		if !provenanceKey(k) {
			continue
		}
		if err := q.UpsertContactFieldSource(ctx, db.UpsertContactFieldSourceParams{ContactID: after.ID, Field: k, Source: source,
			Verified: true, VerifiedBy: actor}); err != nil {
			return err
		}
	}
	return nil
}

var fieldKeyRe = regexp.MustCompile(`^(name|email|var[1-5]|attr:[A-Za-z0-9_.-]{1,64})$`)

// provenanceKey reports whether k fits contact_field_sources.field.
func provenanceKey(k string) bool { return fieldKeyRe.MatchString(k) }

// FieldSources returns where each field of a contact came from.
func (s *Service) FieldSources(ctx context.Context, id uuid.UUID) ([]db.ContactFieldSource, error) {
	return s.q.ListContactFieldSources(ctx, id)
}

// VerifyFields marks values the AI collected as checked by a person. It
// returns the fields that were verified now.
func (s *Service) VerifyFields(ctx context.Context, id uuid.UUID, fields []string, actor *uuid.UUID) ([]string, error) {
	if len(fields) == 0 || len(fields) > 60 {
		return nil, apperr.Validation("fields", "Name 1–60 fields to verify.")
	}
	for _, f := range fields {
		if !provenanceKey(f) {
			return nil, apperr.Validation("fields", fmt.Sprintf("%q is not a contact field (name, email, var1…var5, attr:<key>).", f))
		}
	}
	var done []string
	err := s.txx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		if _, err := q.GetContact(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("Contact not found.")
			}
			return err
		}
		var err error
		done, err = q.VerifyContactFieldSources(ctx, db.VerifyContactFieldSourcesParams{ContactID: id, Fields: fields, VerifiedBy: actor})
		if err != nil || len(done) == 0 {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "contact.verify_fields", EntityType: "contact", EntityID: &id,
			Changes: map[string]any{"fields": done}})
	})
	return done, err
}
