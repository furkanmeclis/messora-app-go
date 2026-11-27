package templates

import (
	"fmt"
	htmltemplate "html/template"
	"strconv"
	"strings"
	"time"
)

// chrome holds the layout strings shared by every kind.
type chrome struct {
	Hello          string // followed by the name and a comma
	HelloAnon      string
	With           string // "<TenantName> · Messora ile"
	ButtonFallback string
	NeedHelp       string
	Support        string
	Unsubscribe    string
	Automated      string
	// Bank transfer block.
	BankTitle     string
	BankName      string
	AccountHolder string
	Reference     string
	ReferenceHint string
}

var chromes = map[string]chrome{
	"tr": {
		Hello:          "Merhaba",
		HelloAnon:      "Merhaba,",
		With:           "Messora ile",
		ButtonFallback: "Buton çalışmazsa bu bağlantıyı tarayıcınıza yapıştırın:",
		NeedHelp:       "Bir sorunuz mu var?",
		Support:        "Destek ekibimize yazın",
		Unsubscribe:    "Bu bildirimleri kapat",
		Automated:      "Bu e-posta otomatik gönderildi.",
		BankTitle:      "Havale / EFT bilgileri",
		BankName:       "Banka",
		AccountHolder:  "Alıcı",
		Reference:      "Açıklama",
		ReferenceHint:  "Ödemenizi hızla eşleştirebilmemiz için açıklama alanına yukarıdaki kodu yazın.",
	},
	"en": {
		Hello:          "Hi",
		HelloAnon:      "Hi there,",
		With:           "with Messora",
		ButtonFallback: "If the button doesn't work, paste this link into your browser:",
		NeedHelp:       "Questions?",
		Support:        "Write to our support team",
		Unsubscribe:    "Turn off these notifications",
		Automated:      "This e-mail was sent automatically.",
		BankTitle:      "Bank transfer details",
		BankName:       "Bank",
		AccountHolder:  "Account holder",
		Reference:      "Reference",
		ReferenceHint:  "Please put the reference above in the transfer description so we can match your payment quickly.",
	},
}

// Brand palette and type. Changing these changes every e-mail.
const (
	colFog    = "#F4F6FB"
	colNight  = "#0B1220"
	colIndigo = "#3B4CCA"
	colMint   = "#2DD4A7"
	colMuted  = "#4A5468"
	colLine   = "#E3E7F0"
	fontHead  = "'Plus Jakarta Sans', Inter, -apple-system, 'Segoe UI', Roboto, Arial, sans-serif"
	fontBody  = "Inter, -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif"
	fontMono  = "'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"
)

// styles are the inline style presets used by the HTML templates via
// {{css "name"}}. Inline styles are the only thing every client keeps.
var styles = map[string]string{
	"body":       "margin:0;padding:0;width:100%;background-color:" + colFog + ";-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%;",
	"preheader":  "display:none;font-size:1px;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;mso-hide:all;",
	"outer":      "background-color:" + colFog + ";",
	"wrap":       "width:100%;max-width:600px;",
	"header":     "padding:0 8px 20px 8px;",
	"mark":       "width:28px;height:28px;background-color:" + colIndigo + ";border-radius:8px;",
	"dot":        "width:10px;height:10px;background-color:" + colMint + ";border-radius:5px;font-size:0;line-height:0;",
	"word":       "padding-left:10px;font-family:" + fontHead + ";font-size:24px;line-height:28px;font-weight:800;letter-spacing:-0.03em;color:" + colNight + ";",
	"tenant":     "font-family:" + fontHead + ";font-size:20px;line-height:28px;font-weight:700;color:" + colNight + ";",
	"with":       "padding-left:8px;font-family:" + fontBody + ";font-size:14px;line-height:28px;color:" + colMuted + ";",
	"card":       "background-color:#FFFFFF;border:1px solid " + colLine + ";border-radius:16px;",
	"accent":     "height:4px;line-height:4px;font-size:0;background-color:" + colIndigo + ";border-radius:16px 16px 0 0;",
	"accentMint": "height:4px;line-height:4px;font-size:0;background-color:" + colMint + ";border-radius:0 16px 0 0;",
	"cardPad":    "padding:36px 40px 32px 40px;",
	"h1":         "margin:0 0 16px 0;font-family:" + fontHead + ";font-size:24px;line-height:32px;font-weight:700;color:" + colNight + ";",
	"h2":         "margin:0 0 8px 0;font-family:" + fontHead + ";font-size:16px;line-height:24px;font-weight:700;color:" + colNight + ";",
	"p":          "margin:0 0 16px 0;font-family:" + fontBody + ";font-size:16px;line-height:26px;color:" + colNight + ";",
	"muted":      "margin:0 0 16px 0;font-family:" + fontBody + ";font-size:14px;line-height:22px;color:" + colMuted + ";",
	"hint":       "margin:8px 0 0 0;font-family:" + fontBody + ";font-size:13px;line-height:20px;color:" + colMuted + ";",
	"link":       "color:" + colIndigo + ";text-decoration:underline;",
	"btnTable":   "margin:8px 0 24px 0;",
	"btnCell":    "border-radius:10px;background-color:" + colIndigo + ";",
	"btnLink":    "display:inline-block;padding:14px 28px;font-family:" + fontHead + ";font-size:16px;line-height:20px;font-weight:700;color:#FFFFFF;text-decoration:none;border-radius:10px;",
	"fallback":   "margin:0 0 16px 0;font-family:" + fontBody + ";font-size:13px;line-height:20px;color:" + colMuted + ";",
	"urlLink":    "color:" + colIndigo + ";text-decoration:underline;word-break:break-all;",
	"codeTable":  "margin:8px 0 24px 0;",
	"code":       "padding:16px 28px;background-color:" + colFog + ";border:1px solid " + colLine + ";border-radius:12px;font-family:" + fontMono + ";font-size:30px;line-height:36px;font-weight:700;letter-spacing:0.25em;color:" + colNight + ";",
	"box":        "background-color:" + colFog + ";border:1px solid " + colLine + ";border-radius:12px;",
	"boxPad":     "padding:16px 20px;",
	"boxTitle":   "margin:0 0 8px 0;font-family:" + fontHead + ";font-size:14px;line-height:20px;font-weight:700;color:" + colNight + ";",
	"rowLabel":   "padding:6px 16px 6px 0;font-family:" + fontBody + ";font-size:14px;line-height:20px;color:" + colMuted + ";vertical-align:top;white-space:nowrap;",
	"rowValue":   "padding:6px 0;font-family:" + fontBody + ";font-size:14px;line-height:20px;font-weight:600;color:" + colNight + ";vertical-align:top;",
	"mono":       "font-family:" + fontMono + ";letter-spacing:0.02em;",
	"note":       "padding:12px 16px;background-color:" + colFog + ";border-left:3px solid " + colMint + ";border-radius:0 8px 8px 0;font-family:" + fontBody + ";font-size:14px;line-height:22px;color:" + colNight + ";",
	"noteTable":  "margin:8px 0 16px 0;",
	"badge":      "width:28px;height:28px;background-color:" + colMint + ";border-radius:14px;font-family:" + fontHead + ";font-size:14px;line-height:28px;font-weight:800;color:" + colNight + ";text-align:center;",
	"stepText":   "padding:0 0 20px 14px;vertical-align:top;",
	"footer":     "padding:24px 16px 8px 16px;font-family:" + fontBody + ";font-size:12px;line-height:19px;color:" + colMuted + ";text-align:center;",
	"footerP":    "margin:0 0 8px 0;font-family:" + fontBody + ";font-size:12px;line-height:19px;color:" + colMuted + ";",
	"footerLink": "color:" + colMuted + ";text-decoration:underline;",
	"spacer":     "height:24px;line-height:24px;font-size:0;",
}

var trMonths = [...]string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"}

// kv is one row of a details table.
type kv struct{ K, V string }

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
		// css returns a style preset; unknown names fail loudly at render.
		"css": func(name string) (htmltemplate.CSS, error) {
			s, ok := styles[name]
			if !ok {
				return "", fmt.Errorf("unknown style %q", name)
			}
			return htmltemplate.CSS(s), nil // #nosec G203 -- constant presets, never user data
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
		// rows builds label/value pairs, skipping empty values.
		"rows": func(pairs ...string) []kv {
			var out []kv
			for i := 0; i+1 < len(pairs); i += 2 {
				if strings.TrimSpace(pairs[i+1]) != "" {
					out = append(out, kv{pairs[i], pairs[i+1]})
				}
			}
			return out
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
