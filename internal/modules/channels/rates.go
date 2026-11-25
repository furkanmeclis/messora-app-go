package channels

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// maxRateCardBytes bounds a rate card upload (Meta's full card is ~10 KB).
const maxRateCardBytes = 2 << 20

// ImportRates stores a rate card CSV (see metacost.ParseCSV) and records
// who imported it.
func (s *Service) ImportRates(ctx context.Context, r io.Reader, actor *uuid.UUID) (rates, markets int, err error) {
	list, mk, err := metacost.ParseCSV(r)
	var ie *metacost.ImportError
	if errors.As(err, &ie) {
		return 0, 0, apperr.Validation("file", "The rate card was not imported: "+strings.Join(ie.Problems, "; "))
	}
	if err != nil {
		return 0, 0, err
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		if err := metacost.Import(ctx, q, list, mk, actor); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "meta_rates.import", EntityType: "meta_rate_card",
			Changes: map[string]any{"rates": len(list), "markets": len(mk)}})
	})
	return len(list), len(mk), err
}

// ListMetaRates implements api.ServerInterface.
func (h *Handler) ListMetaRates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListMetaRates(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.MetaRate, len(rows))
	for i, x := range rows {
		countries := strings.Fields(x.Countries)
		if countries == nil {
			countries = []string{}
		}
		out[i] = api.MetaRate{Id: x.ID, Currency: x.Currency, Market: x.Market, Countries: countries,
			Category: api.MetaRateCategory(x.Category), Price: trimPrice(x.Price), TierLower: x.TierLower, TierUpper: x.TierUpper,
			EffectiveFrom: openapi_types.Date{Time: x.EffectiveFrom.Time}, Source: x.Source, ImportedAt: x.ImportedAt}
		if x.EffectiveTo.Valid {
			out[i].EffectiveTo = &openapi_types.Date{Time: x.EffectiveTo.Time}
		}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ImportMetaRates implements api.ServerInterface: a multipart upload with
// the CSV in the "file" field.
func (h *Handler) ImportMetaRates(w http.ResponseWriter, r *http.Request) {
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" || params["boundary"] == "" {
		httpx.WriteError(w, r, apperr.Validation("file", "Upload the CSV as multipart/form-data in a field named file."), h.log)
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteError(w, r, apperr.Validation("file", "Upload the CSV as multipart/form-data in a field named file."), h.log)
		return
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			httpx.WriteError(w, r, apperr.Validation("file", "The upload could not be read."), h.log)
			return
		}
		if part.FormName() != "file" {
			continue
		}
		actor := principal.From(r.Context())
		rates, markets, err := h.svc.ImportRates(r.Context(), io.LimitReader(part, maxRateCardBytes), &actor.ID)
		if err != nil {
			httpx.WriteError(w, r, err, h.log)
			return
		}
		httpx.JSON(w, r, http.StatusOK, api.MetaRateImport{Rates: rates, Markets: markets})
		return
	}
	httpx.WriteError(w, r, apperr.Validation("file", "The upload has no file field."), h.log)
}

// trimPrice drops trailing zeros of a numeric(12,6) ("0.043600" → "0.0436").
func trimPrice(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
