package consent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

const settingsKey = "consent"

// Unknown-consent policies for marketing.
const (
	UnknownAllow = "allow" // send, with a warning (default)
	UnknownBlock = "block"
)

// Messages are the texts of a confirmation reply, per language.
type Messages struct {
	TR string `json:"tr"`
	EN string `json:"en"`
}

// For returns the text for locale ("tr" or anything else = English).
func (m Messages) For(locale string) string {
	if locale == "tr" && m.TR != "" {
		return m.TR
	}
	if m.EN != "" {
		return m.EN
	}
	return m.TR
}

// Settings is the company's consent configuration ("Abonelik ve izin").
type Settings struct {
	// UnknownMarketing: what marketing does to contacts with unknown consent.
	UnknownMarketing string   `json:"unknown_marketing"`
	Keywords         Keywords `json:"keywords"`
	// Confirm sends a confirmation reply after a stop or resume keyword. It
	// goes out inside the window the customer's message just opened.
	Confirm       bool     `json:"confirm"`
	StopMessage   Messages `json:"stop_message"`
	ResumeMessage Messages `json:"resume_message"`
}

// DefaultSettings returns the initial configuration.
func DefaultSettings() Settings {
	return Settings{
		UnknownMarketing: UnknownAllow,
		Keywords:         DefaultKeywords(),
		Confirm:          true,
		StopMessage: Messages{
			TR: "Pazarlama mesajlarımızdan çıkarıldınız. Tekrar almak isterseniz BAŞLAT yazabilirsiniz.",
			EN: "You will no longer receive marketing messages from us. Reply START to subscribe again.",
		},
		ResumeMessage: Messages{
			TR: "Tekrar abone oldunuz. Çıkmak için istediğiniz zaman İPTAL yazabilirsiniz.",
			EN: "You are subscribed again. Reply STOP at any time to unsubscribe.",
		},
	}
}

// Policy returns the decision policy.
func (s Settings) Policy() Policy {
	return Policy{BlockUnknownMarketing: s.UnknownMarketing == UnknownBlock}
}

// Limits.
const (
	maxKeywords      = 50
	maxKeywordLength = 100
	maxMessageLength = 1024
)

// Validate checks and cleans s.
func (s *Settings) Validate() error {
	if s.UnknownMarketing != UnknownAllow && s.UnknownMarketing != UnknownBlock {
		return apperr.Validation("unknown_marketing", "Choose allow or block.")
	}
	clean := func(field string, list []string) ([]string, error) {
		if len(list) > maxKeywords {
			return nil, apperr.Validation(field, fmt.Sprintf("At most %d entries.", maxKeywords))
		}
		out := []string{}
		seen := map[string]bool{}
		for _, w := range list {
			w = strings.TrimSpace(w)
			n := Normalize(w)
			if n == "" || len([]rune(w)) > maxKeywordLength {
				return nil, apperr.Validation(field, fmt.Sprintf("Entries are 1–%d characters and contain a letter or digit.", maxKeywordLength))
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, w)
			}
		}
		return out, nil
	}
	var err error
	if s.Keywords.Stop, err = clean("keywords.stop", s.Keywords.Stop); err != nil {
		return err
	}
	if s.Keywords.Request, err = clean("keywords.request", s.Keywords.Request); err != nil {
		return err
	}
	if s.Keywords.Resume, err = clean("keywords.resume", s.Keywords.Resume); err != nil {
		return err
	}
	stops := map[string]bool{}
	for _, w := range s.Keywords.Stop {
		stops[Normalize(w)] = true
	}
	for _, w := range s.Keywords.Resume {
		if stops[Normalize(w)] {
			return apperr.Validation("keywords.resume", fmt.Sprintf("%q is also a stop word.", w))
		}
	}
	for field, m := range map[string]*Messages{"stop_message": &s.StopMessage, "resume_message": &s.ResumeMessage} {
		m.TR, m.EN = strings.TrimSpace(m.TR), strings.TrimSpace(m.EN)
		if len([]rune(m.TR)) > maxMessageLength || len([]rune(m.EN)) > maxMessageLength {
			return apperr.Validation(field, fmt.Sprintf("Messages are limited to %d characters.", maxMessageLength))
		}
		if s.Confirm && m.TR == "" && m.EN == "" {
			return apperr.Validation(field, "Write the confirmation message or turn confirmations off.")
		}
	}
	return nil
}

// LoadSettings reads the configuration (defaults when never saved).
func LoadSettings(ctx context.Context, q db.Querier) (Settings, error) {
	out := DefaultSettings()
	raw, err := q.GetSetting(ctx, settingsKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return DefaultSettings(), fmt.Errorf("consent: corrupt settings: %w", err)
	}
	if out.UnknownMarketing == "" {
		out.UnknownMarketing = UnknownAllow
	}
	return out, nil
}

// SaveSettings validates and stores s in q's transaction. Callers call
// Invalidate after the commit.
func SaveSettings(ctx context.Context, q db.Querier, s Settings, by *uuid.UUID) (Settings, error) {
	if err := s.Validate(); err != nil {
		return s, err
	}
	b, _ := json.Marshal(s)
	return s, q.UpsertSetting(ctx, db.UpsertSettingParams{Key: settingsKey, Value: b, UpdatedBy: by})
}

// cacheTTL bounds how long another process (the worker) may use settings
// older than a change made through the API.
const cacheTTL = 10 * time.Second

var cache = tenant.NewCache[Settings](cacheTTL)

// CachedSettings is LoadSettings with a short in-process cache per tenant:
// the send path checks consent for every message.
func CachedSettings(ctx context.Context, q db.Querier) (Settings, error) {
	if v, ok := cache.Get(ctx); ok {
		return v, nil
	}
	v, err := LoadSettings(ctx, q)
	if err != nil {
		return v, err
	}
	cache.Set(ctx, v)
	return v, nil
}

// Invalidate drops the cache after a change.
func Invalidate() { cache.Clear() }

// WorkspaceLocale returns the workspace's default language ("en" when unset),
// which picks the confirmation text.
func WorkspaceLocale(ctx context.Context, q db.Querier) string {
	raw, err := q.GetSetting(ctx, "workspace")
	if err != nil {
		return "en"
	}
	var v struct {
		DefaultLocale string `json:"default_locale"`
	}
	if json.Unmarshal(raw, &v) != nil || v.DefaultLocale == "" {
		return "en"
	}
	return v.DefaultLocale
}
