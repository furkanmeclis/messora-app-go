// Package licensing keeps the installation's licence, checks in with the
// licence gate and tells the rest of Messora whether it may send.
package licensing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/license"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

const settingsKey = "license"

// TypeCheckIn is the periodic task that reports to the licence gate.
const TypeCheckIn = "license:check_in"

// Options configures the service.
type Options struct {
	LicenseKeys license.Keyring
	GateKeys    license.Keyring
	GateURL     string // empty: no check-in
	Version     string
	HTTP        *http.Client
	Logger      *slog.Logger
	Now         func() time.Time
}

// Service owns the stored licence.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	o    Options

	mu      sync.Mutex
	cached  *license.State
	stored  license.Stored
	expires time.Time
}

// New returns the service.
func New(pool *pgxpool.Pool, o Options) *Service {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{pool: pool, q: db.New(pool), o: o}
}

// load returns the stored record, creating it (instance id, install time)
// on first use.
func (s *Service) load(ctx context.Context, q *db.Queries) (license.Stored, error) {
	var st license.Stored
	raw, err := q.GetSetting(ctx, settingsKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return st, err
	}
	if err == nil {
		if err := json.Unmarshal(raw, &st); err != nil {
			return st, fmt.Errorf("licensing: corrupt record: %w", err)
		}
	}
	if st.InstanceID == "" {
		st.InstanceID = uuid.NewString()
		st.InstalledAt = s.o.Now().UTC()
		st.LastSeen = st.InstalledAt
		if err := s.save(ctx, q, st); err != nil {
			return st, err
		}
	}
	return st, nil
}

func (s *Service) save(ctx context.Context, q *db.Queries, st license.Stored) error {
	b, _ := json.Marshal(st)
	return q.UpsertSetting(ctx, db.UpsertSettingParams{Key: settingsKey, Value: b})
}

// State evaluates the licence (cached for a minute) and records the
// latest time seen, so a clock set back cannot extend it.
func (s *Service) State(ctx context.Context) (license.State, license.Stored, error) {
	s.mu.Lock()
	if s.cached != nil && s.o.Now().Before(s.expires) {
		st, stored := *s.cached, s.stored
		s.mu.Unlock()
		return st, stored, nil
	}
	s.mu.Unlock()
	stored, err := s.load(ctx, s.q)
	if err != nil {
		return license.State{}, stored, err
	}
	now := s.o.Now().UTC()
	if now.Sub(stored.LastSeen) > time.Hour {
		stored.LastSeen = now
		if err := s.save(ctx, s.q, stored); err != nil {
			s.o.Logger.WarnContext(ctx, "licensing: could not record the time", "err", err)
		}
	}
	state := license.Evaluate(now, stored, s.o.LicenseKeys)
	s.mu.Lock()
	s.cached, s.stored, s.expires = &state, stored, s.o.Now().Add(time.Minute)
	s.mu.Unlock()
	return state, stored, nil
}

// Allowed reports whether sending may happen; errors fail open (a database
// hiccup must not stop messaging — the next evaluation decides).
func (s *Service) Allowed(ctx context.Context) bool {
	st, _, err := s.State(ctx)
	if err != nil {
		return true
	}
	return st.Allowed()
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// SetKey verifies and stores a licence key.
func (s *Service) SetKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	p, err := license.Parse(key, s.o.LicenseKeys)
	if err != nil {
		return apperr.Validation("key", "This licence key is not valid: check that it was copied completely.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	st, err := s.load(ctx, q)
	if err != nil {
		return err
	}
	now := s.o.Now().UTC()
	st.Key, st.LicensedAt = key, &now
	st.RevokedAt, st.Revocation = nil, ""
	if err := s.save(ctx, q, st); err != nil {
		return err
	}
	if err := audit.Record(ctx, q, audit.Entry{Action: "license.set", EntityType: "license",
		Changes: map[string]any{"license_id": p.ID, "customer": p.Customer, "plan": p.Plan}}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.invalidate()
	return nil
}

// CheckIn reports to the licence gate and applies its signed answer
// (revocations, replacements, the newest release).
func (s *Service) CheckIn(ctx context.Context) error {
	if s.o.GateURL == "" {
		return nil
	}
	st, err := s.load(ctx, s.q)
	if err != nil || st.Key == "" {
		return err
	}
	p, err := license.Parse(st.Key, s.o.LicenseKeys)
	if err != nil {
		return nil // nothing valid to report
	}
	body, _ := json.Marshal(license.CheckInRequest{Key: st.Key, InstanceID: st.InstanceID, Version: s.o.Version})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.o.GateURL, "/")+"/v1/check-in", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Messora/"+s.o.Version)
	res, err := s.o.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("licensing: gate unreachable: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("licensing: gate answered %d", res.StatusCode)
	}
	a, err := license.OpenAnswer(strings.TrimSpace(string(raw)), s.o.GateKeys)
	if err != nil {
		return fmt.Errorf("licensing: gate answer: %w", err)
	}
	if a.LicenseID != p.ID || a.InstanceID != st.InstanceID {
		return errors.New("licensing: gate answer is for another licence or installation")
	}
	now := s.o.Now().UTC()
	switch a.Status {
	case "ok":
		st.LastCheckIn, st.RevokedAt, st.Revocation = &now, nil, ""
	case license.ReasonRevoked, license.ReasonSuperseded:
		st.LastCheckIn = &now
		if st.RevokedAt == nil {
			st.RevokedAt = &now
		}
		st.Revocation = a.Status
	default:
		return fmt.Errorf("licensing: gate does not know this licence (%s)", a.Status)
	}
	st.Latest = a.Latest
	if err := s.save(ctx, s.q, st); err != nil {
		return err
	}
	s.invalidate()
	return nil
}
