package channels

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Embedded Signup, the company side (docs/PHASE-17.md §5.9). Meta's popup
// ends with a code and the ids of the WABA and phone number the company
// chose; the server turns them into a connected account in five steps,
// each persisted so a failure can be retried from where it stopped:
//
//	exchange  code → business token; the token must grant this WABA, the
//	          number must belong to it (the ids come from the browser and
//	          are not trusted)
//	claim     route the WABA to the tenant (meta_route_claim) and store the
//	          account, disabled; a WABA of another tenant or of the
//	          tenant's own app is refused here, before anything changes at
//	          Meta
//	subscribe Messora's app receives the WABA's webhooks
//	register  the number is registered for Cloud API with its PIN
//	finish    numbers, token health, account active, method tech_provider

// Onboarding steps and states (meta_onboardings).
const (
	StepExchange  = "exchange"
	StepClaim     = "claim"
	StepSubscribe = "subscribe"
	StepRegister  = "register"
	StepFinish    = "finish"
	StepDone      = "done"

	OnboardingRunning   = "running"
	OnboardingFailed    = "failed"
	OnboardingCompleted = "completed"
	OnboardingCancelled = "cancelled"
)

// onboardingTimeout bounds one run of the steps; it does not end with the
// request, so a closed browser tab cannot leave a step half done.
const onboardingTimeout = 55 * time.Second // under the API server's 60 s write timeout

// OnboardingInput is what the panel posts after Meta's popup finished.
type OnboardingInput struct {
	Code          string
	WABAID        string
	PhoneNumberID string
	BusinessID    string
	Name          string
	PIN           string // 6 digits; generated when empty
}

// Onboarding is an Embedded Signup with what the panel may show.
type Onboarding struct {
	Row db.MetaOnboarding
	// PIN is the number's two-step verification PIN, returned once by the
	// request that completed an onboarding whose PIN Messora generated.
	PIN string
}

// stepError is a step's failure: a code the panel translates and Meta's
// (or Messora's) English explanation.
type stepError struct{ code, msg string }

func (e *stepError) Error() string { return e.code + ": " + e.msg }

func failStep(code, msg string) *stepError { return &stepError{code: code, msg: msg} }

// metaStepError explains a Graph error of a step.
func metaStepError(code string, err error) *stepError {
	var api *whatsapp.APIError
	if errors.As(err, &api) {
		msg := api.Data.Details
		if msg == "" {
			msg = api.Message
		}
		return failStep(code, fmt.Sprintf("Meta refused the request (%d): %s", api.Code, msg))
	}
	if errors.Is(err, whatsapp.ErrDryRun) {
		return failStep(code, "Requests to Meta are disabled (META_DRY_RUN or APP_ENV=test).")
	}
	return failStep(code, "Meta's API could not be reached; try again.")
}

func onboardingTokenAD(id uuid.UUID) string { return "meta_onboarding.business_token:" + id.String() }
func onboardingPinAD(id uuid.UUID) string   { return "meta_onboarding.pin:" + id.String() }

// newPIN returns a random 6-digit PIN.
func newPIN() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func validPIN(p string) bool { return len(p) == 6 && digits.MatchString(p) }

// Onboardings lists the tenant's unfinished and recent onboardings.
func (s *Service) Onboardings(ctx context.Context) ([]db.MetaOnboarding, error) {
	if _, err := s.enabledTechProvider(ctx); err != nil {
		return nil, err
	}
	return s.q.ListMetaOnboardings(ctx)
}

// StartOnboarding runs a new Embedded Signup (or continues the unfinished
// one of the same WABA with the new code).
func (s *Service) StartOnboarding(ctx context.Context, in OnboardingInput, actor *uuid.UUID) (Onboarding, error) {
	cfg, err := s.enabledTechProvider(ctx)
	if err != nil {
		return Onboarding{}, err
	}
	in.Code, in.WABAID, in.PhoneNumberID = strings.TrimSpace(in.Code), strings.TrimSpace(in.WABAID), strings.TrimSpace(in.PhoneNumberID)
	in.BusinessID, in.Name, in.PIN = strings.TrimSpace(in.BusinessID), strings.TrimSpace(in.Name), strings.TrimSpace(in.PIN)
	switch {
	case in.Code == "" || len(in.Code) > 2048:
		return Onboarding{}, apperr.Validation("code", "The code from Meta's popup is missing.")
	case !digits.MatchString(in.WABAID):
		return Onboarding{}, apperr.Validation("waba_id", "WhatsApp Business Account ID must be numeric.")
	case !digits.MatchString(in.PhoneNumberID):
		return Onboarding{}, apperr.Validation("phone_number_id", "Phone number ID must be numeric.")
	case in.BusinessID != "" && !digits.MatchString(in.BusinessID):
		return Onboarding{}, apperr.Validation("business_id", "Business ID must be numeric.")
	case in.PIN != "" && !validPIN(in.PIN):
		return Onboarding{}, apperr.Validation("pin", "The PIN is 6 digits.")
	case len(in.Name) > 120:
		return Onboarding{}, apperr.Validation("name", "Name is at most 120 characters.")
	}
	if open, err := s.q.GetOpenMetaOnboardingByWABA(ctx, in.WABAID); err == nil {
		if open.PhoneNumberID != in.PhoneNumberID {
			// Another number of the same WABA: the unfinished one ends.
			if _, err := s.CancelOnboarding(ctx, open.ID); err != nil {
				return Onboarding{}, err
			}
		} else {
			return s.RetryOnboarding(ctx, open.ID, in.Code, in.PIN)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Onboarding{}, err
	}
	generated := in.PIN == ""
	if generated {
		if in.PIN, err = newPIN(); err != nil {
			return Onboarding{}, err
		}
	}
	id := uuid.Must(uuid.NewV7())
	pin, err := s.sealer.Seal([]byte(in.PIN), onboardingPinAD(id))
	if err != nil {
		return Onboarding{}, err
	}
	var row db.MetaOnboarding
	err = s.tx(ctx, func(q *db.Queries) error {
		row, err = q.InsertMetaOnboarding(ctx, db.InsertMetaOnboardingParams{ID: id, WabaID: in.WABAID, PhoneNumberID: in.PhoneNumberID,
			BusinessID: nonEmpty(in.BusinessID), Name: in.Name, PinEnc: pin, PinGenerated: generated, CreatedBy: actor})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.tech_provider.start", EntityType: "meta_onboarding", EntityID: &id,
			Changes: map[string]any{"waba_id": in.WABAID, "phone_number_id": in.PhoneNumberID, "pin_generated": generated}})
	})
	if isUnique(err) {
		return Onboarding{}, apperr.Conflict("ONBOARDING_RUNNING", "This WhatsApp Business Account is being connected already.")
	}
	if err != nil {
		return Onboarding{}, err
	}
	return s.runOnboarding(ctx, cfg, row, in.Code)
}

// RetryOnboarding resumes a failed onboarding at its failed step. A new
// code replaces the business token (the exchange runs again); a new PIN
// replaces the one the registration refused.
func (s *Service) RetryOnboarding(ctx context.Context, id uuid.UUID, code, pin string) (Onboarding, error) {
	cfg, err := s.enabledTechProvider(ctx)
	if err != nil {
		return Onboarding{}, err
	}
	code, pin = strings.TrimSpace(code), strings.TrimSpace(pin)
	if pin != "" && !validPIN(pin) {
		return Onboarding{}, apperr.Validation("pin", "The PIN is 6 digits.")
	}
	if len(code) > 2048 {
		return Onboarding{}, apperr.Validation("code", "The code is too long.")
	}
	row, err := s.q.ClaimMetaOnboarding(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		cur, gerr := s.q.GetMetaOnboarding(ctx, id)
		switch {
		case errors.Is(gerr, pgx.ErrNoRows):
			return Onboarding{}, apperr.NotFound("Onboarding not found.")
		case gerr != nil:
			return Onboarding{}, gerr
		case cur.Status == OnboardingRunning:
			return Onboarding{}, apperr.Conflict("ONBOARDING_RUNNING", "This onboarding is running; wait for it to finish.")
		default:
			return Onboarding{}, apperr.Conflict("ONBOARDING_FINISHED", "This onboarding has finished; start a new one.")
		}
	}
	if err != nil {
		return Onboarding{}, err
	}
	p := db.SetMetaOnboardingInputParams{ID: id, ResetToken: code != "" && slices.Contains([]string{StepExchange, StepClaim}, row.Step)}
	if pin != "" {
		if p.PinEnc, err = s.sealer.Seal([]byte(pin), onboardingPinAD(id)); err != nil {
			return Onboarding{}, err
		}
	}
	if p.ResetToken || p.PinEnc != nil {
		if row, err = s.q.SetMetaOnboardingInput(ctx, p); err != nil {
			return Onboarding{}, err
		}
	}
	return s.runOnboarding(ctx, cfg, row, code)
}

// CancelOnboarding ends an unfinished onboarding: Messora's app leaves the
// WABA, its route is removed and an account it created without history is
// deleted (an account with history stays, disabled).
func (s *Service) CancelOnboarding(ctx context.Context, id uuid.UUID) (Onboarding, error) {
	if _, err := s.enabledTechProvider(ctx); err != nil {
		return Onboarding{}, err
	}
	cur, err := s.q.GetMetaOnboarding(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Onboarding{}, apperr.NotFound("Onboarding not found.")
	}
	if err != nil {
		return Onboarding{}, err
	}
	if cur.Status != OnboardingRunning && cur.Status != OnboardingFailed {
		return Onboarding{}, apperr.Conflict("ONBOARDING_FINISHED", "This onboarding has finished.")
	}
	if cur.Step == StepRegister || cur.Step == StepFinish {
		if tok, err := s.onboardingToken(cur); err == nil && tok != "" {
			cfg, _ := s.TechProvider(ctx)
			if err := s.tpClient(cfg).UnsubscribeApp(ctx, cur.WabaID, tok); err != nil {
				s.log.WarnContext(ctx, "cancel onboarding: unsubscribe the app", "waba_id", cur.WabaID, "error", err)
			}
		}
	}
	var row db.MetaOnboarding
	err = s.rawTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if row, err = q.CancelMetaOnboarding(ctx, id); err != nil {
			return err
		}
		if cur.AccountID != nil {
			acc, err := q.GetWhatsAppAccount(ctx, *cur.AccountID)
			if err == nil && acc.ConnectionMethod == MethodTechProvider && acc.Status == "disabled" {
				if _, err := tx.Exec(ctx, `SELECT meta_route_release($1)`, cur.WabaID); err != nil {
					return err
				}
				if err := deleteIfNoHistory(ctx, q, acc.ID); err != nil {
					return err
				}
			}
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.tech_provider.cancel", EntityType: "meta_onboarding", EntityID: &id,
			Changes: map[string]any{"waba_id": cur.WabaID, "step": cur.Step}})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Onboarding{}, apperr.Conflict("ONBOARDING_FINISHED", "This onboarding has finished.")
	}
	if err != nil {
		return Onboarding{}, err
	}
	return Onboarding{Row: row}, nil
}

func deleteIfNoHistory(ctx context.Context, q *db.Queries, id uuid.UUID) error {
	n, err := q.CountMessagesForAccount(ctx, id)
	if err != nil || n > 0 {
		return err
	}
	if c, err := q.CountCampaignsForAccount(ctx, id); err != nil || c > 0 {
		return err
	}
	_, err = q.DeleteWhatsAppAccount(ctx, id)
	return err
}

func (s *Service) onboardingToken(row db.MetaOnboarding) (string, error) {
	if len(row.BusinessTokenEnc) == 0 {
		return "", nil
	}
	b, err := s.sealer.Open(row.BusinessTokenEnc, onboardingTokenAD(row.ID))
	return string(b), err
}

// runOnboarding runs the steps from row.Step on. A failing step is stored
// (status failed, its code and message) and the onboarding returned; only
// failures of Messora itself return an error.
func (s *Service) runOnboarding(parent context.Context, cfg TechProviderConfig, row db.MetaOnboarding, code string) (Onboarding, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), onboardingTimeout)
	defer cancel()
	pinB, err := s.sealer.Open(row.PinEnc, onboardingPinAD(row.ID))
	if err != nil {
		return Onboarding{}, fmt.Errorf("decrypt onboarding pin: %w", err)
	}
	pin := string(pinB)
	for row.Step != StepDone {
		var next db.MetaOnboarding
		var serr error
		switch row.Step {
		case StepExchange:
			next, serr = s.stepExchange(ctx, cfg, row, code)
		case StepClaim:
			next, serr = s.stepClaim(ctx, cfg, row)
		case StepSubscribe:
			next, serr = s.stepSubscribe(ctx, cfg, row)
		case StepRegister:
			next, serr = s.stepRegister(ctx, cfg, row, pin)
		case StepFinish:
			next, serr = s.stepFinish(ctx, cfg, row)
		default:
			serr = fmt.Errorf("unknown onboarding step %q", row.Step)
		}
		if serr != nil {
			return s.failOnboarding(ctx, row, serr)
		}
		row = next
	}
	out := Onboarding{Row: row}
	if row.PinGenerated {
		out.PIN = pin
	}
	return out, nil
}

// failOnboarding stores a step's failure and returns the onboarding; an
// unexpected error is stored as "internal" and returned too.
func (s *Service) failOnboarding(ctx context.Context, row db.MetaOnboarding, err error) (Onboarding, error) {
	var se *stepError
	code, msg, internal := "internal", "Messora could not finish this step; try again.", err
	if errors.As(err, &se) {
		code, msg, internal = se.code, se.msg, nil
	}
	if len(msg) > 1000 {
		msg = msg[:1000]
	}
	s.log.WarnContext(ctx, "embedded signup step failed", "onboarding_id", row.ID, "step", row.Step, "code", code, "error", err)
	var failed db.MetaOnboarding
	ferr := s.tx(ctx, func(q *db.Queries) error {
		var e error
		if failed, e = q.FailMetaOnboarding(ctx, db.FailMetaOnboardingParams{ID: row.ID, ErrorCode: &code, ErrorMessage: &msg}); e != nil {
			return e
		}
		id := row.ID
		return audit.Record(ctx, q, audit.Entry{Action: "channel.tech_provider.fail", EntityType: "meta_onboarding", EntityID: &id,
			Changes: map[string]any{"waba_id": row.WabaID, "step": row.Step, "code": code}})
	})
	if ferr != nil {
		return Onboarding{}, errors.Join(err, ferr)
	}
	return Onboarding{Row: failed}, internal
}

// stepExchange gets the business token and checks that it grants the WABA
// and that the number belongs to it.
func (s *Service) stepExchange(ctx context.Context, cfg TechProviderConfig, row db.MetaOnboarding, code string) (db.MetaOnboarding, error) {
	wa := s.tpClient(cfg)
	token, err := s.onboardingToken(row)
	if err != nil {
		return row, err
	}
	var sealed []byte
	if token == "" {
		if code == "" {
			return row, failStep("code_required", "The code from Meta's popup was used or has expired; connect with Meta again.")
		}
		if token, err = wa.ExchangeCode(ctx, cfg.AppID, cfg.AppSecret, code); err != nil {
			return row, metaStepError("code_invalid", err)
		}
		if sealed, err = s.sealer.Seal([]byte(token), onboardingTokenAD(row.ID)); err != nil {
			return row, err
		}
		// Kept at once: the code works only once.
		if row, err = s.q.AdvanceMetaOnboarding(ctx, db.AdvanceMetaOnboardingParams{ID: row.ID, Step: StepExchange, BusinessTokenEnc: sealed}); err != nil {
			return row, err
		}
	}
	info, err := wa.DebugToken(ctx, token, cfg.appToken())
	switch {
	case err != nil:
		return row, metaStepError("token_check_failed", err)
	case !info.Valid:
		return row, failStep("token_invalid", "Meta says the business token is not valid: "+info.Error)
	case info.AppID != "" && info.AppID != cfg.AppID:
		return row, failStep("token_other_app", fmt.Sprintf("The business token belongs to app %s, not to Messora's app.", info.AppID))
	}
	if ids, ok := info.Targets["whatsapp_business_management"]; !ok || !slices.Contains(ids, row.WabaID) {
		return row, failStep("waba_not_granted", fmt.Sprintf("Meta's popup did not grant access to WhatsApp Business Account %s.", row.WabaID))
	}
	numbers, err := wa.ListPhoneNumbers(ctx, row.WabaID, token)
	if err != nil {
		return row, metaStepError("token_check_failed", err)
	}
	i := slices.IndexFunc(numbers, func(n whatsapp.PhoneNumber) bool { return n.ID == row.PhoneNumberID })
	if i < 0 {
		return row, failStep("phone_not_in_waba", fmt.Sprintf("Phone number %s does not belong to WhatsApp Business Account %s.", row.PhoneNumberID, row.WabaID))
	}
	var name *string
	if row.Name == "" {
		n := numbers[i].VerifiedName
		if n == "" {
			n = "WhatsApp " + numbers[i].DisplayPhoneNumber
		}
		n = strings.TrimSpace(n)
		if len(n) > 120 {
			n = n[:120]
		}
		name = &n
	}
	return s.q.AdvanceMetaOnboarding(ctx, db.AdvanceMetaOnboardingParams{ID: row.ID, Step: StepClaim, Name: name})
}

// errRouteTaken is meta_route_claim's error for a WABA of another tenant.
const errRouteTaken = "MS001"

// stepClaim routes the WABA to the tenant and stores its account
// (disabled until finish), in one transaction.
func (s *Service) stepClaim(ctx context.Context, cfg TechProviderConfig, row db.MetaOnboarding) (db.MetaOnboarding, error) {
	token, err := s.onboardingToken(row)
	if err != nil {
		return row, err
	}
	var accID uuid.UUID
	var next db.MetaOnboarding
	const pending = "Connecting through Messora's Meta app (Embedded Signup) has not finished."
	err = s.rawTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := tx.Exec(ctx, `SELECT meta_route_claim($1)`, row.WabaID); err != nil {
			return err
		}
		acc, err := q.GetWhatsAppAccountByWABA(ctx, row.WabaID)
		switch {
		case err == nil && acc.ConnectionMethod == MethodOwnApp && acc.Status != "disabled":
			return failStep("waba_connected_own_app", "This WhatsApp Business Account is connected with your own Meta app. Disable that account under Channels first; its history is kept.")
		case err == nil:
			accID = acc.ID
			sealed, err := s.sealer.Seal([]byte(token), tokenAD(acc.ID))
			if err != nil {
				return err
			}
			if acc.ConnectionMethod == MethodOwnApp {
				_, err = q.ConvertAccountToTechProvider(ctx, db.ConvertAccountToTechProviderParams{ID: acc.ID, AppID: &cfg.AppID,
					AccessTokenEnc: sealed, LastError: ptrS(pending)})
			} else {
				_, err = q.SetTechProviderAccountToken(ctx, db.SetTechProviderAccountTokenParams{ID: acc.ID, AccessTokenEnc: sealed, AppID: &cfg.AppID})
			}
			if err != nil {
				return err
			}
		case errors.Is(err, pgx.ErrNoRows):
			accID = uuid.Must(uuid.NewV7())
			sealed, err := s.sealer.Seal([]byte(token), tokenAD(accID))
			if err != nil {
				return err
			}
			if _, err := q.InsertTechProviderAccount(ctx, db.InsertTechProviderAccountParams{ID: accID, Name: row.Name, WabaID: row.WabaID,
				AppID: &cfg.AppID, AccessTokenEnc: sealed, LastError: ptrS(pending), CreatedBy: row.CreatedBy}); err != nil {
				return err
			}
		default:
			return err
		}
		next, err = q.AdvanceMetaOnboarding(ctx, db.AdvanceMetaOnboardingParams{ID: row.ID, Step: StepSubscribe, AccountID: &accID})
		return err
	})
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &pg) && pg.Code == errRouteTaken, isUnique(err):
		// Routed to another tenant, or its account exists there (the WABA
		// is unique across the platform).
		return row, failStep("waba_in_use", "This WhatsApp Business Account is connected to another Messora workspace.")
	case err != nil:
		return row, err
	}
	return next, nil
}

func (s *Service) stepSubscribe(ctx context.Context, cfg TechProviderConfig, row db.MetaOnboarding) (db.MetaOnboarding, error) {
	token, err := s.onboardingToken(row)
	if err != nil {
		return row, err
	}
	if err := s.tpClient(cfg).SubscribeApp(ctx, row.WabaID, token); err != nil {
		return row, metaStepError("subscribe_failed", err)
	}
	return s.q.AdvanceMetaOnboarding(ctx, db.AdvanceMetaOnboardingParams{ID: row.ID, Step: StepRegister})
}

func (s *Service) stepRegister(ctx context.Context, cfg TechProviderConfig, row db.MetaOnboarding, pin string) (db.MetaOnboarding, error) {
	token, err := s.onboardingToken(row)
	if err != nil {
		return row, err
	}
	wa := s.tpClient(cfg)
	// Registered already (a retry after the response was lost): skip.
	if n, err := wa.GetPhoneNumber(ctx, row.PhoneNumberID, token); err == nil && n.Status == "CONNECTED" {
		return s.q.AdvanceMetaOnboarding(ctx, db.AdvanceMetaOnboardingParams{ID: row.ID, Step: StepFinish})
	}
	if err := wa.RegisterPhoneNumber(ctx, row.PhoneNumberID, token, pin); err != nil {
		return row, metaStepError("register_failed", err)
	}
	return s.q.AdvanceMetaOnboarding(ctx, db.AdvanceMetaOnboardingParams{ID: row.ID, Step: StepFinish})
}

// stepFinish reads the numbers and the token's health and activates the
// account; the tenant's connection method becomes tech_provider.
func (s *Service) stepFinish(ctx context.Context, cfg TechProviderConfig, row db.MetaOnboarding) (db.MetaOnboarding, error) {
	if row.AccountID == nil {
		return row, errors.New("onboarding has no account")
	}
	token, err := s.onboardingToken(row)
	if err != nil {
		return row, err
	}
	numbers, err := s.tpClient(cfg).ListPhoneNumbers(ctx, row.WabaID, token)
	if err != nil {
		return row, metaStepError("finish_failed", err)
	}
	rep, err := s.inspectToken(ctx, row.WabaID, cfg.AppID, cfg.AppSecret, token)
	if err != nil {
		var p *tokenProblem
		if errors.As(err, &p) {
			return row, failStep("token_invalid", p.msg)
		}
		return row, metaStepError("finish_failed", err)
	}
	if _, err := s.Connection(ctx); err != nil {
		return row, err
	}
	id := *row.AccountID
	var done db.MetaOnboarding
	err = s.tx(ctx, func(q *db.Queries) error {
		if err := upsertNumbers(ctx, q, id, numbers); err != nil {
			return err
		}
		if _, err := s.saveTokenInfo(ctx, q, id, rep); err != nil {
			return err
		}
		if err := q.ActivateWhatsAppAccount(ctx, id); err != nil {
			return err
		}
		if err := q.SetMetaConnectionMethod(ctx, MethodTechProvider); err != nil {
			return err
		}
		if done, err = q.CompleteMetaOnboarding(ctx, row.ID); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.connect", EntityType: "whatsapp_account", EntityID: &id,
			Changes: map[string]any{"method": MethodTechProvider, "waba_id": row.WabaID, "phone_numbers": len(numbers), "onboarding_id": row.ID.String()}})
	})
	var ae *apperr.Error
	if errors.As(err, &ae) && ae.Code == "NUMBER_OWNED_ELSEWHERE" {
		return row, failStep("number_in_use", ae.Message)
	}
	if err != nil {
		return row, err
	}
	go s.syncTemplatesDetached(context.WithoutCancel(ctx), id)
	return done, nil
}

// DisconnectTechProvider ends a Tech Provider connection: Messora's app
// leaves the WABA's webhooks, the route is removed and the account is
// disabled with its history kept. It reports Meta's refusal to unsubscribe
// as a warning (the route is gone either way, so no event is accepted).
func (s *Service) DisconnectTechProvider(ctx context.Context, id uuid.UUID) (db.WhatsappAccount, string, error) {
	cfg, err := s.enabledTechProvider(ctx)
	if err != nil {
		return db.WhatsappAccount{}, "", err
	}
	acc, err := s.q.GetWhatsAppAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return acc, "", apperr.NotFound("WhatsApp account not found.")
	}
	if err != nil {
		return acc, "", err
	}
	if acc.ConnectionMethod != MethodTechProvider {
		return acc, "", apperr.Conflict("NOT_TECH_PROVIDER", "This account is connected with your own Meta app.")
	}
	warning := s.unsubscribeTP(ctx, cfg, acc)
	err = s.rawTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := tx.Exec(ctx, `SELECT meta_route_release($1)`, acc.WabaID); err != nil {
			return err
		}
		if acc, err = q.DisableWhatsAppAccount(ctx, db.DisableWhatsAppAccountParams{ID: id,
			Reason: ptrS("Disconnected from Messora's Meta app. Connect it again with Meta, or with your own app.")}); err != nil {
			return err
		}
		if err := s.afterTechProviderRemoved(ctx, q); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "channel.tech_provider.disconnect", EntityType: "whatsapp_account", EntityID: &id,
			Changes: map[string]any{"waba_id": acc.WabaID, "unsubscribed": warning == ""}})
	})
	return acc, warning, err
}

// unsubscribeTP removes Messora's app from the account's WABA, best effort.
func (s *Service) unsubscribeTP(ctx context.Context, cfg TechProviderConfig, acc db.WhatsappAccount) string {
	tok, err := s.sealer.Open(acc.AccessTokenEnc, tokenAD(acc.ID))
	if err == nil {
		err = s.tpClient(cfg).UnsubscribeApp(ctx, acc.WabaID, string(tok))
	}
	if err != nil {
		s.log.WarnContext(ctx, "unsubscribe Messora's app from the WABA", "waba_id", acc.WabaID, "error", err)
		return "Meta did not confirm that Messora's app left this WhatsApp Business Account; its events are no longer accepted. Remove the app in WhatsApp Manager if it is still listed."
	}
	return ""
}

// afterTechProviderRemoved switches the tenant back to its own app when no
// live Tech Provider account is left.
func (s *Service) afterTechProviderRemoved(ctx context.Context, q *db.Queries) error {
	n, err := q.CountLiveTechProviderAccounts(ctx)
	if err != nil || n > 0 {
		return err
	}
	return q.SetMetaConnectionMethod(ctx, MethodOwnApp)
}

func ptrS(s string) *string { return &s }
