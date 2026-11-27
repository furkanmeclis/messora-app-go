# Transactional e-mail

Messora's own e-mails (verification, password reset, invitations, billing and
security notices) are rendered by `internal/platform/mailer/templates` and sent
by `internal/platform/mailer` as `multipart/alternative` (plain text first,
then HTML). Campaign and automation messages are not covered here.

## Sending

`mailer.Message` carries `To`, `Subject`, `Body` (the text part) and the
optional `HTML`, `ReplyTo` and `ListUnsubscribe`. Without `HTML` the message is
a single `text/plain` part, so older callers work unchanged. Bodies are
quoted-printable (7-bit safe, short lines), subjects are RFC 2047 encoded
(Turkish characters round-trip) and every message gets `Date`, `Message-ID`
and `Auto-Submitted: auto-generated`. An https `ListUnsubscribe` also adds
RFC 8058 one-click (`List-Unsubscribe-Post`).

```go
r, _ := templates.Default() // or templates.New(brand)
msg, err := r.Message(user.Email, templates.PasswordReset, user.Locale, templates.PasswordResetData{
    Common:    templates.Common{Name: user.FirstName},
    ResetURL:  "https://acme.messora.tech/reset-password?token=…",
    ExpiresIn: time.Hour,
})
if err == nil {
    err = sender.Send(ctx, msg)
}
```

`Render(kind, locale, data)` returns `subject, text, html`; `Email(...)` also
returns the preheader and the unsubscribe URL.

### Delivery: Resend

The platform sends through **Resend** over SMTP: host `smtp.resend.com`, port
`465`, `SMTP_TLS=tls`, user `resend`, password = the Resend API key, and a
`SMTP_FROM` on a domain verified in Resend (SPF/DKIM). Enterprise tenants may
send through their own SMTP server (entitlement `custom_smtp`); that per-tenant
sender is wired in Phase 17 WP8. The templates do not depend on the transport.

## Templates

| Kind | When | Unsubscribe |
| --- | --- | --- |
| `verify_email` | Sign-up: code + button, expiry | no |
| `password_reset` | "Forgot password": button, expiry, "didn't request this?" | no |
| `password_changed` | Security notice after a password change | no |
| `invitation` | Invited to a workspace by X with role Y; accept button, expiry | no |
| `welcome` | After verification; next steps: WhatsApp number, contacts, first campaign | no |
| `login_alert` | New sign-in: time, IP, approximate location, device; "wasn't you?" | no |
| `two_factor_code` | One-time sign-in code (kept out of subject and preheader) | no |
| `email_change_confirm` | Confirm a new address (sent to the new address) | no |
| `invoice_issued` | New invoice: number, amount, due date, bank transfer block, view button | no |
| `payment_reminder` | Upcoming or overdue (`Overdue` + `GraceEndsAt`) invoice | no |
| `payment_received` | Payment confirmation | no |
| `subscription_suspended` | Workspace switched to read-only, how to fix it | no |
| `credit_low` | Message or AI credit balance below the alert threshold | yes |

Every kind has a typed data struct (`VerifyEmailData`, …) that embeds
`Common` (`Name`, `TenantName`, `TenantLogoURL`, `UnsubscribeURL`). Rendering
fails with `ErrInvalidData` listing every missing or invalid field. All links
must be absolute `https` URLs; `UnsubscribeURL` is rejected for security and
billing mail. Locales are `tr` and `en`; anything else (`Locale("de-DE")`)
falls back to English.

Preview all of them with `make mail-preview` and open
`tmp/mail-preview/index.html` (git-ignored).

## Adding a template

1. Add a `Kind` constant, append it to `kinds`, and add a data struct with
   `kind()` and `validate()` in `data.go` (and to `unsubscribable` only if the
   recipient may opt out).
2. Write `files/tr/<kind>.txt` and `files/en/<kind>.txt` defining `subject`,
   `preheader`, `reason` (the footer "why you got this" line) and `content`
   (plain-text body).
3. Write `files/tr/<kind>.html` and `files/en/<kind>.html` defining `content`,
   built from the partials in `files/partials.html` (`h1`, `greeting`,
   `button`, `code`, `rows`, `note`, `bank`) and `{{css "preset"}}` styles.
4. Add realistic data to `Sample` in `samples.go`, run
   `go test ./internal/platform/mailer/templates -update`, review the new
   `testdata/*.golden` files and `make mail-preview`.

## Brand and client rules

- **Inline styles only** (`{{css "…"}}` presets in `format.go`) as the light
  default. No external CSS, fonts, scripts or SVG.
- **Full dark theme** in the single `<head>` `<style>` block:
  `@media (prefers-color-scheme: dark)` plus Outlook.com's `[data-ogsc]` /
  `[data-ogsb]` selectors, all `!important` (body `#0B1220`, card `#141C2F`,
  border `#25304A`, text `#F4F6FB`, muted `#C5CCDD`, panels `#1B2540`; button
  stays Indigo with white text, Mint stays). Apple Mail darkens a white card
  but keeps inline text colours, so **every element with an inline colour
  must carry a theming class** (`m-bg`, `m-card`, `m-text`, `m-muted`,
  `m-box`, `m-link`, `m-btn`, `m-accent`, `m-mark`, `m-mint`, `m-word`); the
  golden tests fail otherwise. The same block resets
  `a[x-apple-data-detectors]` (and Gmail's `u + #body a`) so auto-detected
  dates, numbers and addresses keep the surrounding style, and the
  `format-detection` meta turns detection off where supported.
- **Table layout**, 600 px max width, every table `role="presentation"`.
  Buttons are "bulletproof": the coloured `<td>` carries the background, and a
  plain link fallback follows every button.
- **Preheader** text right after `<body>`, padded so clients do not pull body
  text into the inbox preview.
- **Colours**: background Fog `#F4F6FB`, card white, text Night `#0B1220`,
  primary Indigo `#3B4CCA` (buttons, white text; links), accent Mint
  `#2DD4A7` only for small accents (top bar, step badges, note border) with
  Night text on it, muted `#4A5468`.
- **Type**: headings `'Plus Jakarta Sans', Inter, -apple-system, 'Segoe UI',
  Roboto, Arial, sans-serif` bold; body Inter with system fallbacks.
- **Logo**: clients block SVG, so the mark is an indigo rounded cell with a
  mint dot plus the "messora" wordmark (800, letter-spacing -0.03em) in text.
  Setting `Brand.AssetBaseURL` switches to `<AssetBaseURL>/email/logo.png`
  (264×56, shown at 132×28) with `alt="messora"`.
- **Tenant e-mails**: when `TenantName` is set the header reads
  "<TenantName> · Messora ile" ("· with Messora"), with `TenantLogoURL` as an
  image (alt = tenant name) when given.
- **Footer**: why-you-got-this line, support link, unsubscribe link only for
  opt-out kinds, company name (`Brand.CompanyName`, a placeholder until the
  legal entity is configured) and optional address.
- **Copy**: clear, warm and reassuring; no hype. Security mail always says what
  to do if it wasn't you. Codes never go into a 2FA subject.
- Golden tests enforce: no `<script>`, no external CSS, https on every link,
  `alt` on every image, `role="presentation"` tables, HTML under 100 KB.
