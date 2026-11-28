package billing

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the billing operations of workspaces and of the platform
// admin panel.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func respond[T any](h *Handler, w http.ResponseWriter, r *http.Request, status int, v T, err error) {
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, status, v)
}

func decode[T any](h *Handler, w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return v, false
	}
	return v, true
}

var errProformaNotFound = apperr.NotFound("Proforma not found.")

// ── Workspace ──────────────────────────────────────────────────────────────

// GetBilling implements api.ServerInterface.
func (h *Handler) GetBilling(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Overview(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// UpdateBillingProfile implements api.ServerInterface.
func (h *Handler) UpdateBillingProfile(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.BillingProfileInput](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.SetProfile(r.Context(), principal.From(r.Context()), req)
	respond(h, w, r, http.StatusOK, v, err)
}

// ChangeBillingPlan implements api.ServerInterface.
func (h *Handler) ChangeBillingPlan(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.BillingPlanChangeRequest](h, w, r)
	if !ok {
		return
	}
	res, err := h.svc.ChangePlan(r.Context(), principal.From(r.Context()), req.PlanKey, string(req.BillingCycle))
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := api.BillingPlanChangeResult{Effect: api.BillingPlanChangeResultEffect(res.Effect)}
	if res.Proforma != nil {
		v := ProformaToAPI(*res.Proforma)
		out.Proforma = &v
	}
	out.Subscription, err = h.svc.Subscription(r.Context())
	respond(h, w, r, http.StatusOK, out, err)
}

// CancelBillingPlanChange implements api.ServerInterface.
func (h *Handler) CancelBillingPlanChange(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.CancelChange(r.Context()); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	v, err := h.svc.Subscription(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// ListBillingProformas implements api.ServerInterface.
func (h *Handler) ListBillingProformas(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Proformas(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// CreateBillingProforma implements api.ServerInterface.
func (h *Handler) CreateBillingProforma(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.CreateBillingProformaRequest](h, w, r)
	if !ok {
		return
	}
	f, err := h.svc.Purchase(r.Context(), principal.From(r.Context()), req)
	respond(h, w, r, http.StatusCreated, ProformaToAPI(f), err)
}

// GetBillingProforma implements api.ServerInterface.
func (h *Handler) GetBillingProforma(w http.ResponseWriter, r *http.Request, id api.ProformaId) {
	f, err := h.svc.Proforma(r.Context(), id)
	if err == nil && f == nil {
		err = errProformaNotFound
	}
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, ProformaToAPI(*f))
}

// CancelBillingProforma implements api.ServerInterface.
func (h *Handler) CancelBillingProforma(w http.ResponseWriter, r *http.Request, id api.ProformaId) {
	f, err := h.svc.Cancel(r.Context(), id)
	respond(h, w, r, http.StatusOK, ProformaToAPI(f), err)
}

func (h *Handler) servePDF(w http.ResponseWriter, r *http.Request, f *db.Proforma, err error) {
	if err == nil && f == nil {
		err = errProformaNotFound
	}
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	var buf bytes.Buffer
	if err := WriteProformaPDF(&buf, *f); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", storage.DispositionHeader("attachment", "proforma-"+f.Number+".pdf"))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(buf.Bytes())
}

// DownloadBillingProformaPdf implements api.ServerInterface.
func (h *Handler) DownloadBillingProformaPdf(w http.ResponseWriter, r *http.Request, id api.ProformaId) {
	f, err := h.svc.Proforma(r.Context(), id)
	h.servePDF(w, r, f, err)
}

// ListBillingInvoices implements api.ServerInterface.
func (h *Handler) ListBillingInvoices(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Invoices(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

func (h *Handler) serveInvoice(w http.ResponseWriter, r *http.Request, ctx context.Context, id uuid.UUID, platform bool) {
	key, owner, err := h.svc.InvoicePDF(ctx, id, platform)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	if h.svc.Store() == nil {
		httpx.WriteError(w, r, apperr.Unavailable("STORAGE_UNAVAILABLE", "File storage is not available."), h.log)
		return
	}
	r = r.WithContext(tenant.WithID(r.Context(), owner))
	err = storage.Serve(w, r, h.svc.Store(), key, storage.ServeOptions{ContentType: "application/pdf",
		ContentDisposition: storage.DispositionHeader("attachment", "invoice-"+id.String()+".pdf"), CacheControl: "private, no-store"})
	if errors.Is(err, storage.ErrNotFound) {
		httpx.WriteError(w, r, apperr.Conflict("INVOICE_PDF_PENDING", "The invoice PDF is gone; ask Messora to attach it again."), h.log)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
	}
}

// DownloadBillingInvoicePdf implements api.ServerInterface.
func (h *Handler) DownloadBillingInvoicePdf(w http.ResponseWriter, r *http.Request, id api.InvoiceId) {
	h.serveInvoice(w, r, r.Context(), id, false)
}

// GetMessageBalance implements api.ServerInterface.
func (h *Handler) GetMessageBalance(w http.ResponseWriter, r *http.Request, p api.GetMessageBalanceParams) {
	limit := int32(100)
	if p.Limit != nil {
		limit = int32(*p.Limit) //nolint:gosec // bounded by the spec
	}
	v, err := h.svc.MessageBalanceDetail(r.Context(), limit)
	respond(h, w, r, http.StatusOK, v, err)
}

// ── Platform ───────────────────────────────────────────────────────────────

// GetPlatformBillingOverview implements api.ServerInterface.
func (h *Handler) GetPlatformBillingOverview(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.PlatformOverview(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// ListPlatformSubscriptions implements api.ServerInterface.
func (h *Handler) ListPlatformSubscriptions(w http.ResponseWriter, r *http.Request, p api.ListPlatformSubscriptionsParams) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	v, err := h.svc.Subscriptions(r.Context(), status)
	respond(h, w, r, http.StatusOK, v, err)
}

// GetPlatformBillingSettings implements api.ServerInterface.
func (h *Handler) GetPlatformBillingSettings(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.PlatformSettings(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// UpdatePlatformBillingSettings implements api.ServerInterface.
func (h *Handler) UpdatePlatformBillingSettings(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.UpdatePlatformBillingSettingsRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.UpdatePlatformSettings(r.Context(), req)
	respond(h, w, r, http.StatusOK, v, err)
}

// ListPlatformProformas implements api.ServerInterface.
func (h *Handler) ListPlatformProformas(w http.ResponseWriter, r *http.Request, p api.ListPlatformProformasParams) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	v, err := h.svc.PlatformProformas(r.Context(), status, p.TenantId)
	respond(h, w, r, http.StatusOK, v, err)
}

// VoidPlatformProforma implements api.ServerInterface.
func (h *Handler) VoidPlatformProforma(w http.ResponseWriter, r *http.Request, id api.ProformaId) {
	req, ok := decode[api.BillingReasonRequest](h, w, r)
	if !ok {
		return
	}
	f, err := h.svc.Void(r.Context(), id, req.Reason)
	respond(h, w, r, http.StatusOK, h.svc.ProformaWithTenant(r.Context(), f), err)
}

// DownloadPlatformProformaPdf implements api.ServerInterface.
func (h *Handler) DownloadPlatformProformaPdf(w http.ResponseWriter, r *http.Request, id api.ProformaId) {
	f, err := h.svc.PlatformProforma(r.Context(), id)
	h.servePDF(w, r, f, err)
}

// transferView renders a transfer with its suggestion and, when the amount
// alone is ambiguous, the candidates.
func (h *Handler) transferView(ctx context.Context, t db.BankTransfer, slug, name *string) api.BankTransfer {
	out := api.BankTransfer{Id: t.ID, Amount: FormatKurus(t.AmountKurus), ReceivedOn: *datePtr(t.ReceivedOn), SenderName: t.SenderName,
		SenderIban: t.SenderIban, Description: t.Description, Reference: t.Reference, Source: api.BankTransferSource(t.Source),
		Status: api.BankTransferStatus(t.Status), MatchMethod: t.MatchMethod, ProformaId: t.ProformaID, Note: t.Note, CreatedAt: t.CreatedAt,
		MatchedAt: t.MatchedAt, Candidates: []api.Proforma{}}
	if t.Suggestion != nil {
		v := api.BankTransferSuggestion(*t.Suggestion)
		out.Suggestion = &v
	}
	if t.TenantID != nil {
		out.Tenant = &api.TenantRef{Id: *t.TenantID, Slug: deref(slug, ""), Name: deref(name, "")}
	}
	if t.Status != "unmatched" {
		return out
	}
	if t.SuggestedProformaID != nil {
		if f, err := h.svc.PlatformProforma(ctx, *t.SuggestedProformaID); err == nil && f != nil {
			v := h.svc.ProformaWithTenant(ctx, *f)
			out.SuggestedProforma = &v
		}
	} else if t.Reference == nil {
		if rows, err := h.svc.q.OpenProformasByAmount(tenant.Platform(ctx), t.AmountKurus); err == nil && len(rows) > 1 {
			for _, f := range rows {
				out.Candidates = append(out.Candidates, h.svc.ProformaWithTenant(ctx, f))
			}
		}
	}
	return out
}

// ListPlatformTransfers implements api.ServerInterface.
func (h *Handler) ListPlatformTransfers(w http.ResponseWriter, r *http.Request, p api.ListPlatformTransfersParams) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	rows, err := h.svc.Transfers(r.Context(), status)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.BankTransfer, len(rows))
	for i, t := range rows {
		out[i] = h.transferView(r.Context(), db.BankTransfer{ID: t.ID, AmountKurus: t.AmountKurus, Currency: t.Currency, ReceivedOn: t.ReceivedOn,
			SenderName: t.SenderName, SenderIban: t.SenderIban, Description: t.Description, Reference: t.Reference, Source: t.Source,
			Fingerprint: t.Fingerprint, Status: t.Status, SuggestedProformaID: t.SuggestedProformaID, Suggestion: t.Suggestion,
			MatchMethod: t.MatchMethod, TenantID: t.TenantID, ProformaID: t.ProformaID, Note: t.Note, CreatedBy: t.CreatedBy,
			MatchedBy: t.MatchedBy, MatchedAt: t.MatchedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}, t.TenantSlug, t.TenantName)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) transferResponse(w http.ResponseWriter, r *http.Request, status int, t db.BankTransfer, err error) {
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	var slug, name *string
	if t.TenantID != nil {
		if tt, err := h.svc.q.GetTenant(tenant.System(r.Context()), *t.TenantID); err == nil {
			slug, name = &tt.Slug, &tt.Name
		}
	}
	httpx.JSON(w, r, status, h.transferView(r.Context(), t, slug, name))
}

// CreatePlatformTransfer implements api.ServerInterface.
func (h *Handler) CreatePlatformTransfer(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.CreateBankTransferRequest](h, w, r)
	if !ok {
		return
	}
	amount, err := ParseKurus(req.Amount)
	if err != nil {
		httpx.WriteError(w, r, apperr.Validation("amount", err.Error()), h.log)
		return
	}
	t, err := h.svc.RecordTransfer(r.Context(), principal.From(r.Context()), TransferInput{AmountKurus: amount, ReceivedOn: req.ReceivedOn.Time,
		SenderName: deref(req.SenderName, ""), SenderIBAN: deref(req.SenderIban, ""), Description: deref(req.Description, ""),
		Reference: deref(req.Reference, ""), Note: deref(req.Note, "")})
	h.transferResponse(w, r, http.StatusCreated, t, err)
}

// ImportPlatformTransfers implements api.ServerInterface.
func (h *Handler) ImportPlatformTransfers(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.ImportCSV(r.Context(), principal.From(r.Context()), r.Body)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := api.BankTransferImport{Created: len(res.Created), Duplicates: res.Duplicates, Errors: []api.BankTransferImportError{},
		Transfers: make([]api.BankTransfer, len(res.Created))}
	for _, e := range res.Errors {
		out.Errors = append(out.Errors, api.BankTransferImportError{Line: e.Line, Message: e.Message})
	}
	for i, t := range res.Created {
		out.Transfers[i] = h.transferView(r.Context(), t, nil, nil)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// MatchPlatformTransfer implements api.ServerInterface.
func (h *Handler) MatchPlatformTransfer(w http.ResponseWriter, r *http.Request, id api.TransferId) {
	req, ok := decode[api.MatchBankTransferRequest](h, w, r)
	if !ok {
		return
	}
	t, err := h.svc.Match(r.Context(), principal.From(r.Context()), id, req.ProformaId, req.TenantId, deref(req.Note, ""))
	h.transferResponse(w, r, http.StatusOK, t, err)
}

// IgnorePlatformTransfer implements api.ServerInterface.
func (h *Handler) IgnorePlatformTransfer(w http.ResponseWriter, r *http.Request, id api.TransferId) {
	req, ok := decode[api.BillingReasonRequest](h, w, r)
	if !ok {
		return
	}
	t, err := h.svc.Ignore(r.Context(), principal.From(r.Context()), id, req.Reason)
	h.transferResponse(w, r, http.StatusOK, t, err)
}

// ListPlatformInvoices implements api.ServerInterface.
func (h *Handler) ListPlatformInvoices(w http.ResponseWriter, r *http.Request, p api.ListPlatformInvoicesParams) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	v, err := h.svc.PlatformInvoices(r.Context(), status, p.TenantId)
	respond(h, w, r, http.StatusOK, v, err)
}

// RecordPlatformInvoice implements api.ServerInterface.
func (h *Handler) RecordPlatformInvoice(w http.ResponseWriter, r *http.Request, id api.InvoiceId) {
	req, ok := decode[api.RecordInvoiceRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.RecordInvoice(r.Context(), id, req.OfficialNumber, req.IssuedOn)
	respond(h, w, r, http.StatusOK, v, err)
}

// UploadPlatformInvoicePdf implements api.ServerInterface.
func (h *Handler) UploadPlatformInvoicePdf(w http.ResponseWriter, r *http.Request, id api.InvoiceId) {
	v, err := h.svc.UploadInvoicePDF(r.Context(), id, r.Body)
	respond(h, w, r, http.StatusOK, v, err)
}

// DownloadPlatformInvoicePdf implements api.ServerInterface.
func (h *Handler) DownloadPlatformInvoicePdf(w http.ResponseWriter, r *http.Request, id api.InvoiceId) {
	h.serveInvoice(w, r, r.Context(), id, true)
}

// GetPlatformTenantBilling implements api.ServerInterface.
func (h *Handler) GetPlatformTenantBilling(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	v, err := h.svc.TenantBilling(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// ApplyPlatformBillingAction implements api.ServerInterface.
func (h *Handler) ApplyPlatformBillingAction(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.PlatformBillingActionRequest](h, w, r)
	if !ok {
		return
	}
	if err := h.svc.Action(r.Context(), principal.From(r.Context()), id, req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	v, err := h.svc.TenantBilling(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}
