package ai

import (
	"fmt"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Types of a field the AI collects; the value the model extracted is
// checked against it before it is stored.
const (
	FieldText   = "text"
	FieldEmail  = "email"
	FieldPhone  = "phone"
	FieldSelect = "select"
	FieldNumber = "number"
	FieldDate   = "date"
)

// FieldTypes lists the field types.
var FieldTypes = []string{FieldText, FieldEmail, FieldPhone, FieldSelect, FieldNumber, FieldDate}

// Sources of a contact field's value (contact_field_sources.source).
const (
	SourceAI     = "ai"
	SourceUser   = "user"
	SourceImport = "import"
	SourceAPI    = "api"
)

// maxFieldValue bounds a stored value (the contacts' var columns hold 500).
const maxFieldValue = 500

// sensitiveTokens are words of special categories of personal data (KVKK
// art. 6, GDPR art. 9) and secrets. A field whose key or label contains one
// cannot be collected by the AI: the platform never asks customers for
// them on WhatsApp.
var sensitiveTokens = []string{
	// health, genetics, biometrics
	"health", "medical", "diagnosis", "disease", "illness", "genetic", "biometric", "fingerprint", "disability",
	"sağlık", "saglik", "hastalık", "hastalik", "teşhis", "teshis", "tanı", "genetik", "biyometrik", "parmak", "engellilik", "engel",
	// beliefs, origin, politics, sex life, unions, criminal records
	"religion", "religious", "belief", "ethnic", "ethnicity", "race", "racial", "political", "politics", "sexual", "sexuality",
	"union", "criminal", "conviction",
	"din", "dini", "inanç", "inanc", "mezhep", "etnik", "ırk", "irk", "siyasi", "politik", "cinsel", "sendika", "sabıka", "sabika", "mahkumiyet",
	// identity numbers and secrets
	"password", "passcode", "pin", "cvv", "cvc", "ssn", "passport",
	"şifre", "sifre", "parola", "tckn", "kimlik", "pasaport",
}

// sensitivePairs are two-word phrases (both words present).
var sensitivePairs = [][2]string{{"credit", "card"}, {"card", "number"}, {"kredi", "kart"}, {"kredi", "kartı"}, {"kart", "numarası"}, {"tc", "no"}, {"tc", "kimlik"}}

func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// SensitiveField reports whether a field's key or label names a special
// category of personal data or a secret.
func SensitiveField(key, label string) bool {
	ts := append(tokens(strings.TrimPrefix(key, "attr:")), tokens(label)...)
	for _, t := range ts {
		if slices.Contains(sensitiveTokens, t) {
			return true
		}
	}
	for _, p := range sensitivePairs {
		if slices.Contains(ts, p[0]) && slices.Contains(ts, p[1]) {
			return true
		}
	}
	return false
}

// TypeOf is the effective type of a field (email for the email field,
// text when none is set).
func (f DataField) TypeOf() string {
	if f.Key == "email" {
		return FieldEmail
	}
	if f.Type == "" {
		return FieldText
	}
	return f.Type
}

// normalizeField validates a field's type and options.
func normalizeField(f *DataField) error {
	f.Type = strings.TrimSpace(f.Type)
	if f.Type == "" {
		f.Type = FieldText
	}
	if f.Key == "email" {
		f.Type = FieldEmail
	}
	if !slices.Contains(FieldTypes, f.Type) {
		return &FieldError{"data_collection.fields", fmt.Sprintf("%q has an unknown type %q.", f.Key, f.Type)}
	}
	if f.Key == "name" && f.Type != FieldText {
		return &FieldError{"data_collection.fields", "The name field is text."}
	}
	if f.Type != FieldSelect {
		f.Options = nil
		return nil
	}
	opts := []string{}
	for _, o := range f.Options {
		o = strings.TrimSpace(o)
		if o == "" || slices.ContainsFunc(opts, func(x string) bool { return strings.EqualFold(x, o) }) {
			continue
		}
		if len([]rune(o)) > 100 {
			return &FieldError{"data_collection.fields", "Options are limited to 100 characters."}
		}
		opts = append(opts, o)
	}
	if len(opts) == 0 || len(opts) > 50 {
		return &FieldError{"data_collection.fields", fmt.Sprintf("%q needs 1–50 options.", f.Key)}
	}
	f.Options = opts
	return nil
}

// ValidateValue checks a value the model extracted for field f and returns
// it normalised (trimmed e-mail in lower case, E.164 phone number, the
// option as configured, a plain number, a YYYY-MM-DD date). ok is false
// when the value does not fit the type; it is then not stored.
func ValidateValue(f DataField, v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" || len([]rune(v)) > maxFieldValue {
		return "", false
	}
	switch f.TypeOf() {
	case FieldEmail:
		a, err := mail.ParseAddress(v)
		if err != nil || a.Address != v || !strings.Contains(v[strings.LastIndex(v, "@")+1:], ".") || len(v) > 254 {
			return "", false
		}
		return strings.ToLower(v), true
	case FieldPhone:
		return normalizePhone(v)
	case FieldSelect:
		for _, o := range f.Options {
			if strings.EqualFold(o, v) {
				return o, true
			}
		}
		return "", false
	case FieldNumber:
		n := strings.ReplaceAll(v, ",", ".")
		x, err := strconv.ParseFloat(n, 64)
		if err != nil || strings.ContainsAny(n, "eEinfINF") {
			return "", false
		}
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case FieldDate:
		for _, layout := range []string{time.DateOnly, "02.01.2006", "02/01/2006"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.Format(time.DateOnly), true
			}
		}
		return "", false
	}
	return v, true
}

// normalizePhone accepts an international number written with spaces,
// dashes, dots or brackets ("+90 555 111 22 33", "0090…") and returns it
// in E.164.
func normalizePhone(v string) (string, bool) {
	var b strings.Builder
	for i, r := range v {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return "", false
		}
	}
	s := b.String()
	if rest, ok := strings.CutPrefix(s, "00"); ok {
		s = "+" + rest
	}
	if !strings.HasPrefix(s, "+") || len(s) < 8 || len(s) > 16 || s[1] == '0' {
		return "", false
	}
	return s, true
}

// FieldSource is where a contact field's current value came from.
type FieldSource struct {
	Source   string
	Verified bool
}

// MayAIWrite reports whether the AI may store a value in a field whose
// current value is current with provenance src (nil = no record, i.e. set
// by people, an import or the API before provenance existed). The AI fills
// empty fields and replaces only its own unverified values; it never
// overwrites a value a person, an import or the API set, nor one a person
// verified.
func MayAIWrite(current string, src *FieldSource) bool {
	if strings.TrimSpace(current) == "" {
		return true
	}
	return src != nil && src.Source == SourceAI && !src.Verified
}

// Progress is how complete a contact's profile is against the required
// fields of a profile's data collection.
type Progress struct {
	Required int      `json:"required"`
	Filled   int      `json:"filled"`
	Missing  []string `json:"missing"`
}

// Complete reports whether every required field is filled.
func (p Progress) Complete() bool { return p.Filled >= p.Required }

// ProgressOf computes the progress from the contact's current values.
func ProgressOf(fields []DataField, values map[string]string) Progress {
	p := Progress{Missing: []string{}}
	for _, f := range fields {
		if !f.Required {
			continue
		}
		p.Required++
		if strings.TrimSpace(values[f.Key]) != "" {
			p.Filled++
		} else {
			p.Missing = append(p.Missing, f.Key)
		}
	}
	return p
}
