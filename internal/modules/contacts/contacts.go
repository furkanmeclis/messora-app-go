// Package contacts manages the address book: contacts with personalisation
// fields, tags, groups, notes and consent, segments, and file imports.
package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Service is the contacts API.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	log  *slog.Logger
	imp  *ImportStore
}

// New returns the service; imports are stored under importDir.
func New(pool *pgxpool.Pool, importDir string, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), log: log, imp: &ImportStore{root: importDir}}
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// NormalizePhone turns "+90 555 111 22 33", "905551112233" or
// "0090 555…" into E.164.
func NormalizePhone(s string) (string, bool) {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0, r == ' ', r == '-', r == '(', r == ')', r == '.', r == ' ':
		default:
			return "", false
		}
	}
	d := strings.TrimPrefix(b.String(), "00") // international dialling prefix
	out := "+" + d
	return out, e164.MatchString(out)
}

// Fields are the editable contact fields; nil means "leave unchanged" on
// update. For optional text fields a pointer to "" clears the value.
type Fields struct {
	Name       *string
	Email      *string
	Vars       [5]*string
	Attributes map[string]string
	Tags       *[]string
	GroupIDs   *[]uuid.UUID
}

func cleanOptional(p *string, field string, max int) (*string, error) {
	if p == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*p)
	if len([]rune(v)) > max {
		return nil, apperr.Validation(field, "Too long.")
	}
	return &v, nil
}

func validateFields(f *Fields) error {
	var err error
	if f.Name != nil {
		n := strings.TrimSpace(*f.Name)
		if len([]rune(n)) > 200 {
			return apperr.Validation("name", "Name is limited to 200 characters.")
		}
		f.Name = &n
	}
	if f.Email, err = cleanOptional(f.Email, "email", 254); err != nil {
		return err
	}
	if f.Email != nil && *f.Email != "" {
		a, perr := mail.ParseAddress(*f.Email)
		if perr != nil || a.Address != *f.Email {
			return apperr.Validation("email", "E-mail address is not valid.")
		}
	}
	for i := range f.Vars {
		if f.Vars[i], err = cleanOptional(f.Vars[i], "var"+string(rune('1'+i)), 500); err != nil {
			return err
		}
	}
	if len(f.Attributes) > 50 {
		return apperr.Validation("attributes", "At most 50 extra attributes.")
	}
	for k, v := range f.Attributes {
		if k == "" || len(k) > 60 || len(v) > 1000 {
			return apperr.Validation("attributes", "Attribute names are 1–60 characters, values up to 1000.")
		}
	}
	if f.Tags != nil {
		if len(*f.Tags) > 50 {
			return apperr.Validation("tags", "At most 50 tags per contact.")
		}
		for _, t := range *f.Tags {
			if t = strings.TrimSpace(t); t == "" || len([]rune(t)) > 60 {
				return apperr.Validation("tags", "Tags are 1–60 characters.")
			}
		}
	}
	return nil
}

// nullIfEmpty maps "" to NULL for storage.
func nullIfEmpty(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

// Create adds a contact. A duplicate phone number is a conflict.
func (s *Service) Create(ctx context.Context, phone string, f Fields, source string) (db.Contact, error) {
	e, ok := NormalizePhone(phone)
	if !ok {
		return db.Contact{}, apperr.Validation("phone", "Phone number must be in international format, e.g. +905551112233.")
	}
	if err := validateFields(&f); err != nil {
		return db.Contact{}, err
	}
	name := ""
	if f.Name != nil {
		name = *f.Name
	}
	attrs, _ := json.Marshal(nonNil(f.Attributes))
	var c db.Contact
	err := s.tx(ctx, func(q *db.Queries) error {
		var err error
		c, err = q.InsertContact(ctx, db.InsertContactParams{PhoneE164: e, Name: name, Email: nullIfEmpty(f.Email),
			Var1: nullIfEmpty(f.Vars[0]), Var2: nullIfEmpty(f.Vars[1]), Var3: nullIfEmpty(f.Vars[2]), Var4: nullIfEmpty(f.Vars[3]),
			Var5: nullIfEmpty(f.Vars[4]), Attributes: attrs, Source: source})
		if isUnique(err) {
			return apperr.Conflict("CONTACT_EXISTS", "A contact with this phone number already exists.")
		}
		if err != nil {
			return err
		}
		if err := setTagsAndGroups(ctx, q, c.ID, f); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "contact.create", EntityType: "contact", EntityID: &c.ID})
	})
	return c, err
}

// Update edits a contact.
func (s *Service) Update(ctx context.Context, id uuid.UUID, f Fields) (db.Contact, error) {
	if err := validateFields(&f); err != nil {
		return db.Contact{}, err
	}
	p := db.UpdateContactParams{ID: id, Name: f.Name,
		SetEmail: f.Email != nil, Email: nullIfEmpty(f.Email),
		SetVar1: f.Vars[0] != nil, Var1: nullIfEmpty(f.Vars[0]),
		SetVar2: f.Vars[1] != nil, Var2: nullIfEmpty(f.Vars[1]),
		SetVar3: f.Vars[2] != nil, Var3: nullIfEmpty(f.Vars[2]),
		SetVar4: f.Vars[3] != nil, Var4: nullIfEmpty(f.Vars[3]),
		SetVar5: f.Vars[4] != nil, Var5: nullIfEmpty(f.Vars[4])}
	if f.Attributes != nil {
		p.Attributes, _ = json.Marshal(f.Attributes)
	}
	var c db.Contact
	err := s.tx(ctx, func(q *db.Queries) error {
		var err error
		c, err = q.UpdateContact(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Contact not found.")
		}
		if err != nil {
			return err
		}
		if err := setTagsAndGroups(ctx, q, id, f); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "contact.update", EntityType: "contact", EntityID: &id})
	})
	return c, err
}

func setTagsAndGroups(ctx context.Context, q *db.Queries, id uuid.UUID, f Fields) error {
	if f.Tags != nil {
		if err := q.ClearContactTags(ctx, id); err != nil {
			return err
		}
		for _, name := range *f.Tags {
			t, err := q.UpsertTag(ctx, db.UpsertTagParams{Name: strings.TrimSpace(name), Color: "gray"})
			if err != nil {
				return err
			}
			if err := q.AddContactTag(ctx, db.AddContactTagParams{ContactID: id, TagID: t.ID}); err != nil {
				return err
			}
		}
	}
	if f.GroupIDs != nil {
		current, err := q.ContactGroupIDs(ctx, id)
		if err != nil {
			return err
		}
		want := map[uuid.UUID]bool{}
		for _, g := range *f.GroupIDs {
			want[g] = true
		}
		for _, g := range current {
			if !want[g] {
				if _, err := q.RemoveGroupMembers(ctx, db.RemoveGroupMembersParams{GroupID: g, ContactIds: []uuid.UUID{id}}); err != nil {
					return err
				}
			}
			delete(want, g)
		}
		for g := range want {
			if _, err := q.AddGroupMembers(ctx, db.AddGroupMembersParams{GroupID: g, ContactIds: []uuid.UUID{id}}); err != nil {
				if isFK(err) {
					return apperr.Validation("group_ids", "Unknown group.")
				}
				return err
			}
		}
	}
	return nil
}

// Delete removes a contact and its history.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		n, err := q.DeleteContact(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound("Contact not found.")
		}
		return audit.Record(ctx, q, audit.Entry{Action: "contact.delete", EntityType: "contact", EntityID: &id})
	})
}

// Get returns a contact with its tags and groups.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Contact, []db.Tag, []uuid.UUID, error) {
	c, err := s.q.GetContact(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, nil, nil, apperr.NotFound("Contact not found.")
	}
	if err != nil {
		return c, nil, nil, err
	}
	tags, err := s.q.ContactTags(ctx, id)
	if err != nil {
		return c, nil, nil, err
	}
	groups, err := s.q.ContactGroupIDs(ctx, id)
	return c, tags, groups, err
}

// SetConsent records an opt-in or opt-out decision.
func (s *Service) SetConsent(ctx context.Context, id uuid.UUID, optIn bool, source, note string, actor *uuid.UUID) error {
	if len(note) > 500 {
		return apperr.Validation("note", "Note is limited to 500 characters.")
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.GetContact(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Contact not found.")
		} else if err != nil {
			return err
		}
		kind := "opt_out"
		if optIn {
			kind = "opt_in"
		}
		if _, err := q.InsertConsentEvent(ctx, db.InsertConsentEventParams{ContactID: id, Kind: kind, Source: source, Note: note, ActorID: actor}); err != nil {
			return err
		}
		if err := q.SetContactOptOut(ctx, db.SetContactOptOutParams{ID: id, OptedOut: !optIn, Reason: nullIfEmpty(&note)}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "contact." + kind, EntityType: "contact", EntityID: &id})
	})
}

func (s *Service) tx(ctx context.Context, fn func(*db.Queries) error) error {
	return s.txx(ctx, func(_ pgx.Tx, q *db.Queries) error { return fn(q) })
}

func (s *Service) txx(ctx context.Context, fn func(pgx.Tx, *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func isFK(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23503"
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
