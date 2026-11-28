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

### Who sends what (identity)

`internal/modules/identity` sends every identity e-mail through the
templates, in the recipient's language (`users.locale`; an invitation to a
new address uses the workspace's default language), branded with the
workspace's name and logo (`TenantName`, `TenantLogoURL` =
`<workspace>/bff/logo?v=…`) and with absolute links to the workspace's
primary address (`https://<slug>.<base>` or its custom domain):

| Kind | Sent by | Link |
| --- | --- | --- |
| `verify_email` | sign-up, "resend" | `<workspace>/verify-email?token=msv_…` (24 h) |
| `welcome` | after the address is verified | dashboard, channels, contacts, new campaign |
| `password_reset` | "forgot password" on a workspace, a platform admin's forced reset | `<workspace>/reset-password?token=msr_…` (30 min) |
| `password_changed` | password change or reset | `<workspace>/forgot-password` ("wasn't you?") |
| `invitation` | a member's invitation, a platform admin's owner invitation ("Messora" as the inviter) | `<workspace>/invite?token=msi_…` (7 days) |
| `login_alert` | a sign-in from an IP the person never used in that workspace (not the first sign-in) | `<workspace>/forgot-password` |
| `email_change_confirm` | an address change, sent to the new address | `<workspace>/confirm-email?token=mse_…` (24 h) |

`two_factor_code` is not used: the second factor is TOTP. E-mail is best
effort: a failed send is logged and the action still succeeds (the
invitation link is also returned once in the API response).

On stacks whose `PUBLIC_APP_URL` is plain `http://` (development, E2E) the
renderer is switched to `Renderer.AllowHTTP()` so links may be http; in
production every link must be https.

### Delivery: Resend

The platform sends through **Resend** over SMTP: host `smtp.resend.com`, port
`465`, TLS `tls`, user `resend`, password = the Resend API key, and a From
address on a domain verified in Resend (SPF/DKIM). The templates do not depend
on the transport. `settings.Mail` decides per message (Phase 17 WP8):

- **The platform sender** is the one set in the platform admin panel
  (E-mail sender page, `GET/PUT /platform/email`, password sealed, test send,
  `platform.email_update` in the platform audit log), else `SMTP_*` of the
  environment, else e-mail is off. A panel change applies within 15 seconds
  on every replica; an empty host returns to the environment's sender.
- **Mail about a company** through the platform sender carries the
  company's name as the display name (the address stays the platform's,
  e.g. `"Acme Gıda" <no-reply@messora.tech>`) and the company's reply-to
  address (Settings → E-mail, `PATCH /settings/email`), else the platform's
  reply-to. Non-enterprise companies never see SMTP settings.
- **Enterprise companies** (`custom_smtp`) may send through their own SMTP
  server (`PUT /settings/smtp`, `POST /settings/smtp/test`; other plans get
  `403 FEATURE_NOT_IN_PLAN`). When a send through it fails, the message is
  sent through the platform sender instead (never lost) and the failure is
  kept (`settings.smtp_failure`): the E-mail page shows it as a warning until
  a send or test through the company's server works again. A test send never
  falls back. A downgraded company's stored server is kept but unused.
- Platform mail (plan change requests to the platform admins) always goes
  through the platform sender.

| Kind | Sent by | Link |
| --- | --- | --- |
| `plan_change_request` | a workspace asks for another plan; to every active platform admin, in their language | `https://admin.<base>/plan-requests` |

### Billing (Phase 17 WP9)

`internal/modules/billing` sends the billing kinds through
`identity.NotifyBilling`: to the workspace's active owners and to the
billing e-mail of its invoice details (in the workspace's default language
when that address is not an owner's), branded, after the transaction that
triggered them, once each (`billing_events`). Links go to the workspace's
`/settings/billing`.

| Kind | Sent when | Notes |
| --- | --- | --- |
| `invoice_issued` | a proforma is issued (a renewal 10 days before the period ends, a plan choice, an upgrade, an AI credit pack, a message balance top-up) | `InvoiceNo` is the proforma number; the bank block carries the first IBAN and the payment reference |
| `payment_reminder` | 7 and 3 days before a renewal's due date and on the due day; `Overdue` with `GraceEndsAt` when the period ended unpaid | |
| `payment_received` | a proforma is paid in full (matched transfer or account credit) | method "Havale/EFT" |
| `subscription_suspended` | the workspace turns read-only (trial ended, grace ended) | the unpaid renewal and its open amount when there is one |
| `credit_low` | the message balance of a Tech Provider workspace falls below the threshold (once until a top-up re-arms it) | `CreditType` `message`, amounts in whole TL; queued as the outbox task `billing:notify` |

No new kind was needed.

## Templates

| Kind | When | Unsubscribe |
| --- | --- | --- |
| `verify_email` | Sign-up: button (and an optional code), expiry | no |
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
| `plan_change_request` | Platform admins: a workspace's plan change request (current and requested plan, billing, who, message) | no |

Every kind has a typed data struct (`VerifyEmailData`, …) that embeds
`Common` (`Name`, `TenantName`, `TenantLogoURL`, `UnsubscribeURL`). Rendering
fails with `ErrInvalidData` listing every missing or invalid field. All links
must be absolute `https` URLs; `UnsubscribeURL` is rejected for security and
billing mail. Locales are `tr` and `en`; anything else (`Locale("de-DE")`)
falls back to English.

Preview all of them with `make mail-preview` and open
`tmp/mail-preview/index.html` (git-ignored).

## The design system

Every e-mail has the same frame, rendered by `files/layout.html`; a kind only
supplies its words and picks blocks from `files/partials.html`:

1. **Header**: the real Messora logo, identical in every e-mail: the
   horizontal lockup from the logo kit as retina PNGs on
   `Brand.AssetBaseURL` (default `https://messora.tech`, the marketing site):
   `email/logo-light@2x.png` (colour on light) and `email/logo-dark@2x.png`
   (colour on dark), 1020×212 shown at 150×31, `alt="Messora"`, `border=0`,
   `display:block`. The dark one sits in a `.m-logo-dark` wrapper hidden with
   `display:none; mso-hide:all` and swapped in by the dark-mode block
   (`prefers-color-scheme` and `[data-ogsc]`); clients without it show the
   light logo. With images blocked the alt text shows "Messora" styled as
   the wordmark (22 px, 800). `email/mark@2x.png` (the mark alone) is
   published for later use. Mail about a workspace
   shows the workspace on the right: its logo (`TenantLogoURL`, 28 px high,
   alt = name) or its name. The text part starts with `Messora · <workspace>`.
2. **Hero**: the eyebrow (the kind's category, small indigo capitals:
   Hesap, Güvenlik, Davet, Başlangıç, Fatura ve ödeme, Krediler, Webhook'lar,
   Platform yönetimi), the title (`title` in the kind's `.txt`, also the text
   part's heading) and the greeting.
3. **Content** in a fixed order: the lead paragraph (what happened), the
   data (`code`, `card`/`rows`, `bank`), a `notice` that asks for action
   (warning/danger) before the button it leads to, the one primary
   `button`, the `fallback` copy-paste link (one-time token links only:
   verify, reset, invitation, address change), small print (validity, "the
   location is approximate"), and last an info `notice` for "didn't ask for
   this?".
4. **Footer**: why you got this, support link, unsubscribe (opt-out kinds
   only), "sent automatically", © company and address.

| Partial | Call | Use |
| --- | --- | --- |
| `button` | `(dict "URL" u "Label" "…")` | The only button. Indigo cell with white text (bulletproof) plus a VML `v:roundrect` for Outlook for Windows; full width on phones. One per e-mail. |
| `fallback` | `(dict "URL" u "L" .L)` | "If the button doesn't work…" and the URL. |
| `link` | `(dict "URL" u "Label" "…")` | Secondary inline link (indigo, underlined, semibold). |
| `rows` / `card` | `(rows "Label" v …)` / `(dict "Title" t "Rows" r "Hint" h)` | Detail card: fog panel, muted labels, semibold values; empty values are skipped, an empty card is not drawn. `(mono v)` sets a value in monospace (IBAN, references, URLs, error codes). Rows stack on phones. |
| `bank` | `(dict "B" .D.Bank "L" .L)` | Bank transfer card: bank, holder, monospace IBAN and reference, hint. |
| `code` | `.D.Code` | One-time code: full-width monospace panel. |
| `notice` | `(notice "info\|success\|warning\|danger" "Title" "Text")` | Tinted panel with a 4 px left bar in the tone's colour (indigo, mint, amber, red); the bold title runs into the text. |
| `step` | `(dict "N" "1" "Title" … "Text" … "URL" … "Label" …)` | Numbered step with a mint badge (welcome). |
| `h2` | `"…"` | Section heading. |

Paragraphs are `<p {{ui "p"}}>` (16/24 body) and `<p {{ui "small"}}>`
(14/20 muted); `<strong>` is allowed inside. Kind files contain no `style`,
`class`, `bgcolor`, tables, links, images or headings of their own
(`TestKindFilesUseOnlyPartials`).

**Scale.** Spacing on an 8 px grid (card padding 40, 24 on phones; blocks
24 apart; paragraphs 16), line heights on a 4 px grid. Type: eyebrow 12/16
700 +0.08em, title 26/32 800 -0.02em, body 16/24, small 14/20, footer
12/20, code 32/40 mono. Width 600 px, card radius 16, panels 12, button 10.

**Voice.** Warm, short, "siz" in Turkish. Customer mail says "çalışma alanı"
(the app's term); the platform admin mail (`plan_change_request`) says
"firma", like the admin panel. Subjects are sentence case, no full stop, an
identifier after a colon ("Faturanız hazır: MSR-2026-0142"). Security mail
always says what to do if it wasn't you; codes never go into a subject.

## Adding a template

1. Add a `Kind` constant, append it to `kinds`, give it a category in
   `categories` (`format.go`), and add a data struct with `kind()` and
   `validate()` in `data.go` (and to `unsubscribable` only if the recipient
   may opt out).
2. Write `files/tr/<kind>.txt` and `files/en/<kind>.txt` defining `subject`,
   `preheader`, `title` (the hero title), `reason` (the footer "why you got
   this" line) and `content` (plain-text body after the greeting).
3. Write `files/tr/<kind>.html` and `files/en/<kind>.html` defining `content`
   from the partials above, in the order above.
4. Add realistic data to `Sample` in `samples.go`, run
   `go test ./internal/platform/mailer/templates -update`, review the new
   `testdata/*.golden` files and `make mail-preview`.

## Brand and client rules

- **Inline styles only**, written by `{{ui "preset"}}` (class and style
  together, presets in `format.go`) as the light default. No external CSS,
  fonts, scripts or SVG.
- **Full dark theme** in the single `<head>` `<style>` block:
  `@media (prefers-color-scheme: dark)` plus Outlook.com's `[data-ogsc]` /
  `[data-ogsb]` selectors, all `!important` (body `#0B1220`, card `#141C2F`,
  border `#25304A`, text `#F4F6FB`, muted `#C5CCDD`, panels and notices
  `#1B2540`; button stays Indigo with white text, Mint stays, notice bars
  keep their colour). Apple Mail darkens a white card but keeps inline text
  colours, so **every preset with a colour carries a theming class**
  (`m-bg`, `m-card`, `m-text`, `m-muted`, `m-box`, `m-note`, `m-link`,
  `m-btn`, `m-mint`, `m-logo-light`, `m-logo-dark`); `TestPresetsAreThemed` and the
  golden tests fail otherwise. The same block resets
  `a[x-apple-data-detectors]` (and Gmail's `u + #body a`) so auto-detected
  dates, numbers and addresses keep the surrounding style, and the
  `format-detection` meta turns detection off where supported.
- **Outlook for Windows**: the button is a VML rounded rectangle inside
  `<!--[if mso]>`, the HTML button is hidden from it; an `mso` block sets
  96 PPI and an Arial fallback (Outlook would otherwise fall back to Times).
- **Table layout**, 600 px max width, every table `role="presentation"`.
- **Preheader** text right after `<body>`, padded so clients do not pull body
  text into the inbox preview.
- **Colours**: background Fog `#F4F6FB`, card white, text Night `#0B1220`,
  primary Indigo `#3B4CCA` (eyebrow, button, links, info bar), Mint `#2DD4A7`
  only for small accents (step badges, success bar) with Night text
  on it, muted `#4A5468`, warning bar `#E39A0B`, danger bar `#D93F4C`.
- **Type**: headings `'Plus Jakarta Sans', Inter, -apple-system, 'Segoe UI',
  Roboto, Arial, sans-serif`; body Inter with system fallbacks; monospace
  `'SF Mono', Menlo, Consolas`.
- Tests enforce: no `<script>`, no external CSS, https on every link, `alt`
  on every image, `role="presentation"` tables, HTML under 100 KB, the
  shared header, hero and footer in every e-mail, the VML button wherever
  there is an action, and a theming class on every coloured element.
