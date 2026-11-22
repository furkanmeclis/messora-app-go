// Package settings stores workspace-wide configuration as one typed document.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
	MFARequiredRoles    []string `json:"mfa_required_roles"`
	SessionIdleHours    int      `json:"session_idle_hours"`
	SessionLifetimeDays int      `json:"session_lifetime_days"`
}

// Defaults returns the initial settings: owners and admins must use
// two-factor authentication.
func Defaults() Settings {
	return Settings{
		OrganizationName:    "Messora",
		DefaultTimezone:     "UTC",
		MFARequiredRoles:    []string{"owner", "admin"},
		SessionIdleHours:    12,
		SessionLifetimeDays: 30,
	}
}

// Patch is a partial update; nil fields are unchanged.
type Patch struct {
	OrganizationName    *string
	DefaultTimezone     *string
	MFARequiredRoles    *[]string
	SessionIdleHours    *int
	SessionLifetimeDays *int
}

// ErrInvalid wraps validation failures.
var ErrInvalid = errors.New("settings: invalid")

// Apply returns s with p applied, or ErrInvalid.
func (s Settings) Apply(p Patch) (Settings, error) {
	if p.OrganizationName != nil {
		s.OrganizationName = *p.OrganizationName
	}
	if p.DefaultTimezone != nil {
		if _, err := time.LoadLocation(*p.DefaultTimezone); err != nil {
			return s, fmt.Errorf("%w: unknown timezone %q", ErrInvalid, *p.DefaultTimezone)
		}
		s.DefaultTimezone = *p.DefaultTimezone
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
	q   db.Querier
	ttl time.Duration

	mu      sync.Mutex
	cached  *Settings
	expires time.Time
}

// NewStore returns a store; ttl bounds how long other replicas may serve a
// stale value after an update.
func NewStore(q db.Querier, ttl time.Duration) *Store { return &Store{q: q, ttl: ttl} }

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
	updated, err = old.Apply(p)
	if err != nil {
		return
	}
	b, _ := json.Marshal(updated)
	if err = q.UpsertSetting(ctx, db.UpsertSettingParams{Key: workspaceKey, Value: b, UpdatedBy: updatedBy}); err != nil {
		return
	}
	return old, updated, nil
}

// Invalidate drops the cache.
func (s *Store) Invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

func errorsIsInvalid(err error) bool { return errors.Is(err, ErrInvalid) }
