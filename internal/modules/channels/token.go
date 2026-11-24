package channels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// TypeTokenCheck is the periodic task that re-validates every account's
// access token with Meta.
const TypeTokenCheck = "channels:token_check"

// Scopes a token must carry for Messora to work.
var requiredScopes = []string{"whatsapp_business_management", "whatsapp_business_messaging"}

// ExpiryNotice is how long before a token's expiry the owners are warned.
const ExpiryNotice = 7 * 24 * time.Hour

// shortLived: user tokens closer than this to expiry are exchanged for a
// long-lived one when the app secret is known (Graph Explorer tokens live
// about an hour, "temporary" dashboard tokens 24 hours).
const shortLived = 3 * 24 * time.Hour

// Notifier tells the owners and admins that an account needs attention.
type Notifier func(ctx context.Context, subject, body string)

// SetNotifier installs the notifier (e-mail to owners and admins).
func (s *Service) SetNotifier(n Notifier) { s.notify = n }

// tokenReport is what Messora learnt about a token.
type tokenReport struct {
	token   string // possibly exchanged for a long-lived one
	info    whatsapp.TokenInfo
	checked bool // debug_token answered
	warning string
}

// tokenProblem is a token Meta does not accept for this account.
type tokenProblem struct{ msg string }

func (p *tokenProblem) Error() string { return p.msg }

// inspectToken asks Meta what token is: valid, which type, until when, with
// which permissions on which WhatsApp Business Accounts. A token that will
// not work returns *tokenProblem.
func (s *Service) inspectToken(ctx context.Context, wabaID, appID, appSecret, token string) (tokenReport, error) {
	rep := tokenReport{token: token}
	inspector := ""
	if appID != "" && appSecret != "" {
		inspector = appID + "|" + appSecret
	}
	info, err := s.wa.DebugToken(ctx, token, inspector)
	if err != nil {
		if errors.Is(err, whatsapp.ErrDryRun) {
			return rep, err
		}
		// Inspection is best effort: some tokens may not inspect
		// themselves. The phone-number check that follows still proves the
		// token works.
		rep.warning = "Meta did not report this token's expiry, so Messora cannot warn before it stops working. Use a System User token, or add the App ID and App secret."
		s.log.Warn("debug_token failed", "waba_id", wabaID, "error", err)
		return rep, nil
	}
	rep.checked, rep.info = true, info
	if !info.Valid {
		msg := "Meta says this access token is not valid"
		if info.Error != "" {
			msg += ": " + info.Error
		}
		return rep, &tokenProblem{msg + ". Create a System User token in Business Settings (it never expires)."}
	}
	var missing []string
	for _, sc := range requiredScopes {
		if !slices.Contains(info.Scopes, sc) {
			missing = append(missing, sc)
		}
	}
	if len(missing) > 0 {
		return rep, &tokenProblem{"The access token lacks the permission " + strings.Join(missing, " and ") + "."}
	}
	if ids := info.Targets["whatsapp_business_management"]; len(ids) > 0 && !slices.Contains(ids, wabaID) {
		return rep, &tokenProblem{fmt.Sprintf("The access token has no access to WhatsApp Business Account %s; assign the account to the System User in Business Settings.", wabaID)}
	}
	// A short-lived user token: trade it for a long-lived one.
	if info.ExpiresAt != nil && info.Type == "USER" && appID != "" && appSecret != "" && time.Until(*info.ExpiresAt) < shortLived {
		long, _, err := s.wa.ExchangeToken(ctx, appID, appSecret, token)
		if err == nil && long != "" {
			if again, err := s.wa.DebugToken(ctx, long, inspector); err == nil && again.Valid {
				rep.token, rep.info = long, again
			}
		} else if err != nil {
			s.log.Warn("token exchange failed", "waba_id", wabaID, "error", err)
		}
	}
	if exp := rep.info.ExpiresAt; exp != nil {
		rep.warning = fmt.Sprintf("This token expires on %s. A System User token (Business Settings → System users → Generate token) never expires; replace it before then.", exp.Format("2006-01-02"))
	}
	return rep, nil
}

// asValidation turns a token problem into a form error on access_token.
func asValidation(err error) error {
	var p *tokenProblem
	if errors.As(err, &p) {
		return apperr.Validation("access_token", p.msg)
	}
	if errors.Is(err, whatsapp.ErrDryRun) {
		return metaFailure(err)
	}
	return err
}

func (s *Service) saveTokenInfo(ctx context.Context, q *db.Queries, id uuid.UUID, rep tokenReport) (db.WhatsappAccount, error) {
	p := db.SetWhatsAppTokenInfoParams{ID: id}
	if rep.checked {
		t := rep.info.Type
		p.TokenType, p.TokenExpiresAt, p.TokenDataAccessExpiresAt, p.TokenScopes = &t, rep.info.ExpiresAt, rep.info.DataAccessExpiresAt, rep.info.Scopes
	}
	if rep.warning != "" {
		w := rep.warning
		p.TokenWarning = &w
	}
	return q.SetWhatsAppTokenInfo(ctx, p)
}

// appCredentials returns an account's app id and app secret, if stored.
func (s *Service) appCredentials(acc db.WhatsappAccount) (appID, appSecret string) {
	if acc.AppID != nil {
		appID = *acc.AppID
	}
	if len(acc.AppSecretEnc) > 0 {
		if b, err := s.sealer.Open(acc.AppSecretEnc, secretAD(acc.ID)); err == nil {
			appSecret = string(b)
		}
	}
	return appID, appSecret
}

// CheckToken re-validates one account's token now (the "Check connection"
// button and the periodic job). The account is flagged when Meta refuses
// the token and reactivated when it works again.
func (s *Service) CheckToken(ctx context.Context, id uuid.UUID) (db.WhatsappAccount, error) {
	acc, token, err := s.accountToken(ctx, id)
	if err != nil {
		return acc, err
	}
	appID, appSecret := s.appCredentials(acc)
	rep, err := s.inspectToken(ctx, acc.WabaID, appID, appSecret, token)
	var problem *tokenProblem
	if errors.As(err, &problem) {
		s.FlagAccountProblem(ctx, id, problem.msg)
		s.attention(ctx, acc, problem.msg)
		return s.q.GetWhatsAppAccount(ctx, id)
	}
	if err != nil {
		return acc, metaFailure(err)
	}
	// The token must still reach the account's numbers.
	numbers, err := s.wa.ListPhoneNumbers(ctx, acc.WabaID, rep.token)
	if err != nil {
		s.flag(ctx, id, err)
		var api *whatsapp.APIError
		if errors.As(err, &api) {
			s.attention(ctx, acc, whatsapp.AccountProblemReason(api))
		}
		return s.q.GetWhatsAppAccount(ctx, id)
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		if rep.token != token {
			sealed, err := s.sealer.Seal([]byte(rep.token), tokenAD(id))
			if err != nil {
				return err
			}
			if _, err := q.UpdateWhatsAppAccount(ctx, db.UpdateWhatsAppAccountParams{ID: id, AccessTokenEnc: sealed}); err != nil {
				return err
			}
		}
		if err := upsertNumbers(ctx, q, id, numbers); err != nil {
			return err
		}
		if acc, err = s.saveTokenInfo(ctx, q, id, rep); err != nil {
			return err
		}
		return q.ReactivateWhatsAppAccount(ctx, id)
	})
	if err != nil {
		return acc, err
	}
	if exp := rep.info.ExpiresAt; exp != nil && time.Until(*exp) < ExpiryNotice {
		s.attention(ctx, acc, fmt.Sprintf("The access token of %q expires on %s; afterwards nothing can be sent. Replace it with a System User token under Channels.", acc.Name, exp.Format("2006-01-02 15:04 MST")))
	}
	return s.q.GetWhatsAppAccount(ctx, id)
}

// CheckTokens re-validates every active or flagged account.
func (s *Service) CheckTokens(ctx context.Context) error {
	accs, err := s.q.ListWhatsAppAccountsToCheck(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, a := range accs {
		if _, err := s.CheckToken(ctx, a.ID); err != nil {
			errs = append(errs, fmt.Errorf("account %s: %w", a.ID, err))
		}
	}
	return errors.Join(errs...)
}

// attention records and announces a problem, at most once a day per
// account.
func (s *Service) attention(ctx context.Context, acc db.WhatsappAccount, reason string) {
	if acc.TokenNotifiedAt != nil && time.Since(*acc.TokenNotifiedAt) < 24*time.Hour {
		return
	}
	if err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.MarkWhatsAppTokenNotified(ctx, acc.ID); err != nil {
			return err
		}
		id := acc.ID
		return audit.Record(ctx, q, audit.Entry{Action: "channel.attention", EntityType: "whatsapp_account", EntityID: &id,
			Changes: map[string]any{"reason": reason}})
	}); err != nil {
		s.log.Error("record channel attention", "account_id", acc.ID, "error", err)
		return
	}
	if s.notify != nil {
		s.notify(ctx, fmt.Sprintf("WhatsApp account %q needs attention", acc.Name),
			fmt.Sprintf("Hello,\n\nMessora's WhatsApp account %q (WABA %s) needs attention:\n\n%s\n\nOpen Channels in the panel to fix it.\n", acc.Name, acc.WabaID, reason))
	}
}

// AccountRefused handles Meta refusing an account's credentials during a
// send: the account is flagged with a readable reason and the owners are
// told.
func (s *Service) AccountRefused(ctx context.Context, id uuid.UUID, e *whatsapp.APIError) {
	reason := whatsapp.AccountProblemReason(e)
	s.FlagAccountProblem(ctx, id, reason)
	if acc, err := s.q.GetWhatsAppAccount(ctx, id); err == nil {
		s.attention(ctx, acc, reason)
	}
}

// sampleLimits are Meta's limits for template example files.
var sampleLimits = map[string]int64{
	"image/jpeg": 5 << 20, "image/png": 5 << 20,
	"video/mp4":       16 << 20,
	"application/pdf": 100 << 20,
}

// UploadTemplateSample sends the example file of a media-header template to
// Meta and returns its handle.
func (s *Service) UploadTemplateSample(ctx context.Context, accountID uuid.UUID, filename, mimeType string, body io.Reader) (string, string, int64, error) {
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	limit, ok := sampleLimits[mimeType]
	if !ok {
		return "", "", 0, apperr.Validation("content_type", "Template examples are JPEG or PNG images, MP4 videos or PDF documents.")
	}
	acc, token, err := s.accountToken(ctx, accountID)
	if err != nil {
		return "", "", 0, err
	}
	if acc.AppID == nil {
		return "", "", 0, apperr.Validation("account_id", "Add the App ID to this WhatsApp account (Channels → Edit) to create media templates.")
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return "", "", 0, err
	}
	if int64(len(data)) > limit {
		return "", "", 0, apperr.Validation("file", fmt.Sprintf("This file type is limited to %d MB.", limit>>20))
	}
	if len(data) == 0 {
		return "", "", 0, apperr.Validation("file", "The file is empty.")
	}
	name := strings.TrimSpace(filepath.Base(filename))
	if name == "" || name == "." {
		name = "example"
	}
	handle, err := s.wa.UploadTemplateSample(ctx, *acc.AppID, token, name, mimeType, data)
	if err != nil {
		s.flag(ctx, acc.ID, err)
		return "", "", 0, metaFailure(err)
	}
	return handle, mimeType, int64(len(data)), nil
}
