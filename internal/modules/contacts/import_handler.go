package contacts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

func importToAPI(i db.ContactImport) api.ContactImport {
	headers, preview, mapping := []string{}, [][]string{}, map[string]string{}
	_ = json.Unmarshal(i.Headers, &headers)
	_ = json.Unmarshal(i.Preview, &preview)
	_ = json.Unmarshal(i.Mapping, &mapping)
	tags := i.AddTags
	if tags == nil {
		tags = []string{}
	}
	return api.ContactImport{Id: i.ID, Filename: i.Filename, Status: api.ContactImportStatus(i.Status), Headers: headers,
		Preview: preview, Mapping: mapping, DuplicatePolicy: api.ContactImportDuplicatePolicy(i.DuplicatePolicy), AddTags: tags,
		GroupId: i.GroupID, TotalRows: i.TotalRows, ProcessedRows: i.ProcessedRows, CreatedCount: i.CreatedCount,
		UpdatedCount: i.UpdatedCount, SkippedCount: i.SkippedCount, FailedCount: i.FailedCount,
		HasErrorReport: i.ErrorReportKey != nil, LastError: i.LastError, CreatedAt: i.CreatedAt, StartedAt: i.StartedAt,
		FinishedAt: i.FinishedAt}
}

// ListContactImports implements api.ServerInterface.
func (h *Handler) ListContactImports(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListContactImports(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.ContactImport, len(rows))
	for i, x := range rows {
		out[i] = importToAPI(x)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// UploadContactImport implements api.ServerInterface; the body is the file.
func (h *Handler) UploadContactImport(w http.ResponseWriter, r *http.Request, p api.UploadContactImportParams) {
	actor := principal.From(r.Context())
	imp, err := h.svc.Upload(r.Context(), p.Filename, r.Body, &actor.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, importToAPI(imp))
}

func (h *Handler) loadImport(w http.ResponseWriter, r *http.Request, id api.ImportId) (db.ContactImport, bool) {
	imp, err := h.svc.q.GetContactImport(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Import not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return imp, false
	}
	return imp, true
}

// GetContactImport implements api.ServerInterface.
func (h *Handler) GetContactImport(w http.ResponseWriter, r *http.Request, id api.ImportId) {
	if imp, ok := h.loadImport(w, r, id); ok {
		httpx.JSON(w, r, http.StatusOK, importToAPI(imp))
	}
}

// StartContactImport implements api.ServerInterface.
func (h *Handler) StartContactImport(w http.ResponseWriter, r *http.Request, id api.ImportId) {
	var req api.StartContactImportRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	o := ImportOptions{Mapping: req.Mapping, GroupID: req.GroupId}
	if req.DuplicatePolicy != nil {
		o.DuplicatePolicy = string(*req.DuplicatePolicy)
	}
	if req.AddTags != nil {
		o.AddTags = *req.AddTags
	}
	imp, err := h.svc.Start(r.Context(), id, o)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, importToAPI(imp))
}

// CancelContactImport implements api.ServerInterface.
func (h *Handler) CancelContactImport(w http.ResponseWriter, r *http.Request, id api.ImportId) {
	imp, err := h.svc.Cancel(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, importToAPI(imp))
}

// GetContactImportErrors implements api.ServerInterface.
func (h *Handler) GetContactImportErrors(w http.ResponseWriter, r *http.Request, id api.ImportId) {
	imp, ok := h.loadImport(w, r, id)
	if !ok {
		return
	}
	if imp.ErrorReportKey == nil {
		h.fail(w, r, apperr.NotFound("This import has no error report."))
		return
	}
	f, err := h.svc.imp.Open(r.Context(), *imp.ErrorReportKey)
	if err != nil {
		h.fail(w, r, apperr.NotFound("The error report is no longer available."))
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="import-errors.csv"`)
	_, _ = io.Copy(w, f)
}
