// Package fairness holds the platform settings that keep one tenant from
// slowing the others down (the noisy-neighbour model, docs/ARCHITECTURE.md
// §10.1):
//
//   - TenantShare: the largest part of a worker process's handler slots
//     one tenant's tasks may hold at a time (queue.TenantLimiter). Tasks
//     over it are deferred for about a second, not failed.
//   - TenantCampaignMPS: the combined send rate of all campaigns of one
//     tenant, on top of the per-number buckets of the send scheduler
//     (messaging.Throttle). 0 disables the cap.
//
// Both live in platform_settings under the key "fairness" (a JSON object;
// missing fields keep their defaults) and are re-read every half minute,
// so a change applies without a restart. The platform admin panel (WP2)
// edits the row; until then `messora migrate` / SQL does.
package fairness

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// SettingKey is the platform_settings key of the fairness settings.
const SettingKey = "fairness"

// Settings are the fairness limits.
type Settings struct {
	TenantShare       float64 `json:"tenant_share"`
	TenantCampaignMPS float64 `json:"tenant_campaign_mps"`
}

// Defaults are half of every worker for one tenant and 200 campaign messages
// per second per tenant (two and a half numbers at Meta's default 80).
var Defaults = Settings{TenantShare: 0.5, TenantCampaignMPS: 200}

// normalized replaces out-of-range values with the defaults.
func (s Settings) normalized() Settings {
	if math.IsNaN(s.TenantShare) || s.TenantShare <= 0 || s.TenantShare > 1 {
		s.TenantShare = Defaults.TenantShare
	}
	if math.IsNaN(s.TenantCampaignMPS) || s.TenantCampaignMPS < 0 {
		s.TenantCampaignMPS = Defaults.TenantCampaignMPS
	}
	return s
}

// ttl is how long a read value is used.
const ttl = 30 * time.Second

// Source reads the settings from platform_settings, cached in process. The
// value is platform-wide, so one cache serves every tenant.
type Source struct {
	pool *pgxpool.Pool
	now  func() time.Time

	mu      sync.Mutex
	v       Settings
	expires time.Time
}

// NewSource reads through pool (the application pool: platform_settings
// is readable without a tenant). A nil pool always returns the defaults.
func NewSource(pool *pgxpool.Pool) *Source {
	return &Source{pool: pool, now: time.Now, v: Defaults}
}

// Static returns a source that always answers s (tests).
func Static(s Settings) *Source {
	return &Source{v: s.normalized(), expires: time.Unix(1<<62, 0), now: time.Now}
}

// Get returns the current settings. A failed read keeps the last value
// (the defaults before the first success) for another period.
func (s *Source) Get(ctx context.Context) Settings {
	if s == nil {
		return Defaults
	}
	now := s.now()
	s.mu.Lock()
	if now.Before(s.expires) || s.pool == nil {
		v := s.v
		s.mu.Unlock()
		return v
	}
	s.mu.Unlock()

	v, err := s.read(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.v = v
	}
	s.expires = now.Add(ttl)
	return s.v
}

func (s *Source) read(ctx context.Context) (Settings, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	var raw []byte
	err := s.pool.QueryRow(tenant.System(ctx), `SELECT value FROM platform_settings WHERE key = $1`, SettingKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Defaults, nil
	}
	if err != nil {
		return Settings{}, err
	}
	v := Defaults
	if err := json.Unmarshal(raw, &v); err != nil {
		return Settings{}, err
	}
	return v.normalized(), nil
}

// TenantShare is Get(ctx).TenantShare, the shape queue.NewTenantLimiter
// takes.
func (s *Source) TenantShare(ctx context.Context) float64 { return s.Get(ctx).TenantShare }

// TenantCampaignMPS is Get(ctx).TenantCampaignMPS, the shape
// messaging.Throttle.WithTenantCampaignCap takes.
func (s *Source) TenantCampaignMPS(ctx context.Context) float64 {
	return s.Get(ctx).TenantCampaignMPS
}
