package templates

import "time"

// Sample returns realistic, deterministic data for kind in locale. It feeds
// the preview gallery (cmd/mailpreview) and the golden tests.
func Sample(kind Kind, locale string) Data {
	tr := Locale(locale) == "tr"
	pick := func(t, e string) string {
		if tr {
			return t
		}
		return e
	}
	ist := time.FixedZone("", 3*60*60)
	at := time.Date(2026, 11, 27, 14, 5, 0, 0, ist)
	app := "https://acme.messora.tech"
	tenant := Common{Name: "Ayşe", TenantName: "Acme Gıda"}
	bank := &BankTransfer{
		BankName:      "Örnek Bankası",
		AccountHolder: "Messora Yazılım A.Ş.",
		IBAN:          "TR330006100519786457841326",
		Reference:     "MSR-2026-0142",
	}
	amount := Money{Minor: 249000, Currency: "TRY"}
	switch kind {
	case VerifyEmail:
		return VerifyEmailData{Common: Common{Name: "Ayşe"}, Code: "482913",
			VerifyURL: "https://app.messora.tech/verify?token=7f3c9a1e", ExpiresIn: 30 * time.Minute}
	case PasswordReset:
		return PasswordResetData{Common: Common{Name: "Ayşe"},
			ResetURL: app + "/reset-password?token=b81d0c55", ExpiresIn: time.Hour, IP: "88.230.14.7"}
	case PasswordChanged:
		return PasswordChangedData{Common: Common{Name: "Ayşe"}, ChangedAt: at, IP: "88.230.14.7",
			Device: "Safari, macOS", SecureURL: app + "/forgot-password"}
	case Invitation:
		return InvitationData{Common: tenant, InviterName: "Mehmet Demir", WorkspaceName: "Acme Gıda",
			Role: pick("Yönetici", "Admin"), AcceptURL: app + "/invite/accept?token=c4e2a9", ExpiresIn: 7 * 24 * time.Hour}
	case Welcome:
		return WelcomeData{Common: Common{Name: "Ayşe"}, DashboardURL: app + "/",
			ConnectWhatsAppURL: app + "/settings/whatsapp", ImportContactsURL: app + "/contacts/import",
			NewCampaignURL: app + "/campaigns/new"}
	case LoginAlert:
		return LoginAlertData{Common: Common{Name: "Ayşe"}, At: at, IP: "88.230.14.7",
			Location: pick("İstanbul, Türkiye", "Istanbul, Türkiye"), Device: "Chrome, Windows",
			SecureURL: app + "/forgot-password"}
	case TwoFactorCode:
		return TwoFactorCodeData{Common: Common{Name: "Ayşe"}, Code: "730154", ExpiresIn: 10 * time.Minute}
	case EmailChangeConfirm:
		return EmailChangeConfirmData{Common: Common{Name: "Ayşe"}, NewEmail: "ayse.yilmaz@acmegida.com.tr",
			ConfirmURL: app + "/account/email/confirm?token=5d1f0e", ExpiresIn: 24 * time.Hour}
	case InvoiceIssued:
		return InvoiceIssuedData{Common: tenant, InvoiceNo: "MSR-2026-0142", Period: pick("Kasım 2026", "November 2026"),
			Amount: amount, DueDate: at.AddDate(0, 0, 10), Bank: bank, InvoiceURL: app + "/billing/invoices/MSR-2026-0142"}
	case PaymentReminder:
		return PaymentReminderData{Common: tenant, InvoiceNo: "MSR-2026-0142", Amount: amount,
			DueDate: at.AddDate(0, 0, -3), Overdue: true, GraceEndsAt: at.AddDate(0, 0, 4), Bank: bank,
			PayURL: app + "/billing/invoices/MSR-2026-0142"}
	case PaymentReceived:
		return PaymentReceivedData{Common: tenant, InvoiceNo: "MSR-2026-0142", Amount: amount, PaidAt: at,
			Method: pick("Havale / EFT", "Bank transfer"), InvoiceURL: app + "/billing/invoices/MSR-2026-0142"}
	case SubscriptionSuspended:
		return SubscriptionSuspendedData{Common: tenant, SuspendedAt: at, InvoiceNo: "MSR-2026-0142",
			Amount: amount, BillingURL: app + "/billing"}
	case CreditLow:
		c := tenant
		c.UnsubscribeURL = app + "/settings/notifications/unsubscribe?k=credit_low&t=9a7e"
		return CreditLowData{Common: c, CreditType: CreditMessage, Balance: 1250, Threshold: 2000,
			TopUpURL: app + "/billing/credits"}
	}
	return nil
}
