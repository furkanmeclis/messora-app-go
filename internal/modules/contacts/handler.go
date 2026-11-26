package contacts

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts/segment"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves contacts, tags, groups, segments and imports.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

func contactToAPI(c db.Contact, tags []db.Tag, groups []uuid.UUID) api.Contact {
	attrs := map[string]string{}
	_ = json.Unmarshal(c.Attributes, &attrs)
	refs := make([]api.TagRef, len(tags))
	for i, t := range tags {
		refs[i] = api.TagRef{Id: t.ID, Name: t.Name, Color: t.Color}
	}
	if groups == nil {
		groups = []uuid.UUID{}
	}
	return api.Contact{Id: c.ID, Phone: c.PhoneE164, Name: c.Name, ProfileName: c.ProfileName, Email: c.Email,
		Var1: c.Var1, Var2: c.Var2, Var3: c.Var3, Var4: c.Var4, Var5: c.Var5, Attributes: attrs, Tags: refs,
		GroupIds: groups, OptedOut: c.OptedOutAt != nil, OptedOutAt: c.OptedOutAt, LastInboundAt: c.LastInboundAt,
		Consent: ConsentToAPI(c), Source: api.ContactSource(c.Source), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		ContactType: c.ContactType, CreatedByInboundAt: c.CreatedByInboundAt, ProfileStatus: (*api.ContactProfileStatus)(c.ProfileStatus),
		ProfileProgress: progressToAPI(c.ProfileProgress)}
}

func progressToAPI(raw []byte) *api.ContactProfileProgress {
	if len(raw) == 0 {
		return nil
	}
	var p api.ContactProfileProgress
	if json.Unmarshal(raw, &p) != nil {
		return nil
	}
	if p.Missing == nil {
		p.Missing = []string{}
	}
	return &p
}

func sourcesToAPI(rows []db.ContactFieldSource) []api.ContactFieldSource {
	out := make([]api.ContactFieldSource, len(rows))
	for i, r := range rows {
		out[i] = api.ContactFieldSource{Field: r.Field, Source: api.ContactFieldSourceSource(r.Source), Verified: r.Verified,
			CollectedAt: r.CollectedAt, VerifiedAt: r.VerifiedAt, ConversationId: r.ConversationID}
	}
	return out
}

// ConsentToAPI is a contact's consent as the API shows it (the legacy
// opted_out_at counts as opted out of both categories).
func ConsentToAPI(c db.Contact) api.ContactConsent {
	st := consent.StateOf(c)
	return api.ContactConsent{
		Marketing: api.ConsentCategoryState{Status: api.ConsentStatus(st.Marketing.Status), UpdatedAt: st.Marketing.UpdatedAt,
			Source: c.MarketingConsentSource, Reason: c.MarketingConsentReason},
		Service: api.ConsentCategoryState{Status: api.ConsentStatus(st.Service.Status), UpdatedAt: st.Service.UpdatedAt,
			Source: c.ServiceConsentSource, Reason: c.ServiceConsentReason},
		MetaMarketingStoppedAt: c.MetaMarketingStoppedAt, UndeliverableAt: c.UndeliverableAt, UndeliverableCode: c.UndeliverableCode,
	}
}

func fieldsFrom(name, email *string, vars [5]*string, attrs *map[string]string, tags *[]string, groups *[]uuid.UUID, contactType *string) Fields {
	f := Fields{Name: name, Email: email, Vars: vars, Tags: tags, GroupIDs: groups, ContactType: contactType}
	if attrs != nil {
		f.Attributes = *attrs
	}
	return f
}

func (h *Handler) writeContact(w http.ResponseWriter, r *http.Request, status int, id uuid.UUID) {
	c, tags, groups, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sources, err := h.svc.FieldSources(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := contactToAPI(c, tags, groups)
	fs := sourcesToAPI(sources)
	out.FieldSources = &fs
	httpx.JSON(w, r, status, out)
}

// VerifyContactFields implements api.ServerInterface.
func (h *Handler) VerifyContactFields(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	var req api.VerifyContactFieldsRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	var actor *uuid.UUID
	if p := principal.From(r.Context()); p != nil && p.Kind == principal.KindUser {
		uid := p.ID
		actor = &uid
	}
	if _, err := h.svc.VerifyFields(r.Context(), id, req.Fields, actor); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeContact(w, r, http.StatusOK, id)
}

// segmentFilter loads a saved segment into a filter.
func (h *Handler) filter(r *http.Request, q *string, tag, group, seg *uuid.UUID, optedOut *bool,
	marketing, service *api.ConsentStatus, reach *api.Reachability) (Filter, error) {
	f := Filter{TagID: tag, GroupID: group, OptedOut: optedOut}
	if marketing != nil {
		f.MarketingConsent = (*string)(marketing)
	}
	if service != nil {
		f.ServiceConsent = (*string)(service)
	}
	if reach != nil {
		f.Reachability = (*string)(reach)
	}
	if q != nil {
		f.Query = *q
	}
	if seg != nil {
		s, err := h.svc.q.GetSegment(r.Context(), *seg)
		if errors.Is(err, pgx.ErrNoRows) {
			return f, apperr.Validation("segment_id", "Unknown segment.")
		}
		if err != nil {
			return f, err
		}
		n, err := segment.Parse(s.Definition)
		if err != nil {
			return f, apperr.Validation("segment_id", "The saved segment is no longer valid: "+err.Error())
		}
		f.Segment = n
	}
	return f, nil
}

// ListContacts implements api.ServerInterface.
func (h *Handler) ListContacts(w http.ResponseWriter, r *http.Request, p api.ListContactsParams) {
	f, err := h.filter(r, p.Q, p.TagId, p.GroupId, p.SegmentId, p.OptedOut, p.MarketingConsent, p.ServiceConsent, p.Reachability)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f.AIProfile = p.AiProfile
	limit := 50
	if p.Limit != nil {
		limit = *p.Limit
	}
	if p.Page != nil || p.Sort != nil {
		page := 1
		if p.Page != nil {
			page = *p.Page
		}
		sort := SortNewest
		if p.Sort != nil {
			sort = Sort(*p.Sort)
		}
		rows, total, err := h.svc.ListPage(r.Context(), f, sort, page, limit)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.page(r, rows)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpx.JSONTotal(w, r, http.StatusOK, out, total)
		return
	}
	var before *uuid.UUID
	if p.Cursor != nil {
		id, err := uuid.Parse(*p.Cursor)
		if err != nil {
			h.fail(w, r, apperr.Validation("cursor", "Invalid cursor"))
			return
		}
		before = &id
	}
	rows, err := h.svc.List(r.Context(), f, before, limit+1)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	out, err := h.page(r, rows)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

func (h *Handler) page(r *http.Request, rows []db.Contact) ([]api.Contact, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, c := range rows {
		ids[i] = c.ID
	}
	tags, err := h.svc.TagsFor(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	out := make([]api.Contact, len(rows))
	for i, c := range rows {
		out[i] = contactToAPI(c, tags[c.ID], nil)
	}
	return out, nil
}

// CreateContact implements api.ServerInterface.
func (h *Handler) CreateContact(w http.ResponseWriter, r *http.Request, _ api.CreateContactParams) {
	var req api.CreateContactRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	source := "manual"
	if principal.From(r.Context()).Kind != principal.KindUser {
		source = "api"
	}
	c, err := h.svc.Create(r.Context(), req.Phone, fieldsFrom(req.Name, req.Email,
		[5]*string{req.Var1, req.Var2, req.Var3, req.Var4, req.Var5}, req.Attributes, req.Tags, req.GroupIds, req.ContactType), source)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeContact(w, r, http.StatusCreated, c.ID)
}

// GetContact implements api.ServerInterface.
func (h *Handler) GetContact(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	h.writeContact(w, r, http.StatusOK, id)
}

// UpdateContact implements api.ServerInterface.
func (h *Handler) UpdateContact(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	var req api.UpdateContactRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if _, err := h.svc.Update(r.Context(), id, fieldsFrom(req.Name, req.Email,
		[5]*string{req.Var1, req.Var2, req.Var3, req.Var4, req.Var5}, req.Attributes, req.Tags, req.GroupIds, req.ContactType)); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeContact(w, r, http.StatusOK, id)
}

// DeleteContact implements api.ServerInterface.
func (h *Handler) DeleteContact(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// RecordConsent implements api.ServerInterface.
func (h *Handler) RecordConsent(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	var req api.ConsentRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p := principal.From(r.Context())
	in := ConsentInput{Source: consent.SourceManual, Actor: &p.ID}
	if p.Kind != principal.KindUser {
		in.Source = consent.SourceAPI
	}
	if req.Note != nil {
		in.Reason = strings.TrimSpace(*req.Note)
	}
	switch {
	case req.Status != nil && req.Kind != nil:
		h.fail(w, r, apperr.Validation("status", "Give either kind or status, not both."))
		return
	case req.Status != nil:
		in.Status = consent.Status(*req.Status)
		if req.Category != nil && *req.Category != api.ConsentRequestCategoryAll {
			in.Categories = []string{string(*req.Category)}
		}
	case req.Kind != nil:
		// Legacy: both categories.
		if req.Category != nil && *req.Category != api.ConsentRequestCategoryAll {
			h.fail(w, r, apperr.Validation("category", "kind always changes both categories; use status for one."))
			return
		}
		in.Status = consent.OptedOut
		if *req.Kind == api.ConsentRequestKindOptIn {
			in.Status = consent.OptedIn
		}
	default:
		h.fail(w, r, apperr.Validation("status", "Give kind or status."))
		return
	}
	if err := h.svc.SetConsent(r.Context(), id, in); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeContact(w, r, http.StatusOK, id)
}

// ListConsentEvents implements api.ServerInterface.
func (h *Handler) ListConsentEvents(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	rows, err := h.svc.ConsentEvents(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ConsentEvent, len(rows))
	for i, e := range rows {
		var actor *string
		if e.ActorName != "" {
			actor = &e.ActorName
		}
		out[i] = api.ConsentEvent{Id: e.ID, Kind: api.ConsentEventKind(e.Kind), Category: e.Category, Status: e.Status,
			Source: api.ConsentEventSource(e.Source), Note: e.Note, ActorName: actor, CreatedAt: e.CreatedAt}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ── Notes ───────────────────────────────────────────────────────────────────

// ListContactNotes implements api.ServerInterface.
func (h *Handler) ListContactNotes(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	rows, err := h.svc.q.ListContactNotes(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.Note, len(rows))
	for i, n := range rows {
		out[i] = api.Note{Id: n.ID, Body: n.Body, AuthorName: n.AuthorName, CreatedAt: n.CreatedAt}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// AddContactNote implements api.ServerInterface.
func (h *Handler) AddContactNote(w http.ResponseWriter, r *http.Request, id api.ContactId) {
	var req api.AddContactNoteJSONBody
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		h.fail(w, r, apperr.Validation("body", "Note is empty."))
		return
	}
	p := principal.From(r.Context())
	var note db.ContactNote
	err := h.svc.tx(r.Context(), func(q *db.Queries) error {
		if _, err := q.GetContact(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Contact not found.")
		} else if err != nil {
			return err
		}
		var err error
		note, err = q.InsertContactNote(r.Context(), db.InsertContactNoteParams{ContactID: id, AuthorID: &p.ID, Body: body})
		return err
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	author := ""
	if u, err := h.svc.q.GetUserByID(r.Context(), p.ID); err == nil {
		author = u.Name
	}
	httpx.JSON(w, r, http.StatusCreated, api.Note{Id: note.ID, Body: note.Body, AuthorName: author, CreatedAt: note.CreatedAt})
}

// DeleteContactNote implements api.ServerInterface.
func (h *Handler) DeleteContactNote(w http.ResponseWriter, r *http.Request, id api.ContactId, noteID api.NoteId) {
	n, err := h.svc.q.DeleteContactNote(r.Context(), db.DeleteContactNoteParams{ID: noteID, ContactID: id})
	if err == nil && n == 0 {
		err = apperr.NotFound("Note not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ── Export ──────────────────────────────────────────────────────────────────

// ExportColumns is the header of exported CSV files (re-importable).
var ExportColumns = []string{"phone", "name", "profile_name", "email", "var1", "var2", "var3", "var4", "var5", "tags", "opted_out", "created_at"}

// ExportContacts implements api.ServerInterface. Rows are streamed.
func (h *Handler) ExportContacts(w http.ResponseWriter, r *http.Request, p api.ExportContactsParams) {
	f, err := h.filter(r, p.Q, p.TagId, p.GroupId, p.SegmentId, p.OptedOut, p.MarketingConsent, p.ServiceConsent, p.Reachability)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f.AIProfile = p.AiProfile
	// Validate before the first byte so errors can still be JSON.
	if _, _, err := f.where(); err != nil {
		h.fail(w, r, err)
		return
	}
	var out rowWriter
	var xw *xlsxWriter
	if p.Format != nil && *p.Format == api.ExportContactsParamsFormatXlsx {
		if xw, err = newXLSXWriter(h.svc.imp.tmpDir()); err != nil {
			h.fail(w, r, err)
			return
		}
		out = xw
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="messora-contacts.xlsx"`)
	} else {
		cw := csv.NewWriter(w)
		out = csvRowWriter{cw}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="messora-contacts.csv"`)
	}
	_ = out.Write(ExportColumns)
	var batch []db.Contact
	flush := func() error {
		ids := make([]uuid.UUID, len(batch))
		for i, c := range batch {
			ids[i] = c.ID
		}
		tags, err := h.svc.TagsFor(r.Context(), ids)
		if err != nil {
			return err
		}
		for _, c := range batch {
			names := make([]string, len(tags[c.ID]))
			for i, t := range tags[c.ID] {
				names[i] = t.Name
			}
			opted := "false"
			if c.OptedOutAt != nil {
				opted = "true"
			}
			if err := out.Write(safeCSV([]string{c.PhoneE164, c.Name, c.ProfileName, s(c.Email), s(c.Var1), s(c.Var2), s(c.Var3), s(c.Var4), s(c.Var5),
				strings.Join(names, ";"), opted, c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})); err != nil {
				return err
			}
		}
		batch = batch[:0]
		return out.Flush()
	}
	err = h.svc.Each(r.Context(), f, func(c db.Contact) error {
		batch = append(batch, c)
		if len(batch) == 500 {
			return flush()
		}
		return nil
	})
	if err == nil {
		err = flush()
	}
	if err == nil && xw != nil {
		// A workbook is a zip: it is assembled on disk and sent at the end.
		err = xw.Finish(w)
	}
	if err != nil {
		// Headers are gone; the truncated file is the signal.
		h.log.ErrorContext(r.Context(), "contact export aborted", "error", err)
		return
	}
	_ = audit.Record(r.Context(), h.svc.q, audit.Entry{Action: "contact.export", EntityType: "contact"})
}

type rowWriter interface {
	Write([]string) error
	Flush() error
}

type csvRowWriter struct{ w *csv.Writer }

func (c csvRowWriter) Write(r []string) error { return c.w.Write(r) }
func (c csvRowWriter) Flush() error           { c.w.Flush(); return c.w.Error() }

// safeCSV neutralises spreadsheet formulas (CSV injection).
func safeCSV(row []string) []string {
	for i, v := range row {
		if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) && (i != 0 || v[0] != '+') {
			row[i] = "'" + v
		}
	}
	return row
}

func s(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ── Tags ────────────────────────────────────────────────────────────────────

// ListContactAttributeKeys implements api.ServerInterface.
func (h *Handler) ListContactAttributeKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.q.ListContactAttributeKeys(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if keys == nil {
		keys = []string{}
	}
	httpx.JSON(w, r, http.StatusOK, keys)
}

// ListTags implements api.ServerInterface.
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListTags(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.Tag, len(rows))
	for i, t := range rows {
		out[i] = api.Tag{Id: t.ID, Name: t.Name, Color: t.Color, ContactCount: t.ContactCount}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateTag implements api.ServerInterface.
func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req api.TagInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		h.fail(w, r, apperr.Validation("name", "Name is required."))
		return
	}
	color := "gray"
	if req.Color != nil {
		color = *req.Color
	}
	t, err := h.svc.q.UpsertTag(r.Context(), db.UpsertTagParams{Name: strings.TrimSpace(*req.Name), Color: color})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, api.Tag{Id: t.ID, Name: t.Name, Color: t.Color})
}

// UpdateTag implements api.ServerInterface.
func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request, id api.TagId) {
	var req api.TagInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.q.UpdateTag(r.Context(), db.UpdateTagParams{ID: id, Name: req.Name, Color: req.Color})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		err = apperr.NotFound("Tag not found.")
	case isUnique(err):
		err = apperr.Conflict("TAG_EXISTS", "Another tag already has this name.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.Tag{Id: t.ID, Name: t.Name, Color: t.Color})
}

// DeleteTag implements api.ServerInterface.
func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request, id api.TagId) {
	n, err := h.svc.q.DeleteTag(r.Context(), id)
	if err == nil && n == 0 {
		err = apperr.NotFound("Tag not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ── Groups ──────────────────────────────────────────────────────────────────

func groupToAPI(g db.ContactGroup, members int64) api.Group {
	return api.Group{Id: g.ID, Name: g.Name, Description: g.Description, MemberCount: members, CreatedAt: g.CreatedAt}
}

// ListGroups implements api.ServerInterface.
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListContactGroups(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.Group, len(rows))
	for i, g := range rows {
		out[i] = groupToAPI(db.ContactGroup{ID: g.ID, Name: g.Name, Description: g.Description, CreatedAt: g.CreatedAt}, g.MemberCount)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateGroup implements api.ServerInterface.
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req api.GroupInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		h.fail(w, r, apperr.Validation("name", "Name is required."))
		return
	}
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	p := principal.From(r.Context())
	g, err := h.svc.q.InsertContactGroup(r.Context(), db.InsertContactGroupParams{Name: strings.TrimSpace(*req.Name), Description: desc, CreatedBy: &p.ID})
	if isUnique(err) {
		err = apperr.Conflict("GROUP_EXISTS", "A group with this name already exists.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, groupToAPI(g, 0))
}

// UpdateGroup implements api.ServerInterface.
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request, id api.GroupId) {
	var req api.GroupInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	g, err := h.svc.q.UpdateContactGroup(r.Context(), db.UpdateContactGroupParams{ID: id, Name: req.Name, Description: req.Description})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		err = apperr.NotFound("Group not found.")
	case isUnique(err):
		err = apperr.Conflict("GROUP_EXISTS", "A group with this name already exists.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, groupToAPI(g, 0))
}

// DeleteGroup implements api.ServerInterface.
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request, id api.GroupId) {
	n, err := h.svc.q.DeleteContactGroup(r.Context(), id)
	if err == nil && n == 0 {
		err = apperr.NotFound("Group not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ChangeGroupMembers implements api.ServerInterface.
func (h *Handler) ChangeGroupMembers(w http.ResponseWriter, r *http.Request, id api.GroupId) {
	var req api.GroupMembersRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if _, err := h.svc.q.GetContactGroup(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		h.fail(w, r, apperr.NotFound("Group not found."))
		return
	}
	var n int64
	var err error
	if req.Action == api.Add {
		n, err = h.svc.q.AddGroupMembers(r.Context(), db.AddGroupMembersParams{GroupID: id, ContactIds: req.ContactIds})
	} else {
		n, err = h.svc.q.RemoveGroupMembers(r.Context(), db.RemoveGroupMembersParams{GroupID: id, ContactIds: req.ContactIds})
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.Count{Count: n})
}

// ── Segments ────────────────────────────────────────────────────────────────

func parseDefinition(d api.SegmentDefinition) (*segment.Node, []byte, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, nil, apperr.Validation("definition", "Invalid definition.")
	}
	n, err := segment.Parse(raw)
	if err != nil {
		return nil, nil, apperr.Validation("definition", err.Error())
	}
	return n, raw, nil
}

func segmentToAPI(s db.Segment, count *int64) api.Segment {
	def := api.SegmentDefinition{}
	_ = json.Unmarshal(s.Definition, &def)
	return api.Segment{Id: s.ID, Name: s.Name, Description: s.Description, Definition: def, ContactCount: count,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}

// ListSegments implements api.ServerInterface.
func (h *Handler) ListSegments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListSegments(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.Segment, len(rows))
	for i, s := range rows {
		out[i] = segmentToAPI(s, nil)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateSegment implements api.ServerInterface.
func (h *Handler) CreateSegment(w http.ResponseWriter, r *http.Request) {
	var req api.SegmentInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	_, raw, err := parseDefinition(req.Definition)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	p := principal.From(r.Context())
	s, err := h.svc.q.InsertSegment(r.Context(), db.InsertSegmentParams{Name: strings.TrimSpace(req.Name), Description: desc, Definition: raw, CreatedBy: &p.ID})
	if isUnique(err) {
		err = apperr.Conflict("SEGMENT_EXISTS", "A segment with this name already exists.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, segmentToAPI(s, nil))
}

// GetSegment implements api.ServerInterface.
func (h *Handler) GetSegment(w http.ResponseWriter, r *http.Request, id api.SegmentId) {
	s, err := h.svc.q.GetSegment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Segment not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var count *int64
	if n, perr := segment.Parse(s.Definition); perr == nil {
		c, err := h.svc.Count(r.Context(), Filter{Segment: n})
		if err != nil {
			h.fail(w, r, err)
			return
		}
		count = &c
	}
	httpx.JSON(w, r, http.StatusOK, segmentToAPI(s, count))
}

// UpdateSegment implements api.ServerInterface.
func (h *Handler) UpdateSegment(w http.ResponseWriter, r *http.Request, id api.SegmentId) {
	var req api.UpdateSegmentRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p := db.UpdateSegmentParams{ID: id, Name: req.Name, Description: req.Description}
	if req.Definition != nil {
		_, raw, err := parseDefinition(*req.Definition)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		p.Definition = raw
	}
	s, err := h.svc.q.UpdateSegment(r.Context(), p)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		err = apperr.NotFound("Segment not found.")
	case isUnique(err):
		err = apperr.Conflict("SEGMENT_EXISTS", "A segment with this name already exists.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, segmentToAPI(s, nil))
}

// DeleteSegment implements api.ServerInterface.
func (h *Handler) DeleteSegment(w http.ResponseWriter, r *http.Request, id api.SegmentId) {
	n, err := h.svc.q.DeleteSegment(r.Context(), id)
	if err == nil && n == 0 {
		err = apperr.NotFound("Segment not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// PreviewSegment implements api.ServerInterface.
func (h *Handler) PreviewSegment(w http.ResponseWriter, r *http.Request) {
	var req api.PreviewSegmentJSONBody
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	n, _, err := parseDefinition(req.Definition)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f := Filter{Segment: n}
	count, err := h.svc.Count(r.Context(), f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	rows, err := h.svc.List(r.Context(), f, nil, 10)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sample, err := h.page(r, rows)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.SegmentPreview{Count: count, Sample: sample})
}

// BulkContacts implements api.ServerInterface.
func (h *Handler) BulkContacts(w http.ResponseWriter, r *http.Request) {
	var req api.ContactBulkRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p := principal.From(r.Context())
	need := "contacts.update"
	if req.Action == api.ContactBulkRequestActionDelete {
		need = "contacts.delete"
	}
	if !p.Has(need) {
		h.fail(w, r, apperr.Forbidden("FORBIDDEN", "This action needs the "+need+" permission."))
		return
	}
	br := BulkRequest{Action: BulkAction(req.Action), TagID: req.TagId, GroupID: req.GroupId}
	if p.Kind == principal.KindUser {
		br.Actor = &p.ID
	}
	if req.Ids != nil {
		br.IDs = *req.Ids
	}
	if req.ExcludeIds != nil {
		br.Exclude = *req.ExcludeIds
	}
	if req.TagName != nil {
		br.TagName = strings.TrimSpace(*req.TagName)
	}
	if req.Filter != nil {
		fl := req.Filter
		f, err := h.filter(r, fl.Q, fl.TagId, fl.GroupId, fl.SegmentId, fl.OptedOut, fl.MarketingConsent, fl.ServiceConsent, fl.Reachability)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		f.AIProfile = fl.AiProfile
		br.Filter = &f
	}
	res, err := h.svc.Bulk(r.Context(), br)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.ContactBulkResult{Matched: res.Matched, Affected: res.Affected})
}

func (h *Handler) insights(w http.ResponseWriter, r *http.Request, kind AudienceKind, id uuid.UUID, f Filter, tz *string) {
	zone := ""
	if tz != nil {
		zone = *tz
	}
	out, err := h.svc.Insights(r.Context(), kind, id, f, zone)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// GetGroupInsights implements api.ServerInterface.
func (h *Handler) GetGroupInsights(w http.ResponseWriter, r *http.Request, id api.GroupId, p api.GetGroupInsightsParams) {
	if _, err := h.svc.q.GetContactGroup(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = apperr.NotFound("Group not found.")
		}
		h.fail(w, r, err)
		return
	}
	h.insights(w, r, AudienceGroup, id, Filter{GroupID: &id}, p.Tz)
}

// GetTagInsights implements api.ServerInterface.
func (h *Handler) GetTagInsights(w http.ResponseWriter, r *http.Request, id api.TagId, p api.GetTagInsightsParams) {
	if _, err := h.svc.q.GetTag(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = apperr.NotFound("Tag not found.")
		}
		h.fail(w, r, err)
		return
	}
	h.insights(w, r, AudienceTag, id, Filter{TagID: &id}, p.Tz)
}

// GetSegmentInsights implements api.ServerInterface.
func (h *Handler) GetSegmentInsights(w http.ResponseWriter, r *http.Request, id api.SegmentId, p api.GetSegmentInsightsParams) {
	s, err := h.svc.q.GetSegment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Segment not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	n, err := segment.Parse(s.Definition)
	if err != nil {
		h.fail(w, r, apperr.Validation("definition", err.Error()))
		return
	}
	h.insights(w, r, AudienceSegment, id, Filter{Segment: n}, p.Tz)
}
