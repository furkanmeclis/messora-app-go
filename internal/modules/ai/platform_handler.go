package ai

import (
	"errors"
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The platform's AI keys: the admin panel's operations (/platform/ai-*,
// admin host, platform sessions) and the workspaces' read-only list.

func platformProviderToAPI(p db.PlatformAiProvider, models int64) api.PlatformAIProvider {
	return api.PlatformAIProvider{Id: p.ID, Kind: api.AIProviderKind(p.Kind), Name: p.Name, KeyLast4: p.KeyLast4, BaseUrl: p.BaseUrl,
		Status: api.PlatformAIProviderStatus(p.Status), StatusDetail: p.StatusDetail, CheckedAt: p.CheckedAt, ModelsSyncedAt: p.ModelsSyncedAt,
		Enabled: p.Enabled, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, ModelCount: models}
}

func (h *Handler) failPlatform(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrProviderExists) {
		err = apperr.Conflict("PROVIDER_EXISTS", "Messora already has a key for this provider; edit it instead.")
	}
	h.fail(w, r, err)
}

func (h *Handler) respondPlatformProvider(w http.ResponseWriter, r *http.Request, status int, p db.PlatformAiProvider) {
	counts, err := h.svc.PlatformModelCounts(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, platformProviderToAPI(p, counts[p.ID]))
}

// ListAIPlatformProviders implements api.ServerInterface.
func (h *Handler) ListAIPlatformProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListPlatformProviders(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AIPlatformProvider, 0, len(rows))
	for _, p := range rows {
		if Kind(p.Kind) == KindFake && !h.svc.FakeAllowed() {
			continue
		}
		out = append(out, api.AIPlatformProvider{Id: p.ID, Kind: api.AIProviderKind(p.Kind), Name: p.Name, Enabled: p.Enabled,
			Status: api.AIPlatformProviderStatus(p.Status)})
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ListPlatformAIProviders implements api.ServerInterface.
func (h *Handler) ListPlatformAIProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListPlatformProviders(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	counts, err := h.svc.PlatformModelCounts(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.PlatformAIProvider, len(rows))
	for i, p := range rows {
		out[i] = platformProviderToAPI(p, counts[p.ID])
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreatePlatformAIProvider implements api.ServerInterface.
func (h *Handler) CreatePlatformAIProvider(w http.ResponseWriter, r *http.Request) {
	var req api.CreateAIProviderRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreatePlatformProvider(r.Context(), ProviderInput{Kind: Kind(req.Kind), Name: &req.Name, APIKey: req.ApiKey,
		BaseURL: req.BaseUrl, Enabled: req.Enabled})
	if err != nil {
		h.failPlatform(w, r, err)
		return
	}
	h.respondPlatformProvider(w, r, http.StatusCreated, p)
}

// UpdatePlatformAIProvider implements api.ServerInterface.
func (h *Handler) UpdatePlatformAIProvider(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	var req api.UpdateAIProviderRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.UpdatePlatformProvider(r.Context(), id, ProviderInput{Name: req.Name, APIKey: req.ApiKey, BaseURL: req.BaseUrl, Enabled: req.Enabled})
	if err != nil {
		h.failPlatform(w, r, err)
		return
	}
	h.respondPlatformProvider(w, r, http.StatusOK, p)
}

// DeletePlatformAIProvider implements api.ServerInterface.
func (h *Handler) DeletePlatformAIProvider(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	if err := h.svc.DeletePlatformProvider(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// TestPlatformAIProvider implements api.ServerInterface.
func (h *Handler) TestPlatformAIProvider(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	p, err := h.svc.TestPlatformProvider(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.respondPlatformProvider(w, r, http.StatusOK, p)
}

// ModelToAPI renders a catalogue entry (the platform panel's model list).
func ModelToAPI(v ModelView) api.AIModel {
	m := v.Model
	out := api.AIModel{ProviderId: m.ProviderID, ProviderName: m.ProviderName, ProviderKind: api.AIProviderKind(m.ProviderKind),
		ModelId: m.ModelID, DisplayName: m.DisplayName, ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutput, Lifecycle: m.Lifecycle,
		SyncedAt: m.SyncedAt, KeySource: api.AIModelKeySource(v.KeySource), Capabilities: api.AICapabilities{Tools: v.Capabilities.Tools,
			StructuredOutput: v.Capabilities.StructuredOutput, Vision: v.Capabilities.Vision, Thinking: v.Capabilities.Thinking}}
	if v.Price != nil {
		p := priceToAPI(*v.Price)
		out.Price = &p
	}
	return out
}
