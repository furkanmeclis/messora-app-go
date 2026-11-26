package conversations

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

func userRef(id *uuid.UUID, name *string) *api.UserRef {
	if id == nil {
		return nil
	}
	return &api.UserRef{Id: *id, Name: deref(name)}
}

// SetConversationAIMode implements api.ServerInterface.
func (h *Handler) SetConversationAIMode(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	var req api.SetConversationAIModeRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.SetAIMode(r.Context(), principal.From(r.Context()), id, Mode(req.Mode)); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConversation(w, r, id)
}

// HandOffConversation implements api.ServerInterface.
func (h *Handler) HandOffConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	var req api.HandoffRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	var mode Mode
	if req.Mode != nil {
		mode = Mode(*req.Mode)
	}
	if err := h.svc.HandOff(r.Context(), principal.From(r.Context()), id, string(req.To), mode, req.AssigneeId, deref(req.Note)); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConversation(w, r, id)
}

// TakeOverConversation implements api.ServerInterface.
func (h *Handler) TakeOverConversation(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	if err := h.svc.TakeOver(r.Context(), principal.From(r.Context()), id); err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConversation(w, r, id)
}

// ListConversationHandoffs implements api.ServerInterface.
func (h *Handler) ListConversationHandoffs(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	rows, err := h.svc.Handoffs(r.Context(), principal.From(r.Context()), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ConversationHandoff, len(rows))
	for i, x := range rows {
		fields := map[string]string{}
		_ = json.Unmarshal(x.CollectedFields, &fields)
		out[i] = api.ConversationHandoff{Id: x.ID, FromType: api.ConversationHandoffFromType(x.FromType), FromUser: userRef(x.FromUserID, x.FromUserName),
			ToType: api.ConversationHandoffToType(x.ToType), ToUser: userRef(x.ToUserID, x.ToUserName), Reason: api.HandoffReason(x.Reason),
			ReasonDetail: x.ReasonDetail, Summary: x.Summary, SummarySource: api.ConversationHandoffSummarySource(x.SummarySource),
			DetectedNeed: x.DetectedNeed, SuggestedNextAction: x.SuggestedNextAction, CollectedFields: fields, CreatedAt: x.CreatedAt,
			AcceptedBy: userRef(x.AcceptedBy, x.AcceptedByName), AcceptedAt: x.AcceptedAt}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

func noteToAPI(id uuid.UUID, authorType string, author *uuid.UUID, authorName *string, body string, forAI bool, n db.ConversationNote) api.ConversationNote {
	return api.ConversationNote{Id: id, AuthorType: api.ConversationNoteAuthorType(authorType), Author: userRef(author, authorName),
		Body: body, ForAi: forAI, CreatedAt: n.CreatedAt}
}

// ListConversationNotes implements api.ServerInterface.
func (h *Handler) ListConversationNotes(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	rows, err := h.svc.Notes(r.Context(), principal.From(r.Context()), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ConversationNote, len(rows))
	for i, n := range rows {
		out[i] = noteToAPI(n.ID, n.AuthorType, n.AuthorID, n.AuthorName, n.Body, n.ForAi, db.ConversationNote{CreatedAt: n.CreatedAt})
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateConversationNote implements api.ServerInterface.
func (h *Handler) CreateConversationNote(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	var req api.CreateConversationNoteRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p := principal.From(r.Context())
	n, err := h.svc.AddNote(r.Context(), p, id, req.Body, req.ForAi != nil && *req.ForAi)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var name *string
	if p.Kind == principal.KindUser && p.Name != "" {
		name = &p.Name
	}
	httpx.JSON(w, r, http.StatusCreated, noteToAPI(n.ID, n.AuthorType, n.AuthorID, name, n.Body, n.ForAi, n))
}

// ListConversationActivity implements api.ServerInterface.
func (h *Handler) ListConversationActivity(w http.ResponseWriter, r *http.Request, id api.ConversationId, p api.ListConversationActivityParams) {
	cursor, err := parseCursor(p.Cursor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	rows, more, err := h.svc.Activity(r.Context(), principal.From(r.Context()), id, cursor, limitOf(p.Limit))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ConversationActivity, len(rows))
	for i, a := range rows {
		data := map[string]any{}
		_ = json.Unmarshal(a.Data, &data)
		out[i] = api.ConversationActivity{Id: a.ID, Kind: api.ConversationActivityKind(a.Kind), ActorType: api.ConversationActivityActorType(a.ActorType),
			ActorName: a.ActorName, Data: data, CreatedAt: a.CreatedAt}
	}
	next := ""
	if more && len(rows) > 0 {
		next = rows[0].ID.String()
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

func draftToAPI(d db.ConversationAiDraft) api.AIDraft {
	return api.AIDraft{Id: d.ID, Kind: api.AIDraftKind(d.Kind), Body: d.Body, Confidence: d.Confidence, Intent: d.Intent,
		Status: api.AIDraftStatus(d.Status), MessageId: d.MessageID, SentMessageId: d.SentMessageID, DecidedBy: d.DecidedBy,
		DecidedAt: d.DecidedAt, CreatedAt: d.CreatedAt}
}

// ListConversationAIDrafts implements api.ServerInterface.
func (h *Handler) ListConversationAIDrafts(w http.ResponseWriter, r *http.Request, id api.ConversationId, p api.ListConversationAIDraftsParams) {
	rows, err := h.svc.Drafts(r.Context(), principal.From(r.Context()), id, p.Pending != nil && *p.Pending)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AIDraft, len(rows))
	for i, d := range rows {
		out[i] = draftToAPI(d)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// AcceptConversationAIDraft implements api.ServerInterface.
func (h *Handler) AcceptConversationAIDraft(w http.ResponseWriter, r *http.Request, id api.ConversationId, draftID api.DraftId) {
	var req api.AcceptAIDraftRequest
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	d, err := h.svc.AcceptDraft(r.Context(), principal.From(r.Context()), id, draftID, req.Body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, draftToAPI(d))
}

// RejectConversationAIDraft implements api.ServerInterface.
func (h *Handler) RejectConversationAIDraft(w http.ResponseWriter, r *http.Request, id api.ConversationId, draftID api.DraftId) {
	d, err := h.svc.RejectDraft(r.Context(), principal.From(r.Context()), id, draftID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, draftToAPI(d))
}

// ListConversationAutomationRuns implements api.ServerInterface.
func (h *Handler) ListConversationAutomationRuns(w http.ResponseWriter, r *http.Request, id api.ConversationId) {
	rows, err := h.svc.AutomationRuns(r.Context(), principal.From(r.Context()), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ConversationAutomationRun, len(rows))
	for i, x := range rows {
		var last *string
		if x.LastAction != "" {
			last = &x.LastAction
		}
		out[i] = api.ConversationAutomationRun{Id: x.ID, AutomationId: x.AutomationID, AutomationName: x.AutomationName,
			Status: api.ConversationAutomationRunStatus(x.Status), StartedBy: api.ConversationAutomationRunStartedBy(x.StartedBy),
			StepsDone: x.StepsDone, LastAction: last, Error: x.Error, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, FinishedAt: x.FinishedAt}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// SetContactAI implements api.ServerInterface.
func (h *Handler) SetContactAI(w http.ResponseWriter, r *http.Request, contactID api.ContactId) {
	var req api.ContactAISettingRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.SetContactAI(r.Context(), principal.From(r.Context()), contactID, req.Disabled)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.ContactAISetting{ContactId: c.ID, AiDisabled: c.AiDisabled})
}
