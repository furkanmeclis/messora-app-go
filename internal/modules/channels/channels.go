// Package channels manages connected WhatsApp Business Accounts, their
// phone numbers and message templates. Access tokens and app secrets are
// sealed at rest and only decrypted for a Graph API call.
package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Service is the channels API.
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	sealer *crypto.Sealer
	wa     *whatsapp.Client
	log    *slog.Logger
	notify Notifier
	// plans caps the connected phone numbers (Phase 17 WP8).
	plans entitlements.Provider
}

// SetPlans installs the plan provider.
func (s *Service) SetPlans(p entitlements.Provider) { s.plans = p }

// checkNumbers refuses n more phone numbers past the plan's limit (numbers
// of disabled accounts do not count).
func (s *Service) checkNumbers(ctx context.Context, q *db.Queries, n int) error {
	return entitlements.CheckAdd(ctx, q, s.plans, entitlements.PhoneNumbers, int64(n), func() (int64, error) { return q.CountPhoneNumbersForPlan(ctx) })
}

// New returns the service. Every Meta credential is the tenant's own and
// lives in the database: there is no platform-wide app, token or secret.
func New(pool *pgxpool.Pool, sealer *crypto.Sealer, wa *whatsapp.Client, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), sealer: sealer, wa: wa, log: log}
}

// Client returns the Graph API client.
func (s *Service) Client() *whatsapp.Client { return s.wa }

var digits = regexp.MustCompile(`^[0-9]{1,32}$`)

func tokenAD(id uuid.UUID) string  { return "whatsapp_account.token:" + id.String() }
func secretAD(id uuid.UUID) string { return "whatsapp_account.app_secret:" + id.String() }

// AccountInput connects or updates an account.
type AccountInput struct {
	Name        string
	WABAID      string
	AppID       string
	AccessToken string
	AppSecret   string
}

// metaFailure turns a Graph API error during an admin action into a
// client-facing error without leaking the token.
func metaFailure(err error) error {
	if errors.Is(err, whatsapp.ErrDryRun) {
		return apperr.Unavailable("META_DRY_RUN", "Requests to Meta are disabled (META_DRY_RUN or APP_ENV=test).")
	}
	var api *whatsapp.APIError
	if errors.As(err, &api) {
		msg := api.Data.Details
		if msg == "" {
			msg = api.Message
		}
		return &apperr.Error{Status: http.StatusBadGateway, Code: "META_API_ERROR", Message: fmt.Sprintf("Meta rejected the request (%d): %s", api.Code, msg)}
	}
	return apperr.Unavailable("META_UNREACHABLE", "Meta's API could not be reached; try again.")
}

// Connect validates the credentials against Meta, then stores the account
// and its phone numbers.
func (s *Service) Connect(ctx context.Context, in AccountInput, actor *uuid.UUID) (db.WhatsappAccount, error) {
	in.Name, in.WABAID, in.AppID = strings.TrimSpace(in.Name), strings.TrimSpace(in.WABAID), strings.TrimSpace(in.AppID)
	in.AppSecret, in.AccessToken = strings.TrimSpace(in.AppSecret), strings.TrimSpace(in.AccessToken)
	switch {
	case in.Name == "":
		return db.WhatsappAccount{}, apperr.Validation("name", "Name is required.")
	case !digits.MatchString(in.WABAID):
		return db.WhatsappAccount{}, apperr.Validation("waba_id", "WhatsApp Business Account ID must be numeric.")
	case !digits.MatchString(in.AppID):
		return db.WhatsappAccount{}, apperr.Validation("app_id", "App ID must be numeric.")
	case len(strings.TrimSpace(in.AppSecret)) < 8:
		// The app secret signs the webhooks of this account; without it
		// nothing Meta sends could be accepted.
		return db.WhatsappAccount{}, apperr.Validation("app_secret", "The App secret is required (Meta app → App settings → Basic).")
	case len(in.AccessToken) < 20:
		return db.WhatsappAccount{}, apperr.Validation("access_token", "Access token looks too short.")
	}
	// A WABA has one account and one connection method. A Tech Provider
	// account that was disconnected from Messora's app switches back to
	// the company's own app here, with its history.
	var convert *db.WhatsappAccount
	if existing, err := s.q.GetWhatsAppAccountByWABA(ctx, in.WABAID); err == nil {
		if existing.ConnectionMethod != MethodTechProvider {
			return db.WhatsappAccount{}, apperr.Conflict("ACCOUNT_EXISTS", "This WhatsApp Business Account is already connected.")
		}
		routed, err := s.routed(ctx, in.WABAID)
		if err != nil {
			return db.WhatsappAccount{}, err
		}
		if routed || existing.Status != "disabled" {
			return db.WhatsappAccount{}, apperr.Conflict("WABA_CONNECTED_TECH_PROVIDER",
				"This WhatsApp Business Account is connected through Messora's Meta app. Disconnect it there first.")
		}
		convert = &existing
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.WhatsappAccount{}, err
	}
	rep, err := s.inspectToken(ctx, in.WABAID, in.AppID, in.AppSecret, in.AccessToken)
	if err != nil {
		return db.WhatsappAccount{}, asValidation(err)
	}
	in.AccessToken = rep.token
	numbers, err := s.wa.ListPhoneNumbers(ctx, in.WABAID, in.AccessToken)
	if err != nil {
		return db.WhatsappAccount{}, metaFailure(err)
	}
	id := uuid.Must(uuid.NewV7())
	if convert != nil {
		id = convert.ID
	}
	tok, err := s.sealer.Seal([]byte(in.AccessToken), tokenAD(id))
	if err != nil {
		return db.WhatsappAccount{}, err
	}
	sec, err := s.sealer.Seal([]byte(in.AppSecret), secretAD(id))
	if err != nil {
		return db.WhatsappAccount{}, err
	}
	var acc db.WhatsappAccount
	err = s.tx(ctx, func(q *db.Queries) error {
		if err := s.checkNumbers(ctx, q, len(numbers)); err != nil {
			return err
		}
		if convert != nil {
			acc, err = q.ConvertAccountToOwnApp(ctx, db.ConvertAccountToOwnAppParams{ID: id, Name: in.Name, AppID: nonEmpty(in.AppID),
				AccessTokenEnc: tok, AppSecretEnc: sec})
		} else {
			acc, err = q.InsertWhatsAppAccount(ctx, db.InsertWhatsAppAccountParams{ID: id, Name: in.Name, WabaID: in.WABAID,
				AppID: nonEmpty(in.AppID), AccessTokenEnc: tok, AppSecretEnc: sec, CreatedBy: actor})
		}
		if err != nil {
			return err
		}
		if err := s.afterTechProviderRemoved(ctx, q); err != nil {
			return err
		}
		if err := upsertNumbers(ctx, q, id, numbers); err != nil {
			return err
		}
		if acc, err = s.saveTokenInfo(ctx, q, id, rep); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.connect", EntityType: "whatsapp_account", EntityID: &id,
			Changes: map[string]any{"name": in.Name, "waba_id": in.WABAID, "phone_numbers": len(numbers), "method": MethodOwnApp,
				"switched_from_tech_provider": convert != nil}})
	})
	if isUnique(err) {
		return db.WhatsappAccount{}, apperr.Conflict("ACCOUNT_EXISTS", "This WhatsApp Business Account or one of its numbers is already connected.")
	}
	if err == nil {
		// The workspace's webhook address and health exist from the first
		// account on, so the webhooks it is about to receive are recorded.
		if _, cerr := s.Connection(ctx); cerr != nil {
			s.log.WarnContext(ctx, "create the Meta connection", "error", cerr)
		}
		// Import templates in the background; the request does not wait
		// for (or cancel) it.
		go s.syncTemplatesDetached(context.WithoutCancel(ctx), acc.ID)
	}
	return acc, err
}

func (s *Service) syncTemplatesDetached(parent context.Context, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	if _, err := s.SyncTemplates(ctx, id); err != nil {
		s.log.Warn("initial template sync failed", "account_id", id, "error", err)
	}
	if err := s.SyncSignals(ctx, &id); err != nil {
		s.log.Warn("initial signal sync failed", "account_id", id, "error", err)
	}
}

// throughputMPS is the number's throughput in messages per second, nil
// when Meta did not report a known level.
func throughputMPS(n whatsapp.PhoneNumber) *int32 {
	if mps := n.ThroughputMPS(); mps > 0 {
		v := int32(mps) //nolint:gosec // 80 or 1000
		return &v
	}
	return nil
}

func upsertNumbers(ctx context.Context, q *db.Queries, account uuid.UUID, numbers []whatsapp.PhoneNumber) error {
	for _, n := range numbers {
		if !digits.MatchString(n.ID) {
			continue
		}
		quality := n.QualityRating
		if quality == "" {
			quality = "UNKNOWN"
		}
		_, err := q.UpsertPhoneNumber(ctx, db.UpsertPhoneNumberParams{AccountID: account, PhoneNumberID: n.ID,
			DisplayPhoneNumber: n.DisplayPhoneNumber, VerifiedName: n.VerifiedName, QualityRating: quality,
			MessagingLimit: n.MessagingLimit, ThroughputLevel: n.Throughput.Level, ThroughputMps: throughputMPS(n),
			CodeVerificationStatus: n.CodeVerificationStatus, MetaStatus: n.Status})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("NUMBER_OWNED_ELSEWHERE", fmt.Sprintf("Phone number %s belongs to another connected account.", n.DisplayPhoneNumber))
		}
		if err != nil {
			return err
		}
	}
	// Make sure there is a default sender.
	if _, err := q.GetDefaultPhoneNumber(ctx); err == nil {
		var any bool
		nums, err := q.ListPhoneNumbers(ctx)
		if err != nil {
			return err
		}
		for _, n := range nums {
			any = any || n.IsDefault
		}
		if !any && len(nums) > 0 {
			_, err = q.SetDefaultPhoneNumber(ctx, nums[0].ID)
			return err
		}
	}
	return nil
}

// UpdateInput is a partial change to an account; nil fields stay as they are.
type UpdateInput struct {
	Name, AccessToken, AppSecret, AppID, Status *string
}

// Update changes the name or rotates credentials. A new token is checked
// with Meta (validity, permissions, expiry) and, when it works, brings an
// account that Meta had refused back into service.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (db.WhatsappAccount, error) {
	acc, err := s.q.GetWhatsAppAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return acc, apperr.NotFound("WhatsApp account not found.")
	}
	if err != nil {
		return acc, err
	}
	if acc.ConnectionMethod == MethodTechProvider {
		if in.AccessToken != nil || in.AppSecret != nil || in.AppID != nil {
			return acc, apperr.Conflict("MANAGED_BY_MESSORA", "This account is connected through Messora's Meta app; its credentials are managed by Messora.")
		}
		if in.Status != nil && *in.Status == "active" {
			routed, err := s.routed(ctx, acc.WabaID)
			if err != nil {
				return acc, err
			}
			if !routed {
				return acc, apperr.Conflict("TECH_PROVIDER_DISCONNECTED", "This account was disconnected from Messora's Meta app; connect it again with Meta.")
			}
		}
	}
	p := db.UpdateWhatsAppAccountParams{ID: id, Status: in.Status}
	changes := map[string]any{}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return acc, apperr.Validation("name", "Name is required.")
		}
		p.Name, changes["name"] = &n, n
	}
	appID, appSecret := s.appCredentials(ctx, acc)
	if in.AppID != nil {
		a := strings.TrimSpace(*in.AppID)
		if !digits.MatchString(a) {
			return acc, apperr.Validation("app_id", "App ID must be numeric.")
		}
		p.AppID, appID, changes["app_id"] = &a, a, a
	}
	if in.AppSecret != nil {
		if p.AppSecretEnc, err = s.sealer.Seal([]byte(*in.AppSecret), secretAD(id)); err != nil {
			return acc, err
		}
		appSecret, changes["app_secret"] = *in.AppSecret, "rotated"
	}
	var rep *tokenReport
	var numbers []whatsapp.PhoneNumber
	if in.AccessToken != nil {
		tok := strings.TrimSpace(*in.AccessToken)
		if len(tok) < 20 {
			return acc, apperr.Validation("access_token", "Access token looks too short.")
		}
		r, err := s.inspectToken(ctx, acc.WabaID, appID, appSecret, tok)
		if err != nil {
			return acc, asValidation(err)
		}
		if numbers, err = s.wa.ListPhoneNumbers(ctx, acc.WabaID, r.token); err != nil {
			return acc, metaFailure(err)
		}
		if p.AccessTokenEnc, err = s.sealer.Seal([]byte(r.token), tokenAD(id)); err != nil {
			return acc, err
		}
		rep, changes["access_token"] = &r, "rotated"
	}
	if in.Status != nil {
		if *in.Status != "active" && *in.Status != "disabled" {
			return acc, apperr.Validation("status", "Status must be active or disabled.")
		}
		changes["status"] = *in.Status
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		if acc, err = q.UpdateWhatsAppAccount(ctx, p); err != nil {
			return err
		}
		if rep != nil {
			if err := upsertNumbers(ctx, q, id, numbers); err != nil {
				return err
			}
			if acc, err = s.saveTokenInfo(ctx, q, id, *rep); err != nil {
				return err
			}
			// The new token works: lift the "Meta refused" flag at once
			// instead of failing every send until the next sync.
			if err := q.ReactivateWhatsAppAccount(ctx, id); err != nil {
				return err
			}
			if acc, err = q.GetWhatsAppAccount(ctx, id); err != nil {
				return err
			}
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.update", EntityType: "whatsapp_account", EntityID: &id, Changes: changes})
	})
	return acc, err
}

// Disconnect removes an account. Accounts with message history cannot be
// removed (disable them instead) so conversations stay intact.
func (s *Service) Disconnect(ctx context.Context, id uuid.UUID) error {
	var tp *db.WhatsappAccount
	err := s.rawTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		n, err := q.CountMessagesForAccount(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return apperr.Conflict("ACCOUNT_HAS_MESSAGES", "This account has message history; disable it instead.")
		}
		if c, err := q.CountCampaignsForAccount(ctx, id); err != nil {
			return err
		} else if c > 0 {
			return apperr.Conflict("ACCOUNT_HAS_CAMPAIGNS", "Campaigns use this account's numbers; delete them or disable the account instead.")
		}
		acc, err := q.GetWhatsAppAccount(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("WhatsApp account not found.")
		}
		if err != nil {
			return err
		}
		if acc.ConnectionMethod == MethodTechProvider {
			// Its events stop being accepted with the account.
			if _, err := tx.Exec(ctx, `SELECT meta_route_release($1)`, acc.WabaID); err != nil {
				return err
			}
			tp = &acc
		}
		rows, err := q.DeleteWhatsAppAccount(ctx, id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return apperr.NotFound("WhatsApp account not found.")
		}
		if tp != nil {
			if err := s.afterTechProviderRemoved(ctx, q); err != nil {
				return err
			}
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.disconnect", EntityType: "whatsapp_account", EntityID: &id,
			Changes: map[string]any{"method": acc.ConnectionMethod}})
	})
	if err == nil && tp != nil {
		if cfg, cerr := s.TechProvider(ctx); cerr == nil && cfg.AppID != "" {
			_ = s.unsubscribeTP(ctx, cfg, *tp)
		}
	}
	return err
}

// routed reports whether waba has a Tech Provider route (any tenant).
func (s *Service) routed(ctx context.Context, waba string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM meta_routes WHERE waba_id = $1)`, waba).Scan(&ok)
	return ok, err
}

// Sync refreshes phone numbers and templates from Meta.
func (s *Service) Sync(ctx context.Context, id uuid.UUID) (numbers, templates int, err error) {
	acc, token, err := s.accountToken(ctx, id)
	if err != nil {
		return 0, 0, err
	}
	list, err := s.wa.ListPhoneNumbers(ctx, acc.WabaID, token)
	if err != nil {
		s.flag(ctx, acc.ID, err)
		return 0, 0, metaFailure(err)
	}
	if err := s.tx(ctx, func(q *db.Queries) error { return upsertNumbers(ctx, q, acc.ID, list) }); err != nil {
		return 0, 0, err
	}
	if err := s.syncCurrency(ctx, acc, token); err != nil {
		s.log.WarnContext(ctx, "account currency not read", "account_id", acc.ID, "error", err)
	}
	templates, err = s.SyncTemplates(ctx, id)
	return len(list), templates, err
}

// SyncTemplates mirrors the account's templates; ones deleted on Meta's side
// are removed locally.
func (s *Service) SyncTemplates(ctx context.Context, id uuid.UUID) (int, error) {
	acc, token, err := s.accountToken(ctx, id)
	if err != nil {
		return 0, err
	}
	list, err := s.wa.ListTemplates(ctx, acc.WabaID, token)
	if err != nil {
		s.flag(ctx, acc.ID, err)
		return 0, metaFailure(err)
	}
	// Rows not touched by this sync (synced_at older than its start) were
	// deleted on Meta's side.
	var since time.Time
	if err := s.pool.QueryRow(ctx, "SELECT now()").Scan(&since); err != nil {
		return 0, err
	}
	err = s.tx(ctx, func(q *db.Queries) error {
		for _, t := range list {
			if err := upsertTemplate(ctx, q, acc.ID, t); err != nil {
				return err
			}
		}
		if _, err := q.DeleteTemplatesNotSynced(ctx, db.DeleteTemplatesNotSyncedParams{AccountID: acc.ID, Since: since}); err != nil {
			return err
		}
		return q.MarkWhatsAppAccountSynced(ctx, acc.ID)
	})
	return len(list), err
}

func upsertTemplate(ctx context.Context, q *db.Queries, account uuid.UUID, t whatsapp.TemplateInfo) error {
	comps := []byte(t.Components)
	if len(comps) == 0 || !json.Valid(comps) || string(comps) == "null" {
		comps = []byte("[]")
	}
	pf := t.ParameterFormat
	if pf == "" {
		pf = "POSITIONAL"
	}
	var quality string
	if len(t.QualityScore) > 0 {
		var qs struct {
			Score string `json:"score"`
		}
		_ = json.Unmarshal(t.QualityScore, &qs)
		quality = qs.Score
	}
	_, err := q.UpsertTemplate(ctx, db.UpsertTemplateParams{AccountID: account, MetaTemplateID: t.ID, Name: t.Name,
		Language: t.Language, Category: t.Category, Status: t.Status, ParameterFormat: pf, Components: comps,
		RejectedReason: t.RejectedReason, QualityScore: quality})
	return err
}

// TemplateInput creates a template on Meta.
type TemplateInput struct {
	AccountID       uuid.UUID
	Name            string
	Language        string
	Category        string
	ParameterFormat string
	Components      json.RawMessage
}

var templateName = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)

// CreateTemplate submits a template for Meta review and stores it as PENDING.
func (s *Service) CreateTemplate(ctx context.Context, in TemplateInput) (db.MessageTemplate, error) {
	switch {
	case !templateName.MatchString(in.Name):
		return db.MessageTemplate{}, apperr.Validation("name", "Use lowercase letters, digits and underscores.")
	case in.Category != "MARKETING" && in.Category != "UTILITY" && in.Category != "AUTHENTICATION":
		return db.MessageTemplate{}, apperr.Validation("category", "Category must be MARKETING, UTILITY or AUTHENTICATION.")
	case len(in.Language) < 2 || len(in.Language) > 15:
		return db.MessageTemplate{}, apperr.Validation("language", "Language code is required, e.g. en_US.")
	}
	if in.ParameterFormat == "" {
		in.ParameterFormat = "POSITIONAL"
	}
	if err := ValidateComponents(in.Category, in.ParameterFormat, in.Components); err != nil {
		return db.MessageTemplate{}, err
	}
	acc, token, err := s.accountToken(ctx, in.AccountID)
	if err != nil {
		return db.MessageTemplate{}, err
	}
	body, _ := json.Marshal(map[string]any{"name": in.Name, "language": in.Language, "category": in.Category,
		"parameter_format": in.ParameterFormat, "components": in.Components})
	metaID, status, err := s.wa.CreateTemplate(ctx, acc.WabaID, token, body)
	if err != nil {
		s.flag(ctx, acc.ID, err)
		return db.MessageTemplate{}, metaFailure(err)
	}
	if status == "" {
		status = "PENDING"
	}
	var out db.MessageTemplate
	err = s.tx(ctx, func(q *db.Queries) error {
		out, err = q.UpsertTemplate(ctx, db.UpsertTemplateParams{AccountID: acc.ID, MetaTemplateID: metaID, Name: in.Name,
			Language: in.Language, Category: in.Category, Status: status, ParameterFormat: in.ParameterFormat,
			Components: in.Components})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "template.create", EntityType: "message_template", EntityID: &out.ID,
			Changes: map[string]any{"name": in.Name, "language": in.Language, "category": in.Category}})
	})
	return out, err
}

// DeleteTemplate deletes one language version on Meta and locally.
func (s *Service) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	t, err := s.q.GetTemplate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("Template not found.")
	}
	if err != nil {
		return err
	}
	acc, token, err := s.accountToken(ctx, t.AccountID)
	if err != nil {
		return err
	}
	if err := s.wa.DeleteTemplate(ctx, acc.WabaID, token, t.Name, t.MetaTemplateID); err != nil {
		var api *whatsapp.APIError
		// Already gone on Meta's side: just drop the local copy.
		if !errors.As(err, &api) || api.Code != 100 {
			s.flag(ctx, acc.ID, err)
			return metaFailure(err)
		}
	}
	return s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.DeleteTemplate(ctx, id); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "template.delete", EntityType: "message_template", EntityID: &id,
			Changes: map[string]any{"name": t.Name, "language": t.Language}})
	})
}

// SetDefaultNumber picks the sender used when a request names none.
func (s *Service) SetDefaultNumber(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		if err := q.ClearDefaultPhoneNumber(ctx); err != nil {
			return err
		}
		n, err := q.SetDefaultPhoneNumber(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound("Phone number not found.")
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.default_number", EntityType: "phone_number", EntityID: &id})
	})
}

// Sender resolves what is needed to send from a phone number.
type Sender struct {
	PhoneNumber db.PhoneNumber
	Account     db.WhatsappAccount
	Token       string
}

// SenderFor returns the credentials to send from phone number id.
func (s *Service) SenderFor(ctx context.Context, id uuid.UUID) (Sender, error) {
	pn, err := s.q.GetPhoneNumber(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Sender{}, apperr.NotFound("Phone number not found.")
	}
	if err != nil {
		return Sender{}, err
	}
	acc, token, err := s.accountToken(ctx, pn.AccountID)
	return Sender{PhoneNumber: pn, Account: acc, Token: token}, err
}

// AppSecrets returns the app secrets the context's tenant's webhooks may be
// signed with: those of its active accounts (row-level security keeps
// every other tenant's out).
func (s *Service) AppSecrets(ctx context.Context) ([]string, error) {
	rows, err := s.q.ListAppSecrets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		b, err := s.sealer.Open(r.AppSecretEnc, secretAD(r.ID))
		if err != nil {
			s.log.Error("cannot decrypt app secret", "account_id", r.ID, "error", err)
			continue
		}
		out = append(out, string(b))
	}
	return out, nil
}

// FlagAccountProblem marks an account unusable after Meta refused its
// credentials, so operators see it in the panel.
func (s *Service) FlagAccountProblem(ctx context.Context, id uuid.UUID, reason string) {
	if err := s.q.MarkWhatsAppAccountError(ctx, db.MarkWhatsAppAccountErrorParams{ID: id, LastError: &reason}); err != nil {
		s.log.Error("flag account", "account_id", id, "error", err)
	}
}

func (s *Service) flag(ctx context.Context, id uuid.UUID, err error) {
	var api *whatsapp.APIError
	if errors.As(err, &api) {
		if se := whatsapp.Classify(api, 0); se.AccountProblem {
			s.FlagAccountProblem(ctx, id, whatsapp.AccountProblemReason(api))
		}
	}
}

func (s *Service) accountToken(ctx context.Context, id uuid.UUID) (db.WhatsappAccount, string, error) {
	acc, err := s.q.GetWhatsAppAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return acc, "", apperr.NotFound("WhatsApp account not found.")
	}
	if err != nil {
		return acc, "", err
	}
	if acc.Status == "disabled" {
		return acc, "", apperr.Conflict("ACCOUNT_DISABLED", "This WhatsApp account is disabled.")
	}
	tok, err := s.sealer.Open(acc.AccessTokenEnc, tokenAD(acc.ID))
	if err != nil {
		return acc, "", fmt.Errorf("decrypt access token: %w", err)
	}
	return acc, string(tok), nil
}

func (s *Service) tx(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
