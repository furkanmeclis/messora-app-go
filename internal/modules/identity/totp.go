package identity

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// goredisKeepTTL keeps a key's remaining TTL on SET.
const goredisKeepTTL = goredis.KeepTTL

const recoveryCodeCount = 10

func totpAD(userID uuid.UUID) string { return "totp:" + userID.String() }

func constantTimeEqual(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }

func normalizeRecoveryCode(c string) string {
	c = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(c), " ", ""))
	if len(c) == 10 && !strings.Contains(c, "-") {
		c = c[:5] + "-" + c[5:]
	}
	return c
}

// validateTOTP checks code (±1 step of 30 s) and rejects a code that was
// already used, so an observed code cannot be replayed.
func (s *Service) validateTOTP(ctx context.Context, userID uuid.UUID, secret, code string) (bool, error) {
	now := s.now()
	for _, skew := range []int64{0, -1, 1} {
		t := now.Add(time.Duration(skew) * 30 * time.Second)
		ok, err := totp.ValidateCustom(code, secret, t, totp.ValidateOpts{Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if err != nil {
			return false, nil
		}
		if !ok {
			continue
		}
		step := t.Unix() / 30
		set, err := s.rdb.SetNX(ctx, fmt.Sprintf("messora:totp:used:%s:%d", userID, step), 1, 3*time.Minute).Result()
		if err != nil {
			return false, err
		}
		return set, nil // false: this code was already used
	}
	return false, nil
}

// StartTOTPSetup generates a pending secret. It becomes active only after
// ConfirmTOTP proves the authenticator app holds it.
func (s *Service) StartTOTPSetup(ctx context.Context, p *principal.Principal) (api.TotpSetup, error) {
	if err := s.requireReauthUnlessPending(p); err != nil {
		return api.TotpSetup{}, err
	}
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return api.TotpSetup{}, err
	}
	if u.TotpConfirmedAt != nil {
		return api.TotpSetup{}, apperr.Conflict("MFA_ALREADY_ENABLED", "Two-factor authentication is already enabled")
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return api.TotpSetup{}, err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: st.OrganizationName, AccountName: u.Email, SecretSize: 20})
	if err != nil {
		return api.TotpSetup{}, err
	}
	enc, err := s.sealer.Seal([]byte(key.Secret()), totpAD(u.ID))
	if err != nil {
		return api.TotpSetup{}, err
	}
	if err := s.q.SetTotpSecret(ctx, db.SetTotpSecretParams{ID: u.ID, TotpSecretEnc: enc}); err != nil {
		return api.TotpSetup{}, err
	}
	return api.TotpSetup{Secret: key.Secret(), OtpauthUri: key.URL()}, nil
}

// requireReauthUnlessPending lets a user whose role mandates two-factor
// authentication enrol right after signing in (the fresh login counts as
// re-authentication anyway).
func (s *Service) requireReauthUnlessPending(p *principal.Principal) error {
	if p.MFAPending {
		return nil
	}
	return s.requireReauth(p)
}

// ConfirmTOTP activates the pending secret and returns new recovery codes.
func (s *Service) ConfirmTOTP(ctx context.Context, p *principal.Principal, code string) ([]string, error) {
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if u.TotpConfirmedAt != nil {
		return nil, apperr.Conflict("MFA_ALREADY_ENABLED", "Two-factor authentication is already enabled")
	}
	if u.TotpSecretEnc == nil {
		return nil, apperr.Conflict("MFA_NOT_STARTED", "Start the setup first")
	}
	secret, err := s.sealer.Open(u.TotpSecretEnc, totpAD(u.ID))
	if err != nil {
		return nil, err
	}
	ok, err := s.validateTOTP(ctx, u.ID, string(secret), code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.Validation("code", "The code is incorrect. Check the time on your phone and try again.")
	}
	var codes []string
	err = s.tx(ctx, func(q *db.Queries) error {
		now := s.now()
		if err := q.SetTotpSecret(ctx, db.SetTotpSecretParams{ID: u.ID, TotpSecretEnc: u.TotpSecretEnc, TotpConfirmedAt: &now}); err != nil {
			return err
		}
		if codes, err = s.replaceRecoveryCodes(ctx, q, u.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.mfa_enable", EntityType: "user", EntityID: uuidPtr(u.ID)})
	})
	return codes, err
}

// DisableTOTP turns two-factor authentication off.
func (s *Service) DisableTOTP(ctx context.Context, p *principal.Principal) error {
	if err := s.requireReauth(p); err != nil {
		return err
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	if st.MFARequired(p.RoleKey) {
		return apperr.Forbidden("MFA_REQUIRED_BY_POLICY", "Your role requires two-factor authentication")
	}
	// The second factor belongs to the person: a platform admin keeps it.
	if u, err := s.q.GetUserByID(ctx, p.ID); err != nil || u.IsPlatformAdmin {
		if err == nil {
			err = apperr.Forbidden("MFA_REQUIRED_BY_POLICY", "Platform admins must keep two-factor authentication")
		}
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.SetTotpSecret(ctx, db.SetTotpSecretParams{ID: p.ID}); err != nil {
			return err
		}
		if err := q.DeleteRecoveryCodes(ctx, p.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.mfa_disable", EntityType: "user", EntityID: uuidPtr(p.ID)})
	})
}

// RegenerateRecoveryCodes replaces every recovery code.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, p *principal.Principal) ([]string, error) {
	if err := s.requireReauth(p); err != nil {
		return nil, err
	}
	u, err := s.q.GetUserByID(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if u.TotpConfirmedAt == nil {
		return nil, apperr.Conflict("MFA_NOT_ENABLED", "Enable two-factor authentication first")
	}
	var codes []string
	err = s.tx(ctx, func(q *db.Queries) error {
		var err error
		if codes, err = s.replaceRecoveryCodes(ctx, q, u.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "user.recovery_codes_regenerate", EntityType: "user", EntityID: uuidPtr(u.ID)})
	})
	return codes, err
}

func (s *Service) replaceRecoveryCodes(ctx context.Context, q *db.Queries, userID uuid.UUID) ([]string, error) {
	if err := q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, err
	}
	codes := make([]string, 0, recoveryCodeCount)
	for len(codes) < recoveryCodeCount {
		c, err := crypto.RecoveryCode()
		if err != nil {
			return nil, err
		}
		if err := q.InsertRecoveryCode(ctx, db.InsertRecoveryCodeParams{UserID: userID, CodeHash: crypto.HashToken(c)}); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, nil
}
