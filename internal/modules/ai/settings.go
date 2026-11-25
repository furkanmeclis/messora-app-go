package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
)

// settingsKey stores the company AI settings in the settings table.
const settingsKey = "ai"

// Texts is a text in the panel's languages.
type Texts struct {
	TR string `json:"tr"`
	EN string `json:"en"`
}

// For returns the text in lang, falling back to the other language.
func (t Texts) For(lang Language) string {
	if lang == LangEN && t.EN != "" || t.TR == "" {
		return t.EN
	}
	return t.TR
}

// Settings are the company-wide AI settings.
type Settings struct {
	// Enabled is the master switch: while off every AI call fails with
	// ErrAIDisabled and no AI work runs.
	Enabled bool `json:"enabled"`
	// DefaultProviderID and DefaultModel are used when a profile or call
	// names none.
	DefaultProviderID *uuid.UUID `json:"default_provider_id"`
	DefaultModel      string     `json:"default_model"`
	// Disclosure is how the AI introduces itself as an AI.
	Disclosure Texts `json:"disclosure"`
	// DataSharingNotice tells customers their messages go to an AI provider.
	DataSharingNotice Texts `json:"data_sharing_notice"`
	// MaxRepliesPerHour caps the AI requests (replies and drafts) of one
	// conversation per hour (rate and cost guard); above it the
	// conversation goes to a person. 0 = DefaultMaxRepliesPerHour.
	MaxRepliesPerHour int `json:"max_replies_per_hour,omitempty"`
	// ContextMessages is how many recent messages an AI turn sees.
	// 0 = DefaultContextMessages.
	ContextMessages int `json:"context_messages,omitempty"`
}

// Defaults and bounds of the conversation AI settings.
const (
	DefaultMaxRepliesPerHour = 20
	DefaultContextMessages   = 20
	MaxContextMessages       = 100
)

// RepliesPerHour is the effective per-conversation hourly cap.
func (s Settings) RepliesPerHour() int {
	if s.MaxRepliesPerHour <= 0 {
		return DefaultMaxRepliesPerHour
	}
	return s.MaxRepliesPerHour
}

// ContextSize is the effective number of messages given to the model.
func (s Settings) ContextSize() int {
	if s.ContextMessages <= 0 {
		return DefaultContextMessages
	}
	return min(s.ContextMessages, MaxContextMessages)
}

// DefaultSettings returns the initial settings: AI off until an admin connects a provider and turns it on.
func DefaultSettings() Settings {
	return Settings{
		Disclosure: Texts{
			TR: "Bu sohbette yapay zekâ asistanı yanıt veriyor. İstediğiniz an bir temsilciye bağlanabilirsiniz.",
			EN: "An AI assistant is answering in this chat. You can ask for a person at any time.",
		},
		DataSharingNotice: Texts{
			TR: "Mesajlarınız yanıt üretmek için yapay zekâ sağlayıcımıza iletilir ve hiçbir modelin eğitiminde kullanılmaz.",
			EN: "Your messages are sent to our AI provider to generate replies and are never used to train any model.",
		},
	}
}

// Settings returns the stored settings (defaults when none).
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	raw, err := s.q.GetSetting(ctx, settingsKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return DefaultSettings(), nil
	}
	return st, nil
}

// Enabled reports whether AI is switched on for the company.
func (s *Service) Enabled(ctx context.Context) bool {
	st, err := s.Settings(ctx)
	return err == nil && st.Enabled
}

const maxNoticeLen = 1000

// SaveSettings validates and stores the settings.
func (s *Service) SaveSettings(ctx context.Context, in Settings, actor *uuid.UUID) (Settings, error) {
	in.DefaultModel = strings.TrimSpace(in.DefaultModel)
	for _, t := range []*string{&in.Disclosure.TR, &in.Disclosure.EN, &in.DataSharingNotice.TR, &in.DataSharingNotice.EN} {
		*t = strings.TrimSpace(*t)
		if len([]rune(*t)) > maxNoticeLen {
			return Settings{}, &FieldError{"disclosure", "Texts are limited to 1000 characters."}
		}
	}
	if in.Disclosure.TR == "" && in.Disclosure.EN == "" {
		return Settings{}, &FieldError{"disclosure", "Write the AI disclosure in at least one language."}
	}
	if in.MaxRepliesPerHour < 0 || in.MaxRepliesPerHour > 1000 {
		return Settings{}, &FieldError{"max_replies_per_hour", "Between 1 and 1000 AI replies per conversation and hour."}
	}
	if in.ContextMessages < 0 || in.ContextMessages > MaxContextMessages {
		return Settings{}, &FieldError{"context_messages", fmt.Sprintf("Between 2 and %d messages.", MaxContextMessages)}
	}
	if len(in.DefaultModel) > 200 {
		return Settings{}, &FieldError{"default_model", "Model id is too long."}
	}
	if in.DefaultProviderID != nil {
		p, err := s.q.GetAIProvider(ctx, *in.DefaultProviderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Settings{}, &FieldError{"default_provider_id", "Unknown provider."}
		}
		if err != nil {
			return Settings{}, err
		}
		if Kind(p.Kind) == KindFake && !s.opts.AllowFake {
			return Settings{}, &FieldError{"default_provider_id", "The fake provider is not available here."}
		}
		if in.DefaultModel == "" {
			return Settings{}, &FieldError{"default_model", "Choose the default model of the provider."}
		}
	} else if in.DefaultModel != "" {
		return Settings{}, &FieldError{"default_provider_id", "Choose the provider of the default model."}
	}
	old, err := s.Settings(ctx)
	if err != nil {
		return Settings{}, err
	}
	raw, _ := json.Marshal(in)
	err = s.tx(ctx, func(q *db.Queries) error {
		if err := q.UpsertSetting(ctx, db.UpsertSettingParams{Key: settingsKey, Value: raw, UpdatedBy: actor}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "ai.settings.update", EntityType: "ai_settings", Changes: settingsDiff(old, in)})
	})
	return in, err
}

func settingsDiff(a, b Settings) map[string]any {
	ch := map[string]any{}
	if a.Enabled != b.Enabled {
		ch["enabled"] = []any{a.Enabled, b.Enabled}
	}
	if !uuidPtrEq(a.DefaultProviderID, b.DefaultProviderID) {
		ch["default_provider_id"] = []any{a.DefaultProviderID, b.DefaultProviderID}
	}
	if a.DefaultModel != b.DefaultModel {
		ch["default_model"] = []any{a.DefaultModel, b.DefaultModel}
	}
	if a.Disclosure != b.Disclosure {
		ch["disclosure"] = []any{a.Disclosure, b.Disclosure}
	}
	if a.DataSharingNotice != b.DataSharingNotice {
		ch["data_sharing_notice"] = []any{a.DataSharingNotice, b.DataSharingNotice}
	}
	if a.RepliesPerHour() != b.RepliesPerHour() {
		ch["max_replies_per_hour"] = []any{a.RepliesPerHour(), b.RepliesPerHour()}
	}
	if a.ContextSize() != b.ContextSize() {
		ch["context_messages"] = []any{a.ContextSize(), b.ContextSize()}
	}
	return ch
}

func uuidPtrEq(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
