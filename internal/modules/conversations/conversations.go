// Package conversations is the inbox: one conversation per customer and
// business number, derived from messages (see messaging/conversation.go,
// which keeps it current in the transaction of every message write).
//
// Visibility rules, enforced on every operation:
//   - conversations.read_all sees every conversation;
//   - conversations.read alone sees conversations assigned to the caller or
//     to nobody.
//
// A conversation the caller may not see answers 404, exactly like one that
// does not exist.
package conversations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Permissions.
const (
	PermRead    = "conversations.read"
	PermReadAll = "conversations.read_all"
)

// Service implements the inbox.
type Service struct {
	pool  *pgxpool.Pool
	q     *db.Queries
	msg   *messaging.Service
	media *messaging.MediaStore
	log   *slog.Logger
}

// New returns the service.
func New(pool *pgxpool.Pool, msg *messaging.Service, media *messaging.MediaStore, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), msg: msg, media: media, log: log}
}

var errNotFound = apperr.NotFound("Conversation not found.")

// SeesAll reports whether p may read every conversation.
func SeesAll(p *principal.Principal) bool { return p != nil && p.Has(PermReadAll) }

// Visible reports whether p may read a conversation with the given assignee.
func Visible(p *principal.Principal, assignee *uuid.UUID) bool {
	switch {
	case p == nil:
		return false
	case p.Has(PermReadAll):
		return true
	case !p.Has(PermRead):
		return false
	}
	return assignee == nil || *assignee == p.ID
}

// load returns a conversation visible to p.
func (s *Service) load(ctx context.Context, q *db.Queries, p *principal.Principal, id uuid.UUID) (db.Conversation, error) {
	c, err := q.GetConversation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !Visible(p, c.AssigneeID)) {
		return db.Conversation{}, errNotFound
	}
	return c, err
}

// CanRead is the realtime channel check for users with conversations.read.
func (s *Service) CanRead(ctx context.Context, p *principal.Principal, id uuid.UUID) (bool, error) {
	_, err := s.load(ctx, s.q, p, id)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return err == nil, err
}

// ChangedSince lists the conversations in scope that changed at or after
// since and are visible to p, oldest change first (realtime re-sync).
func (s *Service) ChangedSince(ctx context.Context, p *principal.Principal, scope realtime.ConversationScope, since time.Time, limit int) ([]db.Conversation, error) {
	if p == nil {
		return nil, errNotFound
	}
	return s.q.ListConversationsChangedSince(ctx, db.ListConversationsChangedSinceParams{Since: since, SeeAll: SeesAll(p), ViewerID: p.ID,
		OnlyUnassigned: scope.Unassigned, AssigneeID: scope.Assignee, ID: scope.ID, Lim: int32(min(limit, 1000))}) //nolint:gosec // bounded above
}

// Filter narrows a listing.
type Filter struct {
	Status        *string
	Assignee      string // "", "me", "unassigned" or a user ID
	UnreadOnly    bool
	PhoneNumberID *uuid.UUID
	Query         string
}

// params turns f into query parameters for p.
func (f Filter) params(p *principal.Principal) (db.ListConversationsParams, error) {
	out := db.ListConversationsParams{SeeAll: SeesAll(p), ViewerID: p.ID, Status: f.Status, PhoneNumberID: f.PhoneNumberID, OnlyUnread: f.UnreadOnly}
	switch a := strings.TrimSpace(f.Assignee); a {
	case "":
	case "me":
		out.AssigneeID = &p.ID
	case "unassigned":
		out.OnlyUnassigned = true
	default:
		id, err := uuid.Parse(a)
		if err != nil {
			return out, apperr.Validation("assignee", "Use me, unassigned or a user ID.")
		}
		out.AssigneeID = &id
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		if len([]rune(q)) > 100 {
			return out, apperr.Validation("q", "Search is limited to 100 characters.")
		}
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
		out.Q = &like
	}
	return out, nil
}

// List returns up to limit conversations visible to p, most recent first,
// after the cursor conversation.
func (s *Service) List(ctx context.Context, p *principal.Principal, f Filter, cursor *uuid.UUID, limit int) ([]db.ListConversationsRow, error) {
	params, err := f.params(p)
	if err != nil {
		return nil, err
	}
	if cursor != nil {
		c, err := s.q.GetConversation(ctx, *cursor)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.Validation("cursor", "Invalid cursor")
		}
		if err != nil {
			return nil, err
		}
		params.BeforeAt, params.BeforeID = &c.LastMessageAt, &c.ID
	}
	params.Lim = int32(min(limit, 201)) //nolint:gosec // bounded above
	return s.q.ListConversations(ctx, params)
}

// Get returns a conversation with its contact and number.
func (s *Service) Get(ctx context.Context, p *principal.Principal, id uuid.UUID) (db.ListConversationsRow, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return db.ListConversationsRow{}, err
	}
	r, err := s.q.GetConversationRow(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.ListConversationsRow{}, errNotFound
	}
	return db.ListConversationsRow(r), err
}

// Messages returns a page of the conversation's history, newest first,
// with the media files of those messages.
func (s *Service) Messages(ctx context.Context, p *principal.Principal, id uuid.UUID, before *uuid.UUID, limit int) ([]db.Message, map[uuid.UUID]db.MediaFile, error) {
	c, err := s.load(ctx, s.q, p, id)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.q.ListThreadMessages(ctx, db.ListThreadMessagesParams{ContactID: c.ContactID, PhoneNumberID: c.PhoneNumberID,
		BeforeID: before, Lim: int32(min(limit, 201))}) //nolint:gosec // bounded above
	if err != nil {
		return nil, nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, m := range rows {
		ids[i] = m.ID
	}
	files, err := s.q.ListMediaForMessages(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	media := make(map[uuid.UUID]db.MediaFile, len(files))
	for _, f := range files {
		if f.MessageID != nil {
			media[*f.MessageID] = f
		}
	}
	return rows, media, nil
}

// Reply queues a message in the conversation, from its number. The window
// and opt-out rules are messaging.Queue's.
//
// With params, a template is filled the way campaigns fill it: the values
// are checked against the template, then placeholders such as
// {{first_name}} are resolved from the conversation's contact.
func (s *Service) Reply(ctx context.Context, p *principal.Principal, id uuid.UUID, in messaging.SendInput, params *campaigns.Params) (db.Message, error) {
	c, err := s.load(ctx, s.q, p, id)
	if err != nil {
		return db.Message{}, err
	}
	ct, err := s.q.GetContact(ctx, c.ContactID)
	if err != nil {
		return db.Message{}, err
	}
	if params != nil {
		if in.Template == nil {
			return db.Message{}, apperr.Validation("template", "Template parameters need a template.")
		}
		if len(in.Template.Components) > 0 {
			return db.Message{}, apperr.Validation("template.params", "Send either params or components, not both.")
		}
		pn, err := s.q.GetPhoneNumber(ctx, c.PhoneNumberID)
		if err != nil {
			return db.Message{}, err
		}
		t, err := s.q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: in.Template.Name, Language: in.Template.Language.Code})
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Message{}, apperr.Validation("template.name", "This template is not available on the conversation's WhatsApp account.")
		}
		if err != nil {
			return db.Message{}, err
		}
		sh := campaigns.ShapeOf(t.Components, t.ParameterFormat, t.Category)
		if err := campaigns.CheckParams(sh, *params); err != nil {
			return db.Message{}, err
		}
		tpl, missing := campaigns.Render(sh, *params, ct, t.Name, t.Language)
		if missing != "" {
			return db.Message{}, apperr.Validation("template.params", fmt.Sprintf("{{%s}} is empty for this contact; type a value or add a fallback.", missing))
		}
		in.Template = &tpl
	}
	in.To, in.PhoneNumberID, in.CreatedBy = ct.PhoneE164, &c.PhoneNumberID, &p.ID
	in.Source = "api"
	if p.Kind == principal.KindUser {
		in.Source = "panel"
	}
	return s.msg.Queue(ctx, in)
}

// change applies fn to a visible conversation in a transaction, records the
// history event and audit entry fn returns, and announces the result.
func (s *Service) change(ctx context.Context, p *principal.Principal, id uuid.UUID,
	fn func(q *db.Queries, c db.Conversation) (db.Conversation, *db.InsertConversationEventParams, *audit.Entry, error)) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	c, err := s.load(ctx, q, p, id)
	if err != nil {
		return err
	}
	updated, ev, entry, err := fn(q, c)
	if err != nil {
		return err
	}
	if ev == nil { // nothing changed
		return tx.Commit(ctx)
	}
	ev.ConversationID, ev.ActorType, ev.ActorID = id, string(p.Kind), &p.ID
	if _, err := q.InsertConversationEvent(ctx, *ev); err != nil {
		return err
	}
	if entry != nil {
		entry.EntityType, entry.EntityID = "conversation", &id
		if err := audit.Record(ctx, q, *entry); err != nil {
			return err
		}
	}
	if err := realtime.PublishConversationChange(ctx, tx, c, updated); err != nil {
		return err
	}
	if err := webhooks.ConversationChanged(ctx, tx, ev.Kind, updated, ev.ActorType, ev.ActorID); err != nil {
		return err
	}
	if ev.Kind == "closed" {
		// Closed by a person or an integration (automations close through
		// their own path and never trigger automations).
		if err := trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.ConversationClosed, Key: "closed:" + id.String() + ":" + uuid.Must(uuid.NewV7()).String(),
			ContactID: updated.ContactID, ConversationID: &id, PhoneNumberID: &updated.PhoneNumberID}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Assign gives the conversation to a user, or to nobody with nil.
func (s *Service) Assign(ctx context.Context, p *principal.Principal, id uuid.UUID, assignee *uuid.UUID) error {
	return s.change(ctx, p, id, func(q *db.Queries, c db.Conversation) (db.Conversation, *db.InsertConversationEventParams, *audit.Entry, error) {
		if sameUser(c.AssigneeID, assignee) {
			return c, nil, nil, nil
		}
		if assignee != nil {
			_, err := q.GetAssignableUser(ctx, *assignee)
			if errors.Is(err, pgx.ErrNoRows) {
				return c, nil, nil, apperr.Validation("assignee_id", "Choose an active user who can read conversations.")
			}
			if err != nil {
				return c, nil, nil, err
			}
		}
		updated, err := q.SetConversationAssignee(ctx, db.SetConversationAssigneeParams{ID: id, AssigneeID: assignee})
		if err != nil {
			return c, nil, nil, err
		}
		kind := "assigned"
		if assignee == nil {
			kind = "unassigned"
		}
		return updated, &db.InsertConversationEventParams{Kind: kind, AssigneeID: assignee},
			&audit.Entry{Action: "conversation." + kind, Changes: map[string]any{"assignee_id": []any{c.AssigneeID, assignee}}}, nil
	})
}

// SetStatus closes or reopens the conversation.
func (s *Service) SetStatus(ctx context.Context, p *principal.Principal, id uuid.UUID, status string) error {
	return s.change(ctx, p, id, func(q *db.Queries, c db.Conversation) (db.Conversation, *db.InsertConversationEventParams, *audit.Entry, error) {
		if c.Status == status {
			return c, nil, nil, nil
		}
		updated, err := q.SetConversationStatus(ctx, db.SetConversationStatusParams{ID: id, Status: status})
		if err != nil {
			return c, nil, nil, err
		}
		kind, action := "closed", "conversation.close"
		if status == "open" {
			kind, action = "reopened", "conversation.reopen"
		}
		return updated, &db.InsertConversationEventParams{Kind: kind}, &audit.Entry{Action: action}, nil
	})
}

// MarkRead resets the unread counter.
func (s *Service) MarkRead(ctx context.Context, p *principal.Principal, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	c, err := s.load(ctx, q, p, id)
	if err != nil || c.UnreadCount == 0 {
		return err
	}
	updated, err := q.MarkConversationRead(ctx, id)
	if err != nil {
		return err
	}
	if err := realtime.PublishConversation(ctx, tx, updated); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Events returns the conversation's recent history.
func (s *Service) Events(ctx context.Context, p *principal.Principal, id uuid.UUID) ([]db.ListConversationEventsRow, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return nil, err
	}
	return s.q.ListConversationEvents(ctx, db.ListConversationEventsParams{ConversationID: id, Lim: 100})
}

// OpenMedia opens a stored media file of a message in the conversation.
func (s *Service) OpenMedia(ctx context.Context, p *principal.Principal, id, mediaID uuid.UUID) (db.MediaFile, *os.File, error) {
	c, err := s.load(ctx, s.q, p, id)
	if err != nil {
		return db.MediaFile{}, nil, err
	}
	notFound := apperr.NotFound("Media not found.")
	f, err := s.q.GetMediaFile(ctx, mediaID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && f.MessageID == nil) {
		return f, nil, notFound
	}
	if err != nil {
		return f, nil, err
	}
	m, err := s.q.GetMessage(ctx, *f.MessageID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (m.ContactID != c.ContactID || m.PhoneNumberID != c.PhoneNumberID)) {
		return f, nil, notFound // a file of another conversation
	}
	if err != nil {
		return f, nil, err
	}
	if f.Status != "stored" || f.StorageKey == nil {
		return f, nil, apperr.Conflict("MEDIA_NOT_STORED", "The file has not been downloaded from WhatsApp (yet).")
	}
	file, err := s.media.Open(*f.StorageKey)
	if errors.Is(err, os.ErrNotExist) {
		s.log.ErrorContext(ctx, "stored media file is missing from MEDIA_DIR", "media_id", mediaID)
		return f, nil, notFound
	}
	return f, file, err
}

func sameUser(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// windowExpiry is exported for the handler and tests.
func windowExpiry(lastInbound *time.Time, now time.Time) (open bool, expires *time.Time) {
	return messaging.WindowOpen(lastInbound, now), messaging.WindowExpiresAt(lastInbound)
}
