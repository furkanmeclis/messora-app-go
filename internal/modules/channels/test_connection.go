package channels

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// Check statuses of a connection test.
const (
	CheckOK      = "ok"
	CheckWarning = "warning"
	CheckError   = "error"
)

// ConnectionCheck is one line of a connection test.
type ConnectionCheck struct {
	Key    string // app_secret, token, phone_numbers, subscription, webhook
	Status string // ok, warning, error
	// Code names the outcome for the panel's translations; Params fill it.
	Code    string
	Params  map[string]string
	Message string // English, for API clients
}

// ConnectionTest is the result of "Test connection".
type ConnectionTest struct {
	Account       db.WhatsappAccount
	Checks        []ConnectionCheck
	PhoneNumbers  int
	Subscribed    bool
	SubscribedNow bool // Messora subscribed the app during this test
}

// OK reports whether no check failed.
func (t ConnectionTest) OK() bool {
	return !slices.ContainsFunc(t.Checks, func(c ConnectionCheck) bool { return c.Status == CheckError })
}

// TestConnection checks an account end to end with Meta: the token
// (debug_token with the account's app), its access to the WABA's numbers,
// the app's webhook subscription on the WABA (subscribing it when
// missing) and whether webhooks have arrived. What it learns about the
// token and numbers is saved, as with CheckToken.
func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) (ConnectionTest, error) {
	acc, token, err := s.accountToken(ctx, id)
	if err != nil {
		return ConnectionTest{}, err
	}
	out := ConnectionTest{Account: acc}
	add := func(c ConnectionCheck) { out.Checks = append(out.Checks, c) }
	check := func(key, status, code, msg string, params ...string) ConnectionCheck {
		c := ConnectionCheck{Key: key, Status: status, Code: code, Message: msg}
		if len(params) > 0 {
			c.Params = map[string]string{}
			for i := 0; i+1 < len(params); i += 2 {
				c.Params[params[i]] = params[i+1]
			}
		}
		return c
	}
	appID, appSecret := s.appCredentials(ctx, acc)
	switch {
	case appSecret == "":
		add(check("app_secret", CheckError, "app_secret_missing", "No App secret is stored for this account, so Meta's webhooks for it cannot be verified and are rejected. Add it with Replace."))
	case appID == "":
		add(check("app_secret", CheckWarning, "app_id_missing", "No App ID is stored; Messora cannot check the token with your app or create media templates. Add it with Replace."))
	default:
		add(check("app_secret", CheckOK, "app_ok", "App ID and App secret are stored (the secret is never shown)."))
	}

	rep, err := s.inspectToken(ctx, acc.WabaID, appID, appSecret, token)
	var problem *tokenProblem
	switch {
	case errors.As(err, &problem):
		add(check("token", CheckError, "token_invalid", problem.msg, "detail", problem.msg))
		s.FlagAccountProblem(ctx, id, problem.msg)
	case err != nil:
		return ConnectionTest{}, metaFailure(err)
	case !rep.checked:
		add(check("token", CheckWarning, "token_unchecked", rep.warning))
	case rep.info.AppID != "" && appID != "" && rep.info.AppID != appID:
		add(check("token", CheckWarning, "token_other_app", fmt.Sprintf("The access token belongs to app %s, not to App ID %s. Webhooks are signed by the app that is subscribed; use a System User token of the same app.", rep.info.AppID, appID), "token_app", rep.info.AppID, "app_id", appID))
	case rep.info.ExpiresAt != nil:
		add(check("token", CheckWarning, "token_expires", rep.warning, "expires_at", rep.info.ExpiresAt.UTC().Format(time.RFC3339)))
	default:
		msg := "The access token is valid and never expires"
		if rep.info.Type != "" {
			msg += " (" + rep.info.Type + ")"
		}
		add(check("token", CheckOK, "token_ok", msg+".", "type", rep.info.Type))
	}
	if problem != nil {
		out.Account, err = s.q.GetWhatsAppAccount(ctx, id)
		return out, err
	}

	numbers, err := s.wa.ListPhoneNumbers(ctx, acc.WabaID, rep.token)
	if err != nil {
		s.flag(ctx, id, err)
		add(check("phone_numbers", CheckError, "numbers_error", "Meta refused to list the account's phone numbers: "+metaFailure(err).Error(), "detail", metaFailure(err).Error()))
		out.Account, err = s.q.GetWhatsAppAccount(ctx, id)
		return out, err
	}
	out.PhoneNumbers = len(numbers)
	if len(numbers) == 0 {
		add(check("phone_numbers", CheckWarning, "numbers_none", "The WhatsApp Business Account has no phone numbers yet; add one in WhatsApp Manager."))
	} else {
		add(check("phone_numbers", CheckOK, "numbers_ok", fmt.Sprintf("%d phone number(s) found.", len(numbers)), "count", fmt.Sprint(len(numbers))))
	}

	out.Subscribed, err = s.appSubscribed(ctx, acc.WabaID, appID, rep.token)
	if err == nil && !out.Subscribed {
		if err = s.wa.SubscribeApp(ctx, acc.WabaID, rep.token); err == nil {
			out.Subscribed, err = s.appSubscribed(ctx, acc.WabaID, appID, rep.token)
			out.SubscribedNow = out.Subscribed
		}
	}
	switch {
	case err != nil:
		add(check("subscription", CheckError, "subscription_error", "Messora could not read or create the app's webhook subscription on this WhatsApp Business Account: "+metaFailure(err).Error(), "detail", metaFailure(err).Error()))
	case out.SubscribedNow:
		add(check("subscription", CheckOK, "subscribed_now", "Your app was not subscribed to this WhatsApp Business Account's webhooks; Messora subscribed it now."))
	case out.Subscribed:
		add(check("subscription", CheckOK, "subscribed", "Your app is subscribed to this WhatsApp Business Account's webhooks."))
	default:
		add(check("subscription", CheckError, "not_subscribed", "Your app is still not subscribed to this WhatsApp Business Account's webhooks."))
	}
	add(s.webhookCheck(ctx))

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
		if _, err := s.saveTokenInfo(ctx, q, id, rep); err != nil {
			return err
		}
		return q.ReactivateWhatsAppAccount(ctx, id)
	})
	if err != nil {
		return ConnectionTest{}, err
	}
	out.Account, err = s.q.GetWhatsAppAccount(ctx, id)
	return out, err
}

// appSubscribed reports whether appID (any app when unknown) receives the
// WABA's webhooks.
func (s *Service) appSubscribed(ctx context.Context, wabaID, appID, token string) (bool, error) {
	apps, err := s.wa.SubscribedApps(ctx, wabaID, token)
	if err != nil {
		return false, err
	}
	if appID == "" {
		return len(apps) > 0, nil
	}
	return slices.ContainsFunc(apps, func(a whatsapp.SubscribedApp) bool { return a.ID == appID }), nil
}

// webhookCheck reads the tenant's webhook health.
func (s *Service) webhookCheck(ctx context.Context) ConnectionCheck {
	row, err := s.q.GetMetaConnection(ctx)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.LastWebhookAt == nil) {
		return ConnectionCheck{Key: "webhook", Status: CheckWarning, Code: "webhook_none",
			Message: "No webhook from Meta has arrived yet. Enter the callback URL and verify token in your app (WhatsApp > Configuration) and subscribe the fields; Meta sends the first event with the next message or status."}
	}
	if err != nil {
		return ConnectionCheck{Key: "webhook", Status: CheckWarning, Code: "webhook_unknown", Message: "The webhook health could not be read."}
	}
	if f := row.LastSignatureFailureAt; f != nil && f.After(*row.LastWebhookAt) {
		at := f.UTC().Format(time.RFC3339)
		return ConnectionCheck{Key: "webhook", Status: CheckWarning, Code: "webhook_signature", Params: map[string]string{"at": at},
			Message: "Webhooks arrive, but the latest one (" + at + ") had a wrong signature: check that the App secret belongs to the subscribed app."}
	}
	at := row.LastWebhookAt.UTC().Format(time.RFC3339)
	return ConnectionCheck{Key: "webhook", Status: CheckOK, Code: "webhook_ok", Params: map[string]string{"at": at},
		Message: "The last webhook arrived at " + at + "."}
}
