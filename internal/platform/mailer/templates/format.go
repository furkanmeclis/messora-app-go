package templates

import (
	"fmt"
	"html"
	htmltemplate "html/template"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// chrome holds the layout strings shared by every kind.
type chrome struct {
	Hello          string // followed by the name and a comma
	HelloAnon      string
	ButtonFallback string
	NeedHelp       string
	Support        string
	Unsubscribe    string
	Automated      string
	// Eyebrows: the small category label above every title.
	Category map[category]string
	// Bank transfer card.
	BankTitle     string
	BankName      string
	AccountHolder string
	Reference     string
	ReferenceHint string
}

// category groups the kinds under one eyebrow label.
type category string

const (
	catAccount  category = "account"
	catSecurity category = "security"
	catTeam     category = "team"
	catStart    category = "start"
	catBilling  category = "billing"
	catCredits  category = "credits"
	catWebhooks category = "webhooks"
	catPlatform category = "platform"
)

// categories maps every kind to its eyebrow; TestEveryKindHasACategory
// keeps it complete.
var categories = map[Kind]category{
	VerifyEmail: catAccount, EmailChangeConfirm: catAccount,
	PasswordReset: catSecurity, PasswordChanged: catSecurity, LoginAlert: catSecurity, TwoFactorCode: catSecurity,
	Invitation: catTeam, Welcome: catStart,
	InvoiceIssued: catBilling, PaymentReminder: catBilling, PaymentReceived: catBilling, SubscriptionSuspended: catBilling,
	CreditLow: catCredits, WebhookDisabled: catWebhooks, PlanChangeRequest: catPlatform,
}

var chromes = map[string]chrome{
	"tr": {
		Hello:          "Merhaba",
		HelloAnon:      "Merhaba,",
		ButtonFallback: "Buton çalışmazsa bu bağlantıyı tarayıcınıza yapıştırın:",
		NeedHelp:       "Bir sorunuz mu var?",
		Support:        "Destek ekibimize yazın",
		Unsubscribe:    "Bu bildirimleri kapat",
		Automated:      "Bu e-posta otomatik gönderildi.",
		Category: map[category]string{
			catAccount: "Hesap", catSecurity: "Güvenlik", catTeam: "Davet", catStart: "Başlangıç",
			catBilling: "Fatura ve ödeme", catCredits: "Krediler", catWebhooks: "Webhook'lar", catPlatform: "Platform yönetimi",
		},
		BankTitle:     "Havale / EFT bilgileri",
		BankName:      "Banka",
		AccountHolder: "Alıcı",
		Reference:     "Açıklama",
		ReferenceHint: "Ödemenizi hızla eşleştirebilmemiz için açıklama alanına bu kodu yazın.",
	},
	"en": {
		Hello:          "Hi",
		HelloAnon:      "Hi there,",
		ButtonFallback: "If the button doesn't work, paste this link into your browser:",
		NeedHelp:       "Questions?",
		Support:        "Write to our support team",
		Unsubscribe:    "Turn off these notifications",
		Automated:      "This e-mail was sent automatically.",
		Category: map[category]string{
			catAccount: "Account", catSecurity: "Security", catTeam: "Invitation", catStart: "Getting started",
			catBilling: "Billing", catCredits: "Credits", catWebhooks: "Webhooks", catPlatform: "Platform admin",
		},
		BankTitle:     "Bank transfer details",
		BankName:      "Bank",
		AccountHolder: "Account holder",
		Reference:     "Reference",
		ReferenceHint: "Put this reference in the transfer description so we can match your payment quickly.",
	},
}

// Brand palette, type and spacing. Changing these changes every e-mail.
// Spacing sits on an 8 px grid, line heights on a 4 px grid.
const (
	colFog    = "#F4F6FB" // page background, panels
	colNight  = "#0B1220" // text
	colIndigo = "#3B4CCA" // primary: button, links, eyebrow
	colMint   = "#2DD4A7" // accent: step badges, success bar
	colMuted  = "#4A5468" // secondary text
	colLine   = "#E3E7F0" // borders
	colAmber  = "#E39A0B" // warning bar
	colRed    = "#D93F4C" // danger bar
	fontHead  = "'Plus Jakarta Sans', Inter, -apple-system, 'Segoe UI', Roboto, Arial, sans-serif"
	fontBody  = "Inter, -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif"
	fontMono  = "'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"
)

// preset is one inline style plus the theming classes that re-colour it in
// dark mode (the <style> block in layout.html). Every preset that sets a
// colour carries an m-* class; TestPresetsAreThemed enforces it.
type preset struct{ class, style string }

func notePreset(bg, bar string) preset {
	return preset{"m-note m-text", "padding:16px 20px;background-color:" + bg + ";border-left:4px solid " + bar +
		";border-radius:0 10px 10px 0;font-family:" + fontBody + ";font-size:14px;line-height:22px;color:" + colNight + ";"}
}

// presets are written by {{ui "name"}} as class and style attributes.
// Templates use nothing else for styling: inline styles are the only thing
// every client keeps, the classes add the dark theme where supported.
var presets = map[string]preset{
	// Page and chrome.
	"body":      {"m-bg", "margin:0;padding:0;width:100%;background-color:" + colFog + ";-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%;"},
	"preheader": {"", "display:none;font-size:1px;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;mso-hide:all;"},
	"outer":     {"m-bg", "background-color:" + colFog + ";"},
	"outerPad":  {"m-outer", "padding:32px 16px;"},
	"wrap":      {"", "width:100%;max-width:600px;"},
	"header":    {"m-header", "padding:0 8px 24px 8px;"},
	// The logo: the light lockup by default, swapped for the dark one by the
	// dark-mode block. The alt text is styled as the wordmark for blocked images.
	"logoLight":    {"m-logo-light", "display:block;width:150px;height:31px;border:0;outline:none;text-decoration:none;font-family:" + fontHead + ";font-size:22px;line-height:31px;font-weight:800;letter-spacing:-0.03em;color:" + colNight + ";"},
	"logoDarkWrap": {"m-logo-dark", "display:none;mso-hide:all;max-height:0;overflow:hidden;"},
	"logoDark":     {"m-logo-dark-img", "display:block;width:150px;height:31px;border:0;outline:none;text-decoration:none;font-family:" + fontHead + ";font-size:22px;line-height:31px;font-weight:800;letter-spacing:-0.03em;color:#F4F6FB;"},
	"tenant":       {"m-muted", "padding-left:16px;font-family:" + fontBody + ";font-size:14px;line-height:20px;font-weight:600;color:" + colMuted + ";text-align:right;"},
	"tenantImg":    {"", "display:inline-block;height:28px;width:auto;max-width:160px;border:0;outline:none;text-decoration:none;"},
	"card":         {"m-card", "background-color:#FFFFFF;border:1px solid " + colLine + ";border-radius:16px;"},
	"cardPad":      {"m-pad", "padding:40px 40px 24px 40px;"},
	"footer":       {"m-footer m-muted", "padding:24px 24px 0 24px;font-family:" + fontBody + ";font-size:12px;line-height:20px;color:" + colMuted + ";text-align:center;"},
	"footerP":      {"m-muted", "margin:0 0 8px 0;font-family:" + fontBody + ";font-size:12px;line-height:20px;color:" + colMuted + ";"},
	"footerA":      {"m-muted", "color:" + colMuted + ";text-decoration:underline;"},

	// Hero: eyebrow, title, greeting.
	"eyebrow": {"m-link", "margin:0 0 8px 0;font-family:" + fontHead + ";font-size:12px;line-height:16px;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:" + colIndigo + ";"},
	"h1":      {"m-text", "margin:0 0 24px 0;font-family:" + fontHead + ";font-size:26px;line-height:32px;font-weight:800;letter-spacing:-0.02em;color:" + colNight + ";"},
	"h2":      {"m-text", "margin:24px 0 8px 0;font-family:" + fontHead + ";font-size:16px;line-height:24px;font-weight:700;color:" + colNight + ";"},

	// Copy.
	"p":     {"m-text", "margin:0 0 16px 0;font-family:" + fontBody + ";font-size:16px;line-height:24px;color:" + colNight + ";"},
	"small": {"m-muted", "margin:0 0 16px 0;font-family:" + fontBody + ";font-size:14px;line-height:20px;color:" + colMuted + ";"},
	"link":  {"m-link", "color:" + colIndigo + ";font-weight:600;text-decoration:underline;"},

	// The one primary button (bulletproof: the cell carries the colour).
	"btnTable": {"m-btn-wrap", "margin:8px 0 24px 0;"},
	"btnCell":  {"m-btn", "border-radius:10px;background-color:" + colIndigo + ";"},
	"btnLink":  {"m-btn m-btn-a", "display:inline-block;padding:14px 32px;font-family:" + fontHead + ";font-size:16px;line-height:20px;font-weight:700;color:#FFFFFF;text-decoration:none;border-radius:10px;"},

	// Copy-paste link under a token button.
	"fallback":  {"m-muted", "margin:-8px 0 24px 0;font-family:" + fontBody + ";font-size:13px;line-height:20px;color:" + colMuted + ";"},
	"fallbackA": {"m-link", "color:" + colIndigo + ";text-decoration:underline;word-break:break-all;"},

	// Detail card: key/value rows.
	"box":      {"m-box", "margin:0 0 24px 0;background-color:" + colFog + ";border:1px solid " + colLine + ";border-radius:12px;"},
	"boxPad":   {"m-box-pad", "padding:12px 24px;"},
	"boxTitle": {"m-text", "margin:4px 0 4px 0;font-family:" + fontHead + ";font-size:14px;line-height:20px;font-weight:700;color:" + colNight + ";"},
	"rowK":     {"m-muted m-row m-row-k", "width:40%;padding:8px 16px 8px 0;font-family:" + fontBody + ";font-size:14px;line-height:20px;color:" + colMuted + ";vertical-align:top;"},
	"rowV":     {"m-text m-row", "padding:8px 0;font-family:" + fontBody + ";font-size:14px;line-height:20px;font-weight:600;color:" + colNight + ";vertical-align:top;word-break:break-word;"},
	"rowVMono": {"m-text m-row m-mono", "padding:8px 0;font-family:" + fontMono + ";font-size:14px;line-height:20px;font-weight:600;color:" + colNight + ";vertical-align:top;word-break:break-word;overflow-wrap:anywhere;"},
	"boxHint":  {"m-muted", "margin:4px 0 8px 0;font-family:" + fontBody + ";font-size:13px;line-height:20px;color:" + colMuted + ";"},

	// One-time code.
	"codeTable": {"", "margin:0 0 16px 0;"},
	"code":      {"m-box m-text m-code", "padding:16px 24px;background-color:" + colFog + ";border:1px solid " + colLine + ";border-radius:12px;font-family:" + fontMono + ";font-size:32px;line-height:40px;font-weight:700;letter-spacing:0.3em;color:" + colNight + ";text-align:center;"},

	// Notice: tinted panel with a left accent bar in the tone's colour.
	"noteTable":   {"", "margin:0 0 24px 0;"},
	"noteInfo":    notePreset("#EEF0FC", colIndigo),
	"noteSuccess": notePreset("#E6F9F3", colMint),
	"noteWarning": notePreset("#FFF5E1", colAmber),
	"noteDanger":  notePreset("#FDECEE", colRed),
	"noteTitle":   {"m-text", "font-family:" + fontHead + ";font-weight:700;color:" + colNight + ";"},

	// Numbered steps (welcome).
	"steps":    {"", "margin:0 0 8px 0;"},
	"badge":    {"m-mint", "width:28px;height:28px;background-color:" + colMint + ";border-radius:14px;font-family:" + fontHead + ";font-size:14px;line-height:28px;font-weight:800;color:" + colNight + ";text-align:center;"},
	"stepBody": {"", "padding:0 0 16px 16px;vertical-align:top;"},
	"stepH":    {"m-text", "margin:2px 0 4px 0;font-family:" + fontHead + ";font-size:16px;line-height:24px;font-weight:700;color:" + colNight + ";"},
	"stepP":    {"m-muted", "margin:0;font-family:" + fontBody + ";font-size:14px;line-height:20px;color:" + colMuted + ";"},
}

var trMonths = [...]string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"}

// kv is one row of a detail card.
type kv struct {
	K, V string
	Mono bool
}

// Mono is a rows value shown in the monospace style (IBAN, references).
type Mono string

// notice is the argument of the notice partial.
type notice struct{ Preset, Title, Text string }

var noticePresets = map[string]string{"info": "noteInfo", "success": "noteSuccess", "warning": "noteWarning", "danger": "noteDanger"}

// msoButton returns the Outlook for Windows button (VML, which keeps the
// rounded indigo box and a full-size click area) and opens the conditional
// comment that hides the HTML button from Outlook; msoEnd closes it.
func msoButton(url, label string) htmltemplate.HTML {
	w := max(200, 10*utf8.RuneCountInString(label)+72)
	e := html.EscapeString
	return htmltemplate.HTML(fmt.Sprintf( // #nosec G203 -- url and label are escaped
		`<!--[if mso]><v:roundrect xmlns:v="urn:schemas-microsoft-com:vml" xmlns:w="urn:schemas-microsoft-com:office:word" href="%s" style="height:48px;v-text-anchor:middle;width:%dpx;" arcsize="21%%" stroke="f" fillcolor="%s"><w:anchorlock/><center class="m-btn" style="color:#FFFFFF;font-family:Arial,sans-serif;font-size:16px;font-weight:bold;">%s</center></v:roundrect><![endif]--><!--[if !mso]><!-->`,
		e(url), w, colIndigo, e(label)))
}

// funcs are shared by the text and HTML templates of locale.
func funcs(locale string) map[string]any {
	tr := locale == "tr"
	unit := func(n int, trw, one, many string) string {
		if tr {
			return strconv.Itoa(n) + " " + trw
		}
		if n == 1 {
			return "1 " + one
		}
		return strconv.Itoa(n) + " " + many
	}
	and := map[bool]string{true: " ", false: " and "}[tr]
	return map[string]any{
		// dur renders a validity period: "30 dakika", "1 hour and 30 minutes", "7 days".
		"dur": func(d time.Duration) string {
			if d >= 24*time.Hour && d%(24*time.Hour) == 0 {
				return unit(int(d/(24*time.Hour)), "gün", "day", "days")
			}
			h, m := int(d/time.Hour), int(d%time.Hour/time.Minute)
			switch {
			case h == 0:
				return unit(max(m, 1), "dakika", "minute", "minutes")
			case m == 0:
				return unit(h, "saat", "hour", "hours")
			default:
				return unit(h, "saat", "hour", "hours") + and + unit(m, "dakika", "minute", "minutes")
			}
		},
		"date": func(t time.Time) string {
			if tr {
				return fmt.Sprintf("%d %s %d", t.Day(), trMonths[t.Month()-1], t.Year())
			}
			return t.Format("2 January 2006")
		},
		"datetime": func(t time.Time) string {
			zone := "UTC" + t.Format("-07:00")
			if tr {
				return fmt.Sprintf("%d %s %d, %s (%s)", t.Day(), trMonths[t.Month()-1], t.Year(), t.Format("15:04"), zone)
			}
			return fmt.Sprintf("%s at %s (%s)", t.Format("2 January 2006"), t.Format("15:04"), zone)
		},
		"money": func(m Money) string { return formatMoney(m, tr) },
		"num":   func(n int64) string { return group(n, tr) },
		"iban": func(s string) string {
			s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))
			var b strings.Builder
			for i, r := range s {
				if i > 0 && i%4 == 0 {
					b.WriteByte(' ')
				}
				b.WriteRune(r)
			}
			return b.String()
		},
		"credit": func(t string) string {
			switch {
			case t == CreditAI && tr:
				return "AI kredisi"
			case t == CreditAI:
				return "AI credits"
			case tr:
				return "mesaj kredisi"
			default:
				return "message credits"
			}
		},
		// ui writes a preset's class and style attributes; unknown names
		// fail loudly at render.
		"ui": func(name string) (htmltemplate.HTMLAttr, error) {
			p, ok := presets[name]
			if !ok {
				return "", fmt.Errorf("unknown style preset %q", name)
			}
			a := `style="` + p.style + `"`
			if p.class != "" {
				a = `class="` + p.class + `" ` + a
			}
			return htmltemplate.HTMLAttr(a), nil // #nosec G203 -- constant presets, never user data
		},
		"msoButton": msoButton,
		"msoEnd":    func() htmltemplate.HTML { return "<!--<![endif]-->" },
		"msoHead": func() htmltemplate.HTML {
			return `<!--[if mso]><noscript><xml><o:OfficeDocumentSettings><o:AllowPNG/><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml></noscript>` +
				`<style>table,td,p,a,h1,span{font-family:Arial,Helvetica,sans-serif !important;}</style><![endif]-->`
		},
		"dict": func(kvs ...any) (map[string]any, error) {
			if len(kvs)%2 != 0 {
				return nil, fmt.Errorf("dict: odd number of arguments")
			}
			m := make(map[string]any, len(kvs)/2)
			for i := 0; i < len(kvs); i += 2 {
				k, ok := kvs[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key %v is not a string", kvs[i])
				}
				m[k] = kvs[i+1]
			}
			return m, nil
		},
		// opt returns s when cond holds, otherwise "" (rows then skips it).
		"opt": func(cond bool, s string) string {
			if cond {
				return s
			}
			return ""
		},
		// mono marks a rows value for the monospace style.
		"mono": func(s string) Mono { return Mono(s) },
		// rows builds label/value pairs, skipping empty values.
		"rows": func(pairs ...any) ([]kv, error) {
			var out []kv
			for i := 0; i+1 < len(pairs); i += 2 {
				k, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("rows: label %v is not a string", pairs[i])
				}
				r := kv{K: k}
				switch v := pairs[i+1].(type) {
				case string:
					r.V = v
				case Mono:
					r.V, r.Mono = string(v), true
				default:
					return nil, fmt.Errorf("rows: value of %q is a %T", k, pairs[i+1])
				}
				if strings.TrimSpace(r.V) != "" {
					out = append(out, r)
				}
			}
			return out, nil
		},
		// notice builds the argument of the notice partial: tone is info,
		// success, warning or danger; title may be empty.
		"notice": func(tone, title, text string) (notice, error) {
			p, ok := noticePresets[tone]
			if !ok {
				return notice{}, fmt.Errorf("notice: unknown tone %q", tone)
			}
			return notice{Preset: p, Title: title, Text: text}, nil
		},
	}
}

func group(n int64, tr bool) string {
	sep := map[bool]string{true: ".", false: ","}[tr]
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

var currencySymbols = map[string]string{"TRY": "₺", "USD": "$", "EUR": "€", "GBP": "£"}

// formatMoney: tr "1.250,00 ₺", en "₺1,250.00"; unknown codes as "1,250.00 CHF".
func formatMoney(m Money, tr bool) string {
	minor := m.Minor
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	dec := map[bool]string{true: ",", false: "."}[tr]
	amount := sign + group(minor/100, tr) + dec + fmt.Sprintf("%02d", minor%100)
	sym, ok := currencySymbols[m.Currency]
	switch {
	case !ok:
		return amount + " " + m.Currency
	case tr:
		return amount + " " + sym
	default:
		return sym + amount
	}
}
