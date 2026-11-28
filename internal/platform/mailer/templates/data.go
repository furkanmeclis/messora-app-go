package templates

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Data is the typed payload of one kind. Implementations embed Common.
type Data interface {
	kind() Kind
	common() Common
	validate(v *validator)
}

// Common is carried by every kind.
type Common struct {
	// Name is the recipient's first name for the greeting (optional).
	Name string
	// TenantName shows the workspace on the right of the header when set.
	TenantName string
	// TenantLogoURL is an optional https image shown instead of TenantName.
	TenantLogoURL string
	// UnsubscribeURL (https) is only allowed for opt-out notifications
	// (credit_low); it adds a footer link and the List-Unsubscribe header.
	UnsubscribeURL string
}

func (c Common) common() Common { return c }

func (c Common) validateCommon(v *validator, unsubscribable bool) {
	v.url("TenantLogoURL", c.TenantLogoURL, false)
	if c.TenantLogoURL != "" && c.TenantName == "" {
		v.missing("TenantName")
	}
	if c.UnsubscribeURL != "" && !unsubscribable {
		v.fail("UnsubscribeURL is not allowed for this kind")
	}
	v.url("UnsubscribeURL", c.UnsubscribeURL, false)
}

// Money is an amount in minor units (kuruş, cents) and an ISO 4217 code.
type Money struct {
	Minor    int64
	Currency string
}

// BankTransfer is the "pay by bank transfer" block of billing e-mails.
type BankTransfer struct {
	BankName      string
	AccountHolder string
	IBAN          string
	// Reference must be written in the transfer description so the payment
	// can be matched to the invoice.
	Reference string
}

// VerifyEmailData is the payload for sign-up e-mail verification.
type VerifyEmailData struct {
	Common
	// Code is an optional one-time code shown next to the button.
	Code      string
	VerifyURL string
	ExpiresIn time.Duration
}

// PasswordResetData is the payload for "forgot password" link.
type PasswordResetData struct {
	Common
	ResetURL  string
	ExpiresIn time.Duration
	IP        string // optional, the requester's address
}

// PasswordChangedData is the payload for security notice after a password change.
type PasswordChangedData struct {
	Common
	ChangedAt time.Time
	IP        string // optional
	Device    string // optional
	SecureURL string // "wasn't you?" (reset / contact)
}

// InvitationData is the payload for invited to a workspace.
type InvitationData struct {
	Common
	InviterName   string
	WorkspaceName string
	Role          string // already localised label, e.g. "Yönetici"
	AcceptURL     string
	ExpiresIn     time.Duration
}

// WelcomeData is the payload for after the e-mail address is verified.
type WelcomeData struct {
	Common
	DashboardURL       string
	ConnectWhatsAppURL string
	ImportContactsURL  string
	NewCampaignURL     string
}

// LoginAlertData is the payload for sign-in from a new device or place.
type LoginAlertData struct {
	Common
	At        time.Time
	IP        string
	Location  string // optional, approximate ("İstanbul, Türkiye")
	Device    string // optional ("Chrome, macOS")
	SecureURL string
}

// TwoFactorCodeData is the payload for one-time sign-in code.
type TwoFactorCodeData struct {
	Common
	Code      string
	ExpiresIn time.Duration
}

// EmailChangeConfirmData is the payload for confirm a new e-mail address (sent to the new one).
type EmailChangeConfirmData struct {
	Common
	NewEmail   string
	ConfirmURL string
	ExpiresIn  time.Duration
}

// InvoiceIssuedData is the payload for a new invoice.
type InvoiceIssuedData struct {
	Common
	InvoiceNo  string
	Period     string // optional, e.g. "Kasım 2026"
	Amount     Money
	DueDate    time.Time
	Bank       *BankTransfer // optional
	InvoiceURL string
}

// PaymentReminderData is the payload for upcoming (Overdue=false) or overdue invoice.
type PaymentReminderData struct {
	Common
	InvoiceNo   string
	Amount      Money
	DueDate     time.Time
	Overdue     bool
	GraceEndsAt time.Time // required when Overdue
	Bank        *BankTransfer
	PayURL      string
}

// PaymentReceivedData is the payload for payment confirmation.
type PaymentReceivedData struct {
	Common
	InvoiceNo  string
	Amount     Money
	PaidAt     time.Time
	Method     string // optional, localised ("Havale/EFT")
	InvoiceURL string
}

// SubscriptionSuspendedData is the payload for workspace switched to read-only.
type SubscriptionSuspendedData struct {
	Common
	SuspendedAt time.Time
	InvoiceNo   string // optional, the unpaid invoice
	Amount      Money  // optional, the open balance
	BillingURL  string
}

// Credit types for CreditLowData.
const (
	CreditMessage = "message"
	CreditAI      = "ai"
)

// CreditLowData is the payload for message or AI credit balance below a threshold.
type CreditLowData struct {
	Common
	CreditType string // CreditMessage or CreditAI
	Balance    int64
	Threshold  int64
	TopUpURL   string
}

// PlanChangeRequestData is the payload for a workspace's request for
// another plan (sent to the platform admins).
type PlanChangeRequestData struct {
	Common
	// Workspace is the company that asks, with its address.
	Workspace    string
	WorkspaceURL string // optional
	// CurrentPlan and RequestedPlan are plan names in the admin's language.
	CurrentPlan   string // optional (a workspace without a plan)
	RequestedPlan string
	// BillingCycle is "monthly" or "yearly".
	BillingCycle string
	RequestedBy  string // name and address of the person who asked
	Message      string // optional
	RequestedAt  time.Time
	// InboxURL opens the request inbox of the platform admin panel.
	InboxURL string
}

// WebhookDisabledData is the payload for an outgoing webhook endpoint
// Messora switched off (sent to the workspace's owners and admins).
type WebhookDisabledData struct {
	Common
	// EndpointURL is the receiver without query string or credentials.
	EndpointURL string
	Description string // optional, the endpoint's description
	// Failures is the run of failed attempts; 0 when the receiver answered
	// 410 Gone.
	Failures     int64
	FailingSince time.Time // optional, when the failures began
	DisabledAt   time.Time
	// LastError is the last error, e.g. "HTTP 503" (optional).
	LastError string
	// ManageURL opens the webhook settings to re-enable the endpoint.
	ManageURL string
}

func (VerifyEmailData) kind() Kind           { return VerifyEmail }
func (PasswordResetData) kind() Kind         { return PasswordReset }
func (PasswordChangedData) kind() Kind       { return PasswordChanged }
func (InvitationData) kind() Kind            { return Invitation }
func (WelcomeData) kind() Kind               { return Welcome }
func (LoginAlertData) kind() Kind            { return LoginAlert }
func (TwoFactorCodeData) kind() Kind         { return TwoFactorCode }
func (EmailChangeConfirmData) kind() Kind    { return EmailChangeConfirm }
func (InvoiceIssuedData) kind() Kind         { return InvoiceIssued }
func (PaymentReminderData) kind() Kind       { return PaymentReminder }
func (PaymentReceivedData) kind() Kind       { return PaymentReceived }
func (SubscriptionSuspendedData) kind() Kind { return SubscriptionSuspended }
func (CreditLowData) kind() Kind             { return CreditLow }
func (WebhookDisabledData) kind() Kind       { return WebhookDisabled }
func (PlanChangeRequestData) kind() Kind     { return PlanChangeRequest }

func (d VerifyEmailData) validate(v *validator) {
	v.url("VerifyURL", d.VerifyURL, true)
	v.dur("ExpiresIn", d.ExpiresIn)
}

func (d PasswordResetData) validate(v *validator) {
	v.url("ResetURL", d.ResetURL, true)
	v.dur("ExpiresIn", d.ExpiresIn)
}

func (d PasswordChangedData) validate(v *validator) {
	v.time("ChangedAt", d.ChangedAt)
	v.url("SecureURL", d.SecureURL, true)
}

func (d InvitationData) validate(v *validator) {
	v.str("InviterName", d.InviterName)
	v.str("WorkspaceName", d.WorkspaceName)
	v.str("Role", d.Role)
	v.url("AcceptURL", d.AcceptURL, true)
	v.dur("ExpiresIn", d.ExpiresIn)
}

func (d WelcomeData) validate(v *validator) {
	v.url("DashboardURL", d.DashboardURL, true)
	v.url("ConnectWhatsAppURL", d.ConnectWhatsAppURL, true)
	v.url("ImportContactsURL", d.ImportContactsURL, true)
	v.url("NewCampaignURL", d.NewCampaignURL, true)
}

func (d LoginAlertData) validate(v *validator) {
	v.time("At", d.At)
	v.str("IP", d.IP)
	v.url("SecureURL", d.SecureURL, true)
}

func (d TwoFactorCodeData) validate(v *validator) {
	v.str("Code", d.Code)
	v.dur("ExpiresIn", d.ExpiresIn)
}

func (d EmailChangeConfirmData) validate(v *validator) {
	v.str("NewEmail", d.NewEmail)
	v.url("ConfirmURL", d.ConfirmURL, true)
	v.dur("ExpiresIn", d.ExpiresIn)
}

func (d InvoiceIssuedData) validate(v *validator) {
	v.str("InvoiceNo", d.InvoiceNo)
	v.money("Amount", d.Amount, true)
	v.time("DueDate", d.DueDate)
	v.bank(d.Bank)
	v.url("InvoiceURL", d.InvoiceURL, true)
}

func (d PaymentReminderData) validate(v *validator) {
	v.str("InvoiceNo", d.InvoiceNo)
	v.money("Amount", d.Amount, true)
	v.time("DueDate", d.DueDate)
	if d.Overdue {
		v.time("GraceEndsAt", d.GraceEndsAt)
	}
	v.bank(d.Bank)
	v.url("PayURL", d.PayURL, true)
}

func (d PaymentReceivedData) validate(v *validator) {
	v.str("InvoiceNo", d.InvoiceNo)
	v.money("Amount", d.Amount, true)
	v.time("PaidAt", d.PaidAt)
	v.url("InvoiceURL", d.InvoiceURL, true)
}

func (d SubscriptionSuspendedData) validate(v *validator) {
	v.time("SuspendedAt", d.SuspendedAt)
	v.money("Amount", d.Amount, false)
	v.url("BillingURL", d.BillingURL, true)
}

func (d CreditLowData) validate(v *validator) {
	if d.CreditType != CreditMessage && d.CreditType != CreditAI {
		v.fail("CreditType must be message or ai")
	}
	if d.Balance < 0 {
		v.fail("Balance must not be negative")
	}
	if d.Threshold <= 0 {
		v.missing("Threshold")
	}
	v.url("TopUpURL", d.TopUpURL, true)
}

func (d PlanChangeRequestData) validate(v *validator) {
	v.str("Workspace", d.Workspace)
	v.str("RequestedPlan", d.RequestedPlan)
	v.str("RequestedBy", d.RequestedBy)
	if d.BillingCycle != "monthly" && d.BillingCycle != "yearly" {
		v.fail("BillingCycle must be monthly or yearly")
	}
	v.time("RequestedAt", d.RequestedAt)
	if d.WorkspaceURL != "" {
		v.url("WorkspaceURL", d.WorkspaceURL, true)
	}
	v.url("InboxURL", d.InboxURL, true)
}

func (d WebhookDisabledData) validate(v *validator) {
	v.str("EndpointURL", d.EndpointURL)
	v.time("DisabledAt", d.DisabledAt)
	if d.Failures < 0 {
		v.fail("Failures must not be negative")
	}
	v.url("ManageURL", d.ManageURL, true)
}

// validator collects every problem so the caller sees them all at once.
type validator struct {
	problems []string
	// allowHTTP accepts http links (development stacks without TLS).
	allowHTTP bool
}

func (v *validator) fail(s string)       { v.problems = append(v.problems, s) }
func (v *validator) missing(name string) { v.fail("missing " + name) }

func (v *validator) str(name, s string) {
	if strings.TrimSpace(s) == "" {
		v.missing(name)
	}
}

func (v *validator) dur(name string, d time.Duration) {
	if d < time.Minute {
		v.missing(name)
	}
}

func (v *validator) time(name string, t time.Time) {
	if t.IsZero() {
		v.missing(name)
	}
}

func (v *validator) money(name string, m Money, required bool) {
	if !required && m == (Money{}) {
		return
	}
	if len(m.Currency) != 3 || strings.ToUpper(m.Currency) != m.Currency {
		v.fail(name + ".Currency must be an ISO 4217 code")
	}
	if m.Minor <= 0 {
		v.missing(name + ".Minor")
	}
}

func (v *validator) bank(b *BankTransfer) {
	if b == nil {
		return
	}
	v.str("Bank.AccountHolder", b.AccountHolder)
	v.str("Bank.IBAN", b.IBAN)
	v.str("Bank.Reference", b.Reference)
}

// url requires an absolute https URL with a host (links in e-mail must work
// out of context and never downgrade to http).
func (v *validator) url(name, s string, required bool) {
	if s == "" {
		if required {
			v.missing(name)
		}
		return
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && (!v.allowHTTP || u.Scheme != "http")) || u.Host == "" || u.User != nil {
		v.fail(name + " must be an absolute https URL")
	}
}

func (v *validator) err(what string) error {
	if len(v.problems) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s: %s", ErrInvalidData, what, strings.Join(v.problems, "; "))
}
