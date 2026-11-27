package campaigns

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// convert maps a value onto an API type through JSON (the shapes are the
// same; the spec is the contract).
func convert[T any](v any) (T, error) {
	var out T
	raw, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out)
}

func (h *Handler) writeConverted(w http.ResponseWriter, r *http.Request, status int, v any, conv func(any) (any, error)) {
	out, err := conv(v)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, out)
}

func filterOf(f *api.CampaignRecipientFilter) (RecipientFilter, error) {
	if f == nil {
		return RecipientFilter{}, nil
	}
	return convert[RecipientFilter](f)
}

// ForecastCampaign implements api.ServerInterface.
func (h *Handler) ForecastCampaign(w http.ResponseWriter, r *http.Request) {
	var req api.CampaignForecastRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	raw, _ := json.Marshal(req.Audience)
	var a Audience
	if err := json.Unmarshal(raw, &a); err != nil {
		h.fail(w, r, apperr.Validation("audience", "Invalid audience."))
		return
	}
	in := ForecastInput{Audience: a, PhoneNumberID: req.PhoneNumberId, TemplateName: req.TemplateName, TemplateLanguage: req.TemplateLanguage,
		MaxMPS: req.MaxMps, StartAt: req.ScheduledAt}
	if req.Priority != nil {
		in.Priority = string(*req.Priority)
	}
	f, err := h.svc.Forecast(r.Context(), in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeConverted(w, r, http.StatusOK, f, func(v any) (any, error) { return convert[api.CampaignForecast](v) })
}

// GetCampaignForecast implements api.ServerInterface.
func (h *Handler) GetCampaignForecast(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	fva, err := h.svc.CompareForecast(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := api.CampaignForecastVsActual{Final: fva.Final, ActualCost: fva.ActualCost, Currency: fva.Currency, Metrics: []api.ForecastComparison{}}
	if fva.Forecast != nil {
		f, err := convert[api.CampaignForecast](fva.Forecast)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		out.Forecast, out.StoredAt = &f, &fva.StoredAt
	}
	for _, m := range fva.Metrics {
		out.Metrics = append(out.Metrics, api.ForecastComparison{Metric: m.Metric, Forecast: m.Forecast, Actual: m.Actual, Delta: m.Delta, DeltaPct: m.DeltaPct})
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// GetCampaignAnalytics implements api.ServerInterface.
func (h *Handler) GetCampaignAnalytics(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	a, err := h.svc.Analysis(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	metrics := make([]map[string]any, len(a.Metrics))
	for i, m := range a.Metrics {
		metrics[i] = map[string]any{"key": m.Key, "value": m.Value, "source": m.Source}
	}
	costs := make([]map[string]any, len(a.CostBreakdown))
	for i, c := range a.CostBreakdown {
		costs[i] = map[string]any{"category": c.Category, "type": c.Type, "messages": c.Messages, "unpriced": c.Unpriced, "cost": c.Cost, "currency": c.Currency}
	}
	days := make([]map[string]any, len(a.Reconciled))
	for i, d := range a.Reconciled {
		days[i] = map[string]any{"day": d.Day.Format(time.DateOnly), "category": d.Category, "volume": d.Volume, "cost": nonEmptyStr(d.Cost),
			"currency": nonEmptyStr(d.Currency)}
	}
	timeline := make([]map[string]any, len(a.Timeline))
	for i, e := range a.Timeline {
		timeline[i] = map[string]any{"kind": e.Kind, "occurred_at": e.OccurredAt, "payload": e.Payload}
	}
	hourly := make([]map[string]any, len(a.Hourly))
	for i, p := range a.Hourly {
		hourly[i] = map[string]any{"hour": p.Hour, "sent": p.Sent, "delivered": p.Delivered, "read": p.Read, "replied": p.Replied, "failed": p.Failed}
	}
	body := map[string]any{"counts": a.Counts, "metrics": metrics, "rates": ratesJSON(a.Rates), "currency": a.Currency,
		"cost_estimate": a.CostEstimate, "priced": a.Priced, "unpriced": a.Unpriced, "cost_breakdown": costs, "reconciled": days,
		"number_quality": a.NumberQuality, "template_status": a.TemplateStatus, "template_quality": a.TemplateQuality,
		"timeline": timeline, "classes": a.Classes, "hourly": hourly, "quick_replies": a.QuickReplies,
		"ai_cost": aiCostJSON(a.AICost), "total_cost": nonEmptyStr(a.TotalCost)}
	h.writeConverted(w, r, http.StatusOK, body, func(v any) (any, error) { return convert[api.CampaignAnalytics](v) })
}

func aiCostJSON(in []AICost) []map[string]any {
	out := make([]map[string]any, len(in))
	for i, x := range in {
		out[i] = map[string]any{"currency": x.Currency, "cost": x.Cost, "requests": x.Requests, "failed": x.Failed,
			"input_tokens": x.InputTokens, "output_tokens": x.OutputTokens, "cached_tokens": x.CachedTokens, "conversations": x.Conversations}
	}
	return out
}

func ratesJSON(r Rates) map[string]any {
	return map[string]any{"sent": r.Sent, "delivery": r.Delivery, "read": r.Read, "reply": r.Reply, "positive": r.Positive,
		"negative": r.Negative, "opt_out": r.OptOut}
}

func nonEmptyStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetCampaignBreakdown implements api.ServerInterface.
func (h *Handler) GetCampaignBreakdown(w http.ResponseWriter, r *http.Request, id api.CampaignId, p api.GetCampaignBreakdownParams) {
	key := ""
	if p.AttrKey != nil {
		key = *p.AttrKey
	}
	bs, err := h.svc.Breakdown(r.Context(), id, string(p.Dimension), key)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.CampaignBreakdownBucket, len(bs))
	for i, b := range bs {
		out[i] = api.CampaignBreakdownBucket{Key: b.Key, Label: b.Label, Recipients: b.Recipients, Sent: b.Sent, Delivered: b.Delivered,
			Read: b.Read, Replied: b.Replied, Positive: b.Positive, Negative: b.Negative, OptedOut: b.OptedOut, Cost: b.Cost}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

func recipientRowAPI(x RecipientRow) api.CampaignRecipientRow {
	out := api.CampaignRecipientRow{Id: x.ID, ContactId: x.ContactID, Phone: x.PhoneE164, Name: x.Name, ContactType: x.ContactType,
		State: x.State, SkipReason: x.SkipReason, MessageId: x.MessageID, MessageStatus: x.MsgStatus, Outcome: Outcome(x.State, x.MsgStatus),
		UnreachedClass: x.UnreachedClass, FailureClass: x.FailureClass, ErrorCode: x.ErrorCode, ErrorTitle: x.ErrorTitle,
		NextAttemptAt: x.NextAttemptAt, AttemptedAt: x.AttemptedAt,
		SentAt: x.SentAt, DeliveredAt: x.DeliveredAt, ReadAt: x.ReadAt, FailedAt: x.FailedAt, RepliedAt: x.RepliedAt,
		ReplyPayload: x.ReplyPayload, OptedOutAt: x.OptedOutAt, Cost: x.Cost, CostCurrency: x.CostCurrency, Resends: int(x.Resends)}
	if x.Reaction != nil {
		out.Reaction = ptr(api.CampaignRecipientRowReaction(*x.Reaction))
	}
	if x.ReactionSource != nil {
		out.ReactionSource = ptr(api.CampaignRecipientRowReactionSource(*x.ReactionSource))
	}
	return out
}

// SearchCampaignRecipients implements api.ServerInterface.
func (h *Handler) SearchCampaignRecipients(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req api.CampaignRecipientSearchRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := filterOf(req.Filter)
	if err != nil {
		h.fail(w, r, apperr.Validation("filter", "Invalid filter."))
		return
	}
	limit, offset, sort := 50, 0, ""
	if req.Limit != nil {
		limit = *req.Limit
	}
	if req.Offset != nil {
		offset = *req.Offset
	}
	if req.Sort != nil {
		sort = *req.Sort
	}
	page, err := h.svc.SearchRecipients(r.Context(), id, f, sort, offset, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.CampaignRecipientRow, len(page.Rows))
	for i, x := range page.Rows {
		out[i] = recipientRowAPI(x)
	}
	httpx.JSONTotal(w, r, http.StatusOK, out, page.Total)
}

// BulkCampaignRecipients implements api.ServerInterface.
func (h *Handler) BulkCampaignRecipients(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req api.CampaignRecipientBulkRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := filterOf(req.Filter)
	if err != nil {
		h.fail(w, r, apperr.Validation("filter", "Invalid filter."))
		return
	}
	res, err := h.svc.BulkRecipients(r.Context(), id, f, string(req.Action), req.TargetId, actor(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var out struct {
		Affected int64         `json:"affected"`
		Campaign *api.Campaign `json:"campaign,omitempty"`
	}
	out.Affected = res.Affected
	if res.Campaign != nil {
		c, err := h.toAPI(r, *res.Campaign)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		out.Campaign = &c
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ExportCampaignRecipients implements api.ServerInterface (streamed).
func (h *Handler) ExportCampaignRecipients(w http.ResponseWriter, r *http.Request, id api.CampaignId, p api.ExportCampaignRecipientsParams) {
	f := RecipientFilter{TagID: p.TagId, GroupID: p.GroupId, SegmentID: p.SegmentId, ContactType: p.ContactType}
	if p.Q != nil {
		f.Query = *p.Q
	}
	if p.Status != nil {
		for _, st := range strings.Split(*p.Status, ",") {
			if st = strings.TrimSpace(st); st != "" {
				f.Statuses = append(f.Statuses, st)
			}
		}
	}
	if p.AttrKey != nil {
		f.AttrKey = *p.AttrKey
	}
	if p.AttrValue != nil {
		f.AttrValue = *p.AttrValue
	}
	format := FormatCSV
	if p.Format != nil && *p.Format == api.ExportCampaignRecipientsParamsFormatXlsx {
		format = FormatXLSX
	}
	c, err := h.svc.q.GetCampaign(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		h.fail(w, r, apperr.NotFound("Campaign not found."))
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	started := false
	err = h.svc.StreamRecipients(r.Context(), id, f, format, w, func() {
		started = true
		w.Header().Set("Content-Type", ContentType(format))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, exportFilename(c.Name, ExportRecipients, format)))
	})
	if err != nil {
		if !started {
			h.fail(w, r, err)
			return
		}
		h.log.ErrorContext(r.Context(), "campaign recipient export aborted", "error", err)
	}
}

// SetCampaignRecipientReaction implements api.ServerInterface.
func (h *Handler) SetCampaignRecipientReaction(w http.ResponseWriter, r *http.Request, id api.CampaignId, recipientID openapi_types.UUID) {
	var req struct {
		Reaction *string `json:"reaction"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Reaction != nil && !ValidReaction(*req.Reaction) {
		h.fail(w, r, apperr.Validation("reaction", "Reaction is positive, negative, neutral or null."))
		return
	}
	changed, err := h.svc.SetReaction(r.Context(), id, recipientID, req.Reaction, ReactionSourceManual)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if !changed {
		h.fail(w, r, apperr.NotFound("Recipient not found."))
		return
	}
	_ = audit.Record(r.Context(), h.svc.q, audit.Entry{Action: "campaign.reaction_set", EntityType: "campaign", EntityID: &id,
		Changes: map[string]any{"recipient_id": recipientID, "reaction": req.Reaction}})
	httpx.NoContent(w)
}

// SetCampaignReactionMap implements api.ServerInterface.
func (h *Handler) SetCampaignReactionMap(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req struct {
		ReactionMap map[string]string `json:"reaction_map"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.SetReactionMap(r.Context(), id, req.ReactionMap)
	h.write(w, r, http.StatusOK, c, err)
}

func exportAPI(e db.CampaignExport) api.CampaignExport {
	return api.CampaignExport{Id: e.ID, CampaignId: e.CampaignID, Kind: api.CampaignExportKind(e.Kind), Format: api.CampaignExportFormat(e.Format),
		Status: api.CampaignExportStatus(e.Status), Filename: e.Filename, SizeBytes: e.SizeBytes, Rows: e.RowsWritten, Error: e.Error,
		CreatedAt: e.CreatedAt, FinishedAt: e.FinishedAt, ExpiresAt: e.ExpiresAt}
}

// ListCampaignExports implements api.ServerInterface.
func (h *Handler) ListCampaignExports(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	rows, err := h.svc.Exports(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.CampaignExport, len(rows))
	for i, e := range rows {
		out[i] = exportAPI(e)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateCampaignExport implements api.ServerInterface.
func (h *Handler) CreateCampaignExport(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req api.CampaignExportRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := filterOf(req.Filter)
	if err != nil {
		h.fail(w, r, apperr.Validation("filter", "Invalid filter."))
		return
	}
	e, err := h.svc.RequestExport(r.Context(), id, string(req.Kind), string(req.Format), f, actor(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusAccepted, exportAPI(e))
}

// DownloadCampaignExport implements api.ServerInterface.
func (h *Handler) DownloadCampaignExport(w http.ResponseWriter, r *http.Request, id api.CampaignId, exportID openapi_types.UUID) {
	key, e, err := h.svc.OpenExport(r.Context(), id, exportID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	name := "export." + e.Format
	if e.Filename != nil {
		name = *e.Filename
	}
	// Local files are served here; S3 objects redirect to a short-lived
	// presigned URL with the same type and file name.
	err = storage.Serve(w, r, h.svc.ExportObjects(), key, storage.ServeOptions{ContentType: ContentType(e.Format),
		ContentDisposition: storage.DispositionHeader("attachment", name), CacheControl: "private, no-store"})
	if errors.Is(err, storage.ErrNotFound) {
		h.fail(w, r, apperr.Conflict("EXPORT_NOT_READY", "The export file is gone."))
		return
	}
	if err != nil {
		h.fail(w, r, err)
	}
}

func protectionAPI(s AnalyticsSettings) api.CampaignProtectionSettings {
	return api.CampaignProtectionSettings{Enabled: s.Enabled, StopOn131048: s.StopOn131048, StopOnAccountRestriction: s.StopOnAccountRestriction,
		Window: s.Window, MinAttempts: s.MinAttempts, EcosystemRatePct: s.EcosystemRatePct, OptOutRatePct: s.OptOutRatePct,
		QualityStop: api.CampaignProtectionSettingsQualityStop(s.QualityStop), StopOnTemplatePaused: s.StopOnTemplatePaused,
		AttributionDays: s.AttributionDays}
}

// GetCampaignProtectionSettings implements api.ServerInterface.
func (h *Handler) GetCampaignProtectionSettings(w http.ResponseWriter, r *http.Request) {
	st, err := LoadAnalyticsSettings(r.Context(), h.svc.q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, protectionAPI(st))
}

// UpdateCampaignProtectionSettings implements api.ServerInterface.
func (h *Handler) UpdateCampaignProtectionSettings(w http.ResponseWriter, r *http.Request) {
	var req api.CampaignProtectionSettings
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	st := AnalyticsSettings{Enabled: req.Enabled, StopOn131048: req.StopOn131048, StopOnAccountRestriction: req.StopOnAccountRestriction,
		Window: req.Window, MinAttempts: req.MinAttempts, EcosystemRatePct: req.EcosystemRatePct, OptOutRatePct: req.OptOutRatePct,
		QualityStop: string(req.QualityStop), StopOnTemplatePaused: req.StopOnTemplatePaused, AttributionDays: req.AttributionDays}
	err := h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
		old, err := LoadAnalyticsSettings(r.Context(), q)
		if err != nil {
			return err
		}
		if err := SaveAnalyticsSettings(r.Context(), q, st, actor(r)); err != nil {
			return err
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: "settings.campaign_protection_update", EntityType: "settings",
			Changes: map[string]any{"before": old, "after": st}})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	invalidateAnalytics()
	httpx.JSON(w, r, http.StatusOK, protectionAPI(st))
}
