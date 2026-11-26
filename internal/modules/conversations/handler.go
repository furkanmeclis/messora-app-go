package conversations

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the inbox operations.
type Handler struct {
	svc *Service
	log *slog.Logger
	now func() time.Time
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, now: time.Now}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

func conversationToAPI(c db.ListConversationsRow, now time.Time) api.Conversation {
	open, expires := windowExpiry(c.LastInboundAt, now)
	out := api.Conversation{Id: c.ID, Status: api.ConversationStatus(c.Status), UnreadCount: c.UnreadCount,
		Contact: api.ConversationContact{Id: c.ContactID, Phone: c.PhoneE164, Name: c.ContactName, ProfileName: c.ProfileName,
			OptedOut: c.OptedOutAt != nil, AiDisabled: c.ContactAiDisabled},
		PhoneNumber:   api.ConversationPhoneNumber{Id: c.PhoneNumberID, AccountId: c.PhoneAccountID, DisplayPhoneNumber: c.DisplayPhoneNumber, VerifiedName: c.VerifiedName},
		LastMessageAt: c.LastMessageAt, LastInboundAt: c.LastInboundAt, WindowOpen: open, WindowExpiresAt: expires,
		ClosedAt: c.ClosedAt, CreatedAt: c.CreatedAt, OwnerType: api.ConversationOwnerType(c.OwnerType), AiMode: api.ConversationAIMode(c.AiMode),
		AiStatus: api.ConversationAiStatus(c.AiStatus), AiStatusDetail: c.AiStatusDetail, AiProfileId: c.AiProfileID, PendingHandoffId: c.PendingHandoffID}
	if c.AssigneeID != nil {
		out.Assignee = &api.UserRef{Id: *c.AssigneeID, Name: deref(c.AssigneeName)}
	}
	if c.LastDirection != nil {
		out.LastMessage = &api.ConversationLastMessage{Direction: api.ConversationLastMessageDirection(*c.LastDirection),
			Type: deref(c.LastType), Body: preview(deref(c.LastBody)), Status: deref(c.LastStatus)}
	}
	return out
}

// preview shortens a message body for the list.
func preview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 140 {
		return string(r[:139]) + "…"
	}
	return s
}

func messageToAPI(m db.Message, f *db.MediaFile) api.InboxMessage {
	out := api.InboxMessage{Id: m.ID, Direction: api.InboxMessageDirection(m.Direction), Type: m.Type, Body: m.Body,
		Status: api.InboxMessageStatus(m.Status), Wamid: m.Wamid, ReplyToWamid: m.ReplyToWamid, Source: api.InboxMessageSource(m.Source),
		ErrorCode: m.ErrorCode, ErrorTitle: m.ErrorTitle, OccurredAt: m.OccurredAt, SentAt: m.SentAt, DeliveredAt: m.DeliveredAt,
		ReadAt: m.ReadAt, FailedAt: m.FailedAt, CreatedAt: m.CreatedAt, FailureClass: (*api.FailureClass)(m.FailureClass),
		SentBy: (*api.InboxMessageSentBy)(m.SentBy), AiUsageId: m.AiUsageID}
	if f != nil {
		out.Media = &api.InboxMedia{Id: f.ID, MimeType: f.MimeType, Filename: f.Filename, SizeBytes: f.SizeBytes,
			Status: api.InboxMediaStatus(f.Status)}
	}
	return out
}

func parseCursor(c *string) (*uuid.UUID, error) {
	if c == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*c)
	if err != nil {
		return nil, apperr.Validation("cursor", "Invalid cursor")
	}
	return &id, nil
}

func limitOf(l *int) int {
	if l == nil {
		return 50
	}
	return *l
}

// ListConversations implements api.ServerInterface.
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request, p api.ListConversationsParams) {
	f := Filter{PhoneNumberID: p.PhoneNumberId, UnreadOnly: p.Unread != nil && *p.Unread}
	if p.Status != nil {
		st := string(*p.Status)
		f.Status = &st
	}
	if p.Assignee != nil {
		f.Assignee = *p.Assignee
	}
	if p.Q != nil {
		f.Query = *p.Q
	}
	if p.Ai != nil {
		f.AI = string(*p.Ai)
	}
	cursor, err := parseCursor(p.Cursor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	limit := limitOf(p.Limit)
	rows, err := h.svc.List(r.Context(), principal.From(r.Context()), f, cursor, limit+1)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	now := h.now()
	out := make([]api.Conversation, len(rows))
	for i, c := range rows {
		out[i] = conversationToAPI(c, now)
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

func (h *Handler) writeConversation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	c, err := h.svc.Get(r.Context(), principal.From(r.Context()), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, conversationToAPI(c, h.now()))
}

// GetConversation implements api.ServerInterface.
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	h.writeConversation(w, r, id)
}

// ListConversationMessages implements api.ServerInterface.
func (h *Handler) ListConversationMessages(w http.ResponseWriter, r *http.Request, id api.ConversationId, p api.ListConversationMessagesParams) {
	cursor, err := parseCursor(p.Cursor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	limit := limitOf(p.Limit)
	rows, media, err := h.svc.Messages(r.Context(), principal.From(r.Context()), id, cursor, limit+1)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	out := make([]api.InboxMessage, len(rows))
	for i, m := range rows {
		var f *db.MediaFile
		if x, ok := media[m.ID]; ok {
			f = &x
		}
		out[i] = messageToAPI(m, f)
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// ReplyToConversation implements api.ServerInterface.
func (h *Handler) ReplyToConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId, _ api.ReplyToConversationParams) {
	var req api.ReplyRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in, params, err := replyInput(req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Reply(r.Context(), principal.From(r.Context()), id, in, params)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, messageToAPI(m, nil))
}

// replyInput maps the request onto messaging's input; validation of the
// content itself is messaging's.
func replyInput(req api.ReplyRequest) (messaging.SendInput, *campaigns.Params, error) {
	in := messaging.SendInput{Type: string(req.Type)}
	var params *campaigns.Params
	if req.ReplyTo != nil {
		in.ReplyTo = *req.ReplyTo
	}
	if t := req.Text; t != nil {
		in.Text = &whatsapp.Text{Body: t.Body, PreviewURL: t.PreviewUrl != nil && *t.PreviewUrl}
	}
	if t := req.Template; t != nil {
		in.Template = &whatsapp.Template{Name: t.Name, Language: whatsapp.TemplateLanguage{Code: t.Language}}
		if t.Components != nil {
			raw, _ := json.Marshal(*t.Components)
			if err := json.Unmarshal(raw, &in.Template.Components); err != nil {
				return in, nil, apperr.Validation("template.components", "Components do not match Meta's format.")
			}
		}
		if t.Params != nil {
			raw, _ := json.Marshal(*t.Params)
			params = &campaigns.Params{}
			if err := json.Unmarshal(raw, params); err != nil {
				return in, nil, apperr.Validation("template.params", "The template parameters are malformed.")
			}
		}
	}
	if m := req.Media; m != nil {
		in.Media = &whatsapp.Media{ID: deref(m.Id), Link: deref(m.Link), Caption: deref(m.Caption), Filename: deref(m.Filename)}
	}
	return in, params, nil
}

// AssignConversation implements api.ServerInterface.
func (h *Handler) AssignConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	var req api.AssignConversationRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Assign(r.Context(), principal.From(r.Context()), id, req.AssigneeId); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConversation(w, r, id)
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request, id uuid.UUID, status string) {
	if err := h.svc.SetStatus(r.Context(), principal.From(r.Context()), id, status); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConversation(w, r, id)
}

// CloseConversation implements api.ServerInterface.
func (h *Handler) CloseConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	h.setStatus(w, r, id, "closed")
}

// ReopenConversation implements api.ServerInterface.
func (h *Handler) ReopenConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	h.setStatus(w, r, id, "open")
}

// MarkConversationRead implements api.ServerInterface.
func (h *Handler) MarkConversationRead(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	if err := h.svc.MarkRead(r.Context(), principal.From(r.Context()), id); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConversation(w, r, id)
}

// ListConversationEvents implements api.ServerInterface.
func (h *Handler) ListConversationEvents(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	rows, err := h.svc.Events(r.Context(), principal.From(r.Context()), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ConversationEvent, len(rows))
	for i, e := range rows {
		out[i] = api.ConversationEvent{Id: e.ID, Kind: api.ConversationEventKind(e.Kind), ActorType: api.ConversationEventActorType(e.ActorType),
			ActorName: e.ActorName, CreatedAt: e.CreatedAt}
		if e.AssigneeID != nil {
			out[i].Assignee = &api.UserRef{Id: *e.AssigneeID, Name: deref(e.AssigneeName)}
		}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// DownloadConversationMedia implements api.ServerInterface.
func (h *Handler) DownloadConversationMedia(w http.ResponseWriter, r *http.Request, id api.ConversationId, mediaID api.MediaId) {
	f, file, err := h.svc.OpenMedia(r.Context(), principal.From(r.Context()), id, mediaID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	defer func() { _ = file.Close() }()
	st, err := file.Stat()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ctype, disp := mediaHeaders(f.MimeType, f.Filename, f.ID)
	hd := w.Header()
	hd.Set("Content-Type", ctype)
	hd.Set("Content-Disposition", disp)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Cache-Control", "private, max-age=300")
	// ServeContent adds Content-Length and range support (audio and video
	// seeking); the content type is already fixed, so it does not sniff.
	http.ServeContent(w, r, "", st.ModTime(), file)
}

// inlineTypes are rendered by browsers without running code; everything
// else (SVG, HTML, PDF, office files …) is downloaded.
var inlineTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true,
	"video/mp4": true, "video/3gpp": true,
	"audio/ogg": true, "audio/mpeg": true, "audio/aac": true, "audio/mp4": true, "audio/amr": true, "audio/opus": true,
}

var extensions = map[string]string{
	"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif", "video/mp4": ".mp4",
	"video/3gpp": ".3gp", "audio/ogg": ".ogg", "audio/mpeg": ".mp3", "audio/aac": ".aac", "audio/mp4": ".m4a",
	"audio/amr": ".amr", "application/pdf": ".pdf", "text/plain": ".txt",
}

// mediaHeaders returns a safe Content-Type and Content-Disposition for a
// stored file. Meta's MIME type is only trusted after parsing.
func mediaHeaders(mimeType, filename string, id uuid.UUID) (contentType, disposition string) {
	mt, params, err := mime.ParseMediaType(mimeType)
	contentType = "application/octet-stream"
	if err == nil && strings.Count(mt, "/") == 1 {
		// Only well-formed parameters such as codecs survive.
		keep := map[string]string{}
		for k, v := range params {
			if k == "codecs" || k == "charset" {
				keep[k] = v
			}
		}
		if ct := mime.FormatMediaType(mt, keep); ct != "" {
			contentType = ct
		}
	} else {
		mt = ""
	}
	kind := "attachment"
	if inlineTypes[mt] {
		kind = "inline"
	}
	name := cleanFilename(filename)
	if name == "" {
		name = "media-" + id.String() + extensions[mt]
	}
	disposition = mime.FormatMediaType(kind, map[string]string{"filename": name})
	if disposition == "" {
		disposition = kind
	}
	return contentType, disposition
}

// cleanFilename keeps the base name without control characters, quotes or
// separators, at most 200 characters.
func cleanFilename(s string) string {
	s = path.Base(strings.ReplaceAll(s, `\`, "/"))
	if s == "." || s == "/" || s == ".." {
		return ""
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200])
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
