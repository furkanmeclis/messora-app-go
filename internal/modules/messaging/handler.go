package messaging

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the message operations.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) toAPI(r *http.Request, m db.Message) (api.Message, error) {
	c, err := h.svc.q.GetContact(r.Context(), m.ContactID)
	if err != nil {
		return api.Message{}, err
	}
	return api.Message{Id: m.ID, PhoneNumberId: m.PhoneNumberID, ContactId: m.ContactID, To: c.PhoneE164,
		Direction: api.MessageDirection(m.Direction), Wamid: m.Wamid, Type: m.Type, Body: m.Body,
		Status: api.MessageStatus(m.Status), ErrorCode: m.ErrorCode, ErrorTitle: m.ErrorTitle, SentAt: m.SentAt,
		DeliveredAt: m.DeliveredAt, ReadAt: m.ReadAt, FailedAt: m.FailedAt, CreatedAt: m.CreatedAt}, nil
}

// SendMessage implements api.ServerInterface.
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request, _ api.SendMessageParams) {
	var req api.SendMessageRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	p := principal.From(r.Context())
	in := SendInput{PhoneNumberID: req.PhoneNumberId, To: req.To, Type: string(req.Type), Source: "api"}
	if p.Kind == principal.KindUser {
		in.Source, in.CreatedBy = "panel", &p.ID
	} else {
		in.CreatedBy = &p.ID
	}
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
				httpx.WriteError(w, r, validation("template.components", "Components do not match Meta's format."), h.log)
				return
			}
		}
	}
	if m := req.Media; m != nil {
		in.Media = &whatsapp.Media{ID: deref(m.Id), Link: deref(m.Link), Caption: deref(m.Caption), Filename: deref(m.Filename)}
	}
	if l := req.Location; l != nil {
		in.Location = &whatsapp.Location{Latitude: l.Latitude, Longitude: l.Longitude, Name: deref(l.Name), Address: deref(l.Address)}
	}
	if x := req.Reaction; x != nil {
		in.Reaction = &whatsapp.Reaction{MessageID: x.MessageId, Emoji: x.Emoji}
	}
	if req.Interactive != nil {
		in.Interactive, _ = json.Marshal(*req.Interactive)
	}
	m, err := h.svc.Queue(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out, err := h.toAPI(r, m)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, out)
}

// GetMessage implements api.ServerInterface.
func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request, id api.MessageId) {
	m, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out, err := h.toAPI(r, m)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// UploadMedia implements api.ServerInterface; the body is the file.
func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request, p api.UploadMediaParams) {
	m, err := h.svc.Upload(r.Context(), p.PhoneNumberId, p.Filename, r.Header.Get("Content-Type"), r.ContentLength, r.Body)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, api.UploadedMedia{Id: m.ID, MimeType: m.MimeType, Filename: m.Filename, Size: m.Size, ExpiresAt: m.ExpiresAt})
}
