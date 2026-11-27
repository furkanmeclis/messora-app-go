// Package settings stores workspace-wide configuration as one typed document.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

const workspaceKey = "workspace"

// Settings is the workspace configuration.
type Settings struct {
	OrganizationName    string   `json:"organization_name"`
	DefaultTimezone     string   `json:"default_timezone"`
	DefaultLocale       string   `json:"default_locale"`
	DateFormat          string   `json:"date_format"`
	TimeFormat          string   `json:"time_format"`
	MFARequiredRoles    []string `json:"mfa_required_roles"`
	SessionIdleHours    int      `json:"session_idle_hours"`
	SessionLifetimeDays int      `json:"session_lifetime_days"`
	Logo                *Logo    `json:"logo,omitempty"`
	Setup               Setup    `json:"setup"`
	PWA                 PWA      `json:"pwa"`

	// Locked lists the fields set by environment variables (never stored).
	Locked []string `json:"-"`
}

// Logo is the uploaded organisation logo, stored under the media directory.
type Logo struct {
	ContentType string `json:"content_type"`
	// Version changes with every upload so browsers refetch it.
	Version string `json:"version"`
}

// Setup tracks the first-run wizard.
type Setup struct {
	Completed bool     `json:"completed"`
	Steps     []string `json:"steps"`
}

// Locales, date and time formats the panel supports.
var (
	Locales     = []string{"en", "tr"}
	DateFormats = []string{"dd.MM.yyyy", "dd/MM/yyyy", "MM/dd/yyyy", "yyyy-MM-dd", "d MMM yyyy"}
	TimeFormats = []string{"24h", "12h"}
	SetupSteps  = []string{"organization", "language", "region", "email", "whatsapp", "security", "team"}
)

// Defaults returns the initial settings. Two-factor authentication is not
// required for any role until an owner or admin turns it on.
func Defaults() Settings {
	return Settings{
		OrganizationName:    "Messora",
		DefaultTimezone:     "UTC",
		DefaultLocale:       "en",
		DateFormat:          "dd.MM.yyyy",
		TimeFormat:          "24h",
		MFARequiredRoles:    []string{},
		SessionIdleHours:    12,
		SessionLifetimeDays: 30,
		Setup:               Setup{Steps: []string{}},
		PWA:                 DefaultPWA(),
	}
}

// Patch is a partial update; nil fields are unchanged.
type Patch struct {
	OrganizationName    *string
	DefaultTimezone     *string
	DefaultLocale       *string
	DateFormat          *string
	TimeFormat          *string
	MFARequiredRoles    *[]string
	SessionIdleHours    *int
	SessionLifetimeDays *int
	SetupSteps          *[]string
	SetupCompleted      *bool
	PWA                 *PWA
}

// fields names the patched fields (API names), for lock checks.
func (p Patch) fields() []string {
	var out []string
	add := func(set bool, name string) {
		if set {
			out = append(out, name)
		}
	}
	add(p.OrganizationName != nil, "organization_name")
	add(p.DefaultTimezone != nil, "default_timezone")
	add(p.DefaultLocale != nil, "default_locale")
	add(p.DateFormat != nil, "date_format")
	add(p.TimeFormat != nil, "time_format")
	add(p.MFARequiredRoles != nil, "mfa_required_roles")
	add(p.SessionIdleHours != nil, "session_idle_hours")
	add(p.SessionLifetimeDays != nil, "session_lifetime_days")
	return out
}

// ErrInvalid wraps validation failures.
var ErrInvalid = errors.New("settings: invalid")

// ErrLocked is returned for changes to fields set by environment variables.
var ErrLocked = errors.New("settings: managed by an environment variable")

// Apply returns s with p applied, or ErrInvalid.
func (s Settings) Apply(p Patch) (Settings, error) {
	for _, f := range p.fields() {
		if slices.Contains(s.Locked, f) {
			return s, fmt.Errorf("%w: %s", ErrLocked, f)
		}
	}
	if p.OrganizationName != nil {
		s.OrganizationName = *p.OrganizationName
	}
	if p.DefaultTimezone != nil {
		if _, err := time.LoadLocation(*p.DefaultTimezone); err != nil {
			return s, fmt.Errorf("%w: unknown timezone %q", ErrInvalid, *p.DefaultTimezone)
		}
		s.DefaultTimezone = *p.DefaultTimezone
	}
	if p.DefaultLocale != nil {
		if !slices.Contains(Locales, *p.DefaultLocale) {
			return s, fmt.Errorf("%w: unsupported language %q", ErrInvalid, *p.DefaultLocale)
		}
		s.DefaultLocale = *p.DefaultLocale
	}
	if p.DateFormat != nil {
		if !slices.Contains(DateFormats, *p.DateFormat) {
			return s, fmt.Errorf("%w: unsupported date format %q", ErrInvalid, *p.DateFormat)
		}
		s.DateFormat = *p.DateFormat
	}
	if p.TimeFormat != nil {
		if !slices.Contains(TimeFormats, *p.TimeFormat) {
			return s, fmt.Errorf("%w: time format must be 24h or 12h", ErrInvalid)
		}
		s.TimeFormat = *p.TimeFormat
	}
	if p.PWA != nil {
		if err := p.PWA.Validate(); err != nil {
			return s, err
		}
		s.PWA = *p.PWA
	}
	if p.SetupSteps != nil {
		steps := []string{}
		for _, st := range *p.SetupSteps {
			if !slices.Contains(SetupSteps, st) {
				return s, fmt.Errorf("%w: unknown setup step %q", ErrInvalid, st)
			}
			if !slices.Contains(steps, st) {
				steps = append(steps, st)
			}
		}
		s.Setup.Steps = steps
	}
	if p.SetupCompleted != nil {
		s.Setup.Completed = *p.SetupCompleted
	}
	if p.MFARequiredRoles != nil {
		r := slices.Clone(*p.MFARequiredRoles)
		slices.Sort(r)
		s.MFARequiredRoles = slices.Compact(r)
	}
	if p.SessionIdleHours != nil {
		s.SessionIdleHours = *p.SessionIdleHours
	}
	if p.SessionLifetimeDays != nil {
		s.SessionLifetimeDays = *p.SessionLifetimeDays
	}
	if s.SessionIdleHours > s.SessionLifetimeDays*24 {
		return s, fmt.Errorf("%w: idle timeout cannot exceed the session lifetime", ErrInvalid)
	}
	return s, nil
}

// MFARequired reports whether role must use two-factor authentication.
func (s Settings) MFARequired(role string) bool { return slices.Contains(s.MFARequiredRoles, role) }

// Store reads and writes settings with a short in-process cache.
type Store struct {
	q         db.Querier
	ttl       time.Duration
	overrides Patch
	locked    []string

	mu      sync.Mutex
	cached  *Settings
	expires time.Time
}

// NewStore returns a store; ttl bounds how long other replicas may serve a
// stale value after an update.
func NewStore(q db.Querier, ttl time.Duration) *Store { return &Store{q: q, ttl: ttl} }

// SetOverrides installs values from environment variables (setting name →
// value). They are applied on every read, never stored, and the panel cannot
// change them.
func (s *Store) SetOverrides(env map[string]string) error {
	var p Patch
	for name, v := range env {
		v := v
		switch name {
		case "organization_name":
			p.OrganizationName = &v
		case "default_locale":
			p.DefaultLocale = &v
		case "default_timezone":
			p.DefaultTimezone = &v
		case "date_format":
			p.DateFormat = &v
		case "time_format":
			p.TimeFormat = &v
		case "mfa_required_roles":
			roles := []string{}
			if v != "none" {
				for _, r := range strings.Split(v, ",") {
					if r = strings.TrimSpace(r); r != "" {
						roles = append(roles, r)
					}
				}
			}
			for _, r := range roles {
				if !slices.Contains([]string{"owner", "admin", "manager", "agent"}, r) {
					return fmt.Errorf("settings: unknown role %q in the two-factor roles variable", r)
				}
			}
			p.MFARequiredRoles = &roles
		default:
			return fmt.Errorf("settings: %q cannot be set by an environment variable", name)
		}
	}
	if _, err := Defaults().Apply(p); err != nil {
		return fmt.Errorf("settings from environment variables: %w", err)
	}
	locked := p.fields()
	slices.Sort(locked)
	s.mu.Lock()
	s.overrides, s.locked, s.cached = p, locked, nil
	s.mu.Unlock()
	return nil
}

// withOverrides applies the environment values to a stored document.
func (s *Store) withOverrides(v Settings) Settings {
	s.mu.Lock()
	p, locked := s.overrides, s.locked
	s.mu.Unlock()
	if out, err := v.Apply(p); err == nil {
		v = out
	}
	v.Locked = slices.Clone(locked)
	if v.Locked == nil {
		v.Locked = []string{}
	}
	return v
}

// Get returns the current settings (defaults merged in).
func (s *Store) Get(ctx context.Context) (Settings, error) {
	s.mu.Lock()
	if s.cached != nil && time.Now().Before(s.expires) {
		v := *s.cached
		s.mu.Unlock()
		return v, nil
	}
	s.mu.Unlock()
	v, err := s.load(ctx, s.q)
	if err != nil {
		return Settings{}, err
	}
	v = s.withOverrides(v)
	s.mu.Lock()
	s.cached, s.expires = &v, time.Now().Add(s.ttl)
	s.mu.Unlock()
	return v, nil
}

func (s *Store) load(ctx context.Context, q db.Querier) (Settings, error) {
	out := Defaults()
	raw, err := q.GetSetting(ctx, workspaceKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Defaults(), fmt.Errorf("settings: corrupt workspace document: %w", err)
	}
	return out, nil
}

// Update applies p inside q's transaction and returns old and new values.
// Callers must call Invalidate after the transaction commits: invalidating
// earlier would let a concurrent read re-cache the pre-commit value.
func (s *Store) Update(ctx context.Context, q db.Querier, p Patch, updatedBy *uuid.UUID) (old, updated Settings, err error) {
	old, err = s.load(ctx, q)
	if err != nil {
		return
	}
	s.mu.Lock()
	old.Locked = s.locked
	s.mu.Unlock()
	stored, err := old.Apply(p)
	if err != nil {
		return
	}
	if err = s.save(ctx, q, stored, updatedBy); err != nil {
		return
	}
	return s.withOverrides(old), s.withOverrides(stored), nil
}

func (s *Store) save(ctx context.Context, q db.Querier, v Settings, updatedBy *uuid.UUID) error {
	b, _ := json.Marshal(v)
	return q.UpsertSetting(ctx, db.UpsertSettingParams{Key: workspaceKey, Value: b, UpdatedBy: updatedBy})
}

// SetLogo records (or with nil clears) the logo inside q's transaction.
// Callers must call Invalidate after the transaction commits.
func (s *Store) SetLogo(ctx context.Context, q db.Querier, logo *Logo, updatedBy *uuid.UUID) error {
	v, err := s.load(ctx, q)
	if err != nil {
		return err
	}
	v.Logo = logo
	return s.save(ctx, q, v, updatedBy)
}

// Invalidate drops the cache.
func (s *Store) Invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}
