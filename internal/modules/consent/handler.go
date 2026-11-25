package consent

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the consent settings.
type Handler struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(pool *pgxpool.Pool, log *slog.Logger) *Handler { return &Handler{pool: pool, log: log} }

// SettingsToAPI converts the settings.
func SettingsToAPI(s Settings) api.ConsentSettings {
	nn := func(l []string) []string {
		if l == nil {
			return []string{}
		}
		return l
	}
	return api.ConsentSettings{
		UnknownMarketing: api.ConsentSettingsUnknownMarketing(s.UnknownMarketing),
		Keywords:         api.ConsentKeywords{Stop: nn(s.Keywords.Stop), Request: nn(s.Keywords.Request), Resume: nn(s.Keywords.Resume)},
		Confirm:          s.Confirm,
		StopMessage:      api.LocalizedText{Tr: s.StopMessage.TR, En: s.StopMessage.EN},
		ResumeMessage:    api.LocalizedText{Tr: s.ResumeMessage.TR, En: s.ResumeMessage.EN},
	}
}

func settingsFromAPI(in api.ConsentSettings) Settings {
	return Settings{
		UnknownMarketing: string(in.UnknownMarketing),
		Keywords:         Keywords{Stop: in.Keywords.Stop, Request: in.Keywords.Request, Resume: in.Keywords.Resume},
		Confirm:          in.Confirm,
		StopMessage:      Messages{TR: in.StopMessage.Tr, EN: in.StopMessage.En},
		ResumeMessage:    Messages{TR: in.ResumeMessage.Tr, EN: in.ResumeMessage.En},
	}
}

// GetConsentSettings implements api.ServerInterface.
func (h *Handler) GetConsentSettings(w http.ResponseWriter, r *http.Request) {
	s, err := LoadSettings(r.Context(), db.New(h.pool))
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, SettingsToAPI(s))
}

// UpdateConsentSettings implements api.ServerInterface.
func (h *Handler) UpdateConsentSettings(w http.ResponseWriter, r *http.Request) {
	var req api.ConsentSettings
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	actor := principal.From(r.Context())
	var saved Settings
	err := inTx(r.Context(), h.pool, func(q *db.Queries) error {
		old, err := LoadSettings(r.Context(), q)
		if err != nil {
			return err
		}
		if saved, err = SaveSettings(r.Context(), q, settingsFromAPI(req), &actor.ID); err != nil {
			return err
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: "settings.consent_update", EntityType: "settings",
			Changes: map[string]any{"unknown_marketing": map[string]any{"from": old.UnknownMarketing, "to": saved.UnknownMarketing},
				"confirm": saved.Confirm, "stop_keywords": len(saved.Keywords.Stop), "request_phrases": len(saved.Keywords.Request),
				"resume_keywords": len(saved.Keywords.Resume)}})
	})
	Invalidate()
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, SettingsToAPI(saved))
}

func inTx(ctx context.Context, pool *pgxpool.Pool, fn func(*db.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
