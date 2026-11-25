package channels

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Limits Meta documents for template components (docs/WHATSAPP.md,
// "Template components"). Lengths count characters, not bytes.
const (
	maxHeaderText      = 60
	maxBodyText        = 1024
	maxLTOBodyText     = 600
	maxCardBodyText    = 160
	maxFooterText      = 60
	maxButtons         = 10
	maxButtonText      = 25
	maxVoiceCallText   = 20
	maxURL             = 2000
	maxPhoneNumber     = 20
	maxCopyCode        = 20
	maxLTOCopyCode     = 15
	maxLTOText         = 16
	minCards, maxCards = 2, 10
	maxCardButtons     = 2
	maxSupportedApps   = 5
	maxPackageName     = 224
)

var (
	componentVar  = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)
	namedVar      = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	positionalVar = regexp.MustCompile(`^[1-9][0-9]*$`)
	urlVarAtEnd   = regexp.MustCompile(`\{\{\s*[A-Za-z0-9_]+\s*\}\}$`)
	packageName   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)
	signatureHash = regexp.MustCompile(`^[a-zA-Z0-9+/=]{11}$`)
)

type tplApp struct {
	PackageName   string `json:"package_name"`
	SignatureHash string `json:"signature_hash"`
}

type tplButton struct {
	Type                 string          `json:"type"`
	Text                 *string         `json:"text"`
	URL                  string          `json:"url"`
	PhoneNumber          string          `json:"phone_number"`
	Example              json.RawMessage `json:"example"`
	OTPType              string          `json:"otp_type"`
	AutofillText         *string         `json:"autofill_text"`
	SupportedApps        []tplApp        `json:"supported_apps"`
	ZeroTapTermsAccepted *bool           `json:"zero_tap_terms_accepted"`
	FlowID               json.RawMessage `json:"flow_id"`
	FlowName             string          `json:"flow_name"`
	FlowJSON             string          `json:"flow_json"`
	FlowAction           string          `json:"flow_action"`
	TTLMinutes           *int            `json:"ttl_minutes"`
}

type tplComponent struct {
	Type                  string          `json:"type"`
	Format                string          `json:"format"`
	Text                  *string         `json:"text"`
	Example               json.RawMessage `json:"example"`
	Buttons               []tplButton     `json:"buttons"`
	Cards                 []tplCard       `json:"cards"`
	LimitedTimeOffer      *tplLTO         `json:"limited_time_offer"`
	CodeExpirationMinutes *int            `json:"code_expiration_minutes"`
}

type tplCard struct {
	Components []tplComponent `json:"components"`
}

type tplLTO struct {
	Text          string `json:"text"`
	HasExpiration *bool  `json:"has_expiration"`
}

func (c tplComponent) text() string {
	if c.Text == nil {
		return ""
	}
	return *c.Text
}

func chars(s string) int { return utf8.RuneCountInString(s) }

// templateCheck carries what one validation needs.
type templateCheck struct {
	category string // MARKETING | UTILITY | AUTHENTICATION
	named    bool
}

// ValidateComponents checks a template's components against the limits Meta
// documents, so a mistake is reported on its field instead of as Meta's
// generic "invalid parameter". Catalog buttons (MPM, SPM, CATALOG) are
// refused: they need a connected catalog, which Messora does not manage.
func ValidateComponents(category, parameterFormat string, raw json.RawMessage) error {
	var comps []tplComponent
	if err := json.Unmarshal(raw, &comps); err != nil {
		return apperr.Validation("components", "The components are malformed: "+jsonProblem(err))
	}
	if len(comps) == 0 {
		return apperr.Validation("components", "At least one component (a BODY) is required.")
	}
	ck := templateCheck{category: strings.ToUpper(category), named: strings.EqualFold(parameterFormat, "NAMED")}
	seen := map[string]int{}
	for i, c := range comps {
		t := strings.ToUpper(c.Type)
		if !slices.Contains([]string{"HEADER", "BODY", "FOOTER", "BUTTONS", "CAROUSEL", "LIMITED_TIME_OFFER"}, t) {
			return apperr.Validation(fmt.Sprintf("components.%d.type", i), fmt.Sprintf("Unknown component type %q.", c.Type))
		}
		if _, dup := seen[t]; dup {
			return apperr.Validation(fmt.Sprintf("components.%d.type", i), fmt.Sprintf("A template has at most one %s component.", t))
		}
		seen[t] = i
	}
	if _, ok := seen["BODY"]; !ok {
		return apperr.Validation("components", "A BODY component is required.")
	}
	if ck.category == "AUTHENTICATION" {
		return ck.authentication(comps, seen)
	}
	_, lto := seen["LIMITED_TIME_OFFER"]
	_, carousel := seen["CAROUSEL"]
	if (lto || carousel) && ck.category != "MARKETING" {
		kind := "Limited-time offer"
		if carousel {
			kind = "Carousel"
		}
		return apperr.Validation("category", kind+" templates must be in the MARKETING category.")
	}
	if lto && carousel {
		return apperr.Validation("components", "A template is either a carousel or a limited-time offer, not both.")
	}
	for i, c := range comps {
		f := fmt.Sprintf("components.%d", i)
		var err error
		switch strings.ToUpper(c.Type) {
		case "HEADER":
			err = ck.header(f, c, lto, carousel)
		case "BODY":
			limit := maxBodyText
			if lto {
				limit = maxLTOBodyText
			}
			err = ck.body(f, c, limit, true)
		case "FOOTER":
			err = ck.footer(f, c, lto, carousel)
		case "BUTTONS":
			err = ck.buttons(f, c.Buttons, lto, carousel)
		case "LIMITED_TIME_OFFER":
			err = ck.limitedTimeOffer(f, c)
		case "CAROUSEL":
			err = ck.carousel(f, c)
		}
		if err != nil {
			return err
		}
	}
	if lto {
		if _, ok := seen["BUTTONS"]; !ok {
			return apperr.Validation("components", "A limited-time offer needs a URL button (and optionally a copy code button).")
		}
	}
	return nil
}

func jsonProblem(err error) string {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) && te.Field != "" {
		return fmt.Sprintf("%s should be a %s.", te.Field, te.Type.Kind())
	}
	return "expected an array of component objects."
}

// variables checks the {{…}} of text against the parameter format and
// returns them in order of appearance (each once).
func (ck templateCheck) variables(field, text string) ([]string, error) {
	var out []string
	for _, m := range componentVar.FindAllStringSubmatch(text, -1) {
		v := m[1]
		if ck.named && !namedVar.MatchString(v) {
			return nil, apperr.Validation(field, fmt.Sprintf("{{%s}}: named variables use lowercase letters, digits and underscores, e.g. {{first_name}}.", v))
		}
		if !ck.named && !positionalVar.MatchString(v) {
			return nil, apperr.Validation(field, fmt.Sprintf("{{%s}}: numbered templates use {{1}}, {{2}} …; switch the variable format to named for {{%s}}.", v, v))
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if !ck.named {
		nums := make([]int, len(out))
		for i, v := range out {
			nums[i], _ = strconv.Atoi(v)
		}
		slices.Sort(nums)
		for i, n := range nums {
			if n != i+1 {
				return nil, apperr.Validation(field, "Numbered variables must run {{1}}, {{2}}, {{3}} … without gaps.")
			}
		}
	}
	return out, nil
}

type namedExample struct {
	ParamName string `json:"param_name"`
	Example   string `json:"example"`
}

// examples checks that every variable of a header or body has Meta's
// required sample value. key is "header_text" or "body_text".
func (ck templateCheck) examples(field string, vars []string, example json.RawMessage, key string) error {
	if len(vars) == 0 {
		return nil
	}
	var ex map[string]json.RawMessage
	_ = json.Unmarshal(example, &ex)
	missing := apperr.Validation(field+".example", "Meta needs a sample value for every variable.")
	if ck.named {
		var list []namedExample
		if json.Unmarshal(ex[key+"_named_params"], &list) != nil {
			return missing
		}
		have := map[string]bool{}
		for _, e := range list {
			if strings.TrimSpace(e.Example) != "" {
				have[e.ParamName] = true
			}
		}
		for _, v := range vars {
			if !have[v] {
				return apperr.Validation(field+".example", fmt.Sprintf("Meta needs a sample value for {{%s}}.", v))
			}
		}
		return nil
	}
	var values []string
	var nested [][]string
	switch {
	case json.Unmarshal(ex[key], &nested) == nil && len(nested) > 0:
		values = nested[0]
	case json.Unmarshal(ex[key], &values) == nil:
	default:
		var one string
		if json.Unmarshal(ex[key], &one) != nil {
			return missing
		}
		values = []string{one}
	}
	if len(values) != len(vars) {
		return apperr.Validation(field+".example", fmt.Sprintf("Give %d sample value(s), one per variable (got %d).", len(vars), len(values)))
	}
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return missing
		}
	}
	return nil
}

func headerHandle(example json.RawMessage) bool {
	var ex struct {
		HeaderHandle []string `json:"header_handle"`
	}
	_ = json.Unmarshal(example, &ex)
	return len(ex.HeaderHandle) > 0 && strings.TrimSpace(ex.HeaderHandle[0]) != ""
}

func (ck templateCheck) header(f string, c tplComponent, lto, carousel bool) error {
	if carousel {
		return apperr.Validation(f, "A carousel has no message header; each card has its own image or video.")
	}
	format := strings.ToUpper(c.Format)
	if format == "" {
		format = "TEXT"
	}
	if lto && format != "IMAGE" && format != "VIDEO" {
		return apperr.Validation(f+".format", "A limited-time offer's header is an image or a video.")
	}
	switch format {
	case "TEXT":
		t := strings.TrimSpace(c.text())
		if t == "" {
			return apperr.Validation(f+".text", "A text header needs its text.")
		}
		if chars(t) > maxHeaderText {
			return apperr.Validation(f+".text", fmt.Sprintf("A text header is at most %d characters.", maxHeaderText))
		}
		vars, err := ck.variables(f+".text", t)
		if err != nil {
			return err
		}
		if len(vars) > 1 {
			return apperr.Validation(f+".text", "A text header has at most one variable.")
		}
		return ck.examples(f, vars, c.Example, "header_text")
	case "IMAGE", "VIDEO", "DOCUMENT":
		if !headerHandle(c.Example) {
			return apperr.Validation(f+".example", "Meta reviews media headers with a sample file: upload one.")
		}
	case "LOCATION":
		if ck.category != "UTILITY" && ck.category != "MARKETING" {
			return apperr.Validation(f+".format", "Location headers are for utility and marketing templates only.")
		}
	case "GIF":
		return apperr.Validation(f+".format", "GIF headers need Meta's Marketing Messages API, which Messora does not use; use a video.")
	default:
		return apperr.Validation(f+".format", "A header is TEXT, IMAGE, VIDEO, DOCUMENT or LOCATION.")
	}
	return nil
}

func (ck templateCheck) body(f string, c tplComponent, limit int, required bool) error {
	t := c.text()
	if strings.TrimSpace(t) == "" {
		if required {
			return apperr.Validation(f+".text", "The body text is required.")
		}
		return nil
	}
	if chars(t) > limit {
		return apperr.Validation(f+".text", fmt.Sprintf("The body is at most %d characters here.", limit))
	}
	vars, err := ck.variables(f+".text", t)
	if err != nil {
		return err
	}
	return ck.examples(f, vars, c.Example, "body_text")
}

func (ck templateCheck) footer(f string, c tplComponent, lto, carousel bool) error {
	if lto {
		return apperr.Validation(f, "Limited-time offer templates cannot have a footer.")
	}
	if carousel {
		return apperr.Validation(f, "Carousel templates have a message body and cards only, no footer.")
	}
	t := c.text()
	if strings.TrimSpace(t) == "" {
		return apperr.Validation(f+".text", "A footer needs its text (or remove the footer).")
	}
	if chars(t) > maxFooterText {
		return apperr.Validation(f+".text", fmt.Sprintf("The footer is at most %d characters.", maxFooterText))
	}
	if componentVar.MatchString(t) {
		return apperr.Validation(f+".text", "The footer cannot contain variables.")
	}
	return nil
}

// buttonLimits are the per-type maxima of a template's buttons. VOICE_CALL
// and FLOW have no documented maximum; one each is assumed.
var buttonLimits = map[string]int{"QUICK_REPLY": 10, "URL": 2, "PHONE_NUMBER": 1, "COPY_CODE": 1, "VOICE_CALL": 1, "FLOW": 1}

var buttonNames = map[string]string{"QUICK_REPLY": "quick reply", "URL": "URL", "PHONE_NUMBER": "phone number",
	"COPY_CODE": "copy code", "VOICE_CALL": "WhatsApp call", "FLOW": "Flow"}

func (ck templateCheck) buttons(f string, bs []tplButton, lto, carousel bool) error {
	if carousel {
		return apperr.Validation(f, "In a carousel, buttons belong to the cards.")
	}
	if len(bs) == 0 {
		return apperr.Validation(f+".buttons", "Add at least one button (or remove the buttons).")
	}
	if len(bs) > maxButtons {
		return apperr.Validation(f+".buttons", fmt.Sprintf("A template has at most %d buttons.", maxButtons))
	}
	count := map[string]int{}
	groups := 0
	prevQR := false
	for j, b := range bs {
		bf := fmt.Sprintf("%s.buttons.%d", f, j)
		t := strings.ToUpper(b.Type)
		if lto && t != "COPY_CODE" && t != "URL" {
			return apperr.Validation(bf+".type", "A limited-time offer has a copy code button and a URL button only.")
		}
		if err := ck.button(bf, t, b, lto); err != nil {
			return err
		}
		count[t]++
		if count[t] > buttonLimits[t] {
			return apperr.Validation(bf+".type", fmt.Sprintf("A template has at most %d %s button(s).", buttonLimits[t], buttonNames[t]))
		}
		qr := t == "QUICK_REPLY"
		if j == 0 || qr != prevQR {
			groups++
		}
		prevQR = qr
	}
	if count["QUICK_REPLY"] > 0 && groups > 2 {
		return apperr.Validation(f+".buttons", "Keep quick reply buttons together: all of them before or after the other buttons.")
	}
	if lto && count["URL"] != 1 {
		return apperr.Validation(f+".buttons", "A limited-time offer needs exactly one URL button.")
	}
	return nil
}

func label(bf string, b tplButton, limit int, required bool) error {
	if b.Text == nil || strings.TrimSpace(*b.Text) == "" {
		if required {
			return apperr.Validation(bf+".text", "The button needs a label.")
		}
		return nil
	}
	if chars(*b.Text) > limit {
		return apperr.Validation(bf+".text", fmt.Sprintf("A button label is at most %d characters.", limit))
	}
	return nil
}

// button checks one button of type t (upper case).
func (ck templateCheck) button(bf, t string, b tplButton, lto bool) error {
	switch t {
	case "QUICK_REPLY":
		return label(bf, b, maxButtonText, true)
	case "URL":
		if err := label(bf, b, maxButtonText, true); err != nil {
			return err
		}
		return urlButton(bf, b)
	case "PHONE_NUMBER":
		if err := label(bf, b, maxButtonText, true); err != nil {
			return err
		}
		p := strings.TrimSpace(b.PhoneNumber)
		if p == "" || chars(p) > maxPhoneNumber {
			return apperr.Validation(bf+".phone_number", fmt.Sprintf("Enter the phone number to call (at most %d characters).", maxPhoneNumber))
		}
	case "COPY_CODE":
		var ex string
		limit := maxCopyCode
		if lto {
			limit = maxLTOCopyCode
		}
		if json.Unmarshal(b.Example, &ex) != nil || strings.TrimSpace(ex) == "" {
			return apperr.Validation(bf+".example", "A copy code button needs a sample code.")
		}
		if chars(ex) > limit {
			return apperr.Validation(bf+".example", fmt.Sprintf("The code is at most %d characters.", limit))
		}
	case "VOICE_CALL":
		if err := label(bf, b, maxVoiceCallText, false); err != nil {
			return err
		}
		if b.TTLMinutes != nil && (*b.TTLMinutes < 1440 || *b.TTLMinutes > 43200) {
			return apperr.Validation(bf+".ttl_minutes", "The call button stays active between 1440 minutes (1 day) and 43200 (30 days).")
		}
	case "FLOW":
		if err := label(bf, b, maxButtonText, true); err != nil {
			return err
		}
		refs := 0
		if id := strings.Trim(strings.TrimSpace(string(b.FlowID)), `"`); id != "" && id != "null" {
			refs++
		}
		if strings.TrimSpace(b.FlowName) != "" {
			refs++
		}
		if strings.TrimSpace(b.FlowJSON) != "" {
			refs++
		}
		if refs != 1 {
			return apperr.Validation(bf+".flow_id", "Name the Flow with exactly one of flow_id, flow_name or flow_json.")
		}
		if a := strings.ToLower(b.FlowAction); a != "" && a != "navigate" && a != "data_exchange" {
			return apperr.Validation(bf+".flow_action", "flow_action is navigate or data_exchange.")
		}
	case "OTP":
		return apperr.Validation(bf+".type", "One-time password buttons are for AUTHENTICATION templates only.")
	case "MPM", "SPM", "CATALOG", "PRODUCT":
		return apperr.Validation(bf+".type", fmt.Sprintf("%s buttons require a connected product catalog, which Messora does not support.", t))
	default:
		return apperr.Validation(bf+".type", fmt.Sprintf("Unknown button type %q.", b.Type))
	}
	return nil
}

func urlButton(bf string, b tplButton) error {
	raw := strings.TrimSpace(b.URL)
	if raw == "" || chars(raw) > maxURL {
		return apperr.Validation(bf+".url", fmt.Sprintf("Enter the web address (at most %d characters).", maxURL))
	}
	u, err := url.Parse(componentVar.ReplaceAllString(raw, "x"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return apperr.Validation(bf+".url", "Enter a full web address starting with https://.")
	}
	n := len(componentVar.FindAllString(raw, -1))
	if n == 0 {
		return nil
	}
	if n > 1 || !urlVarAtEnd.MatchString(raw) {
		return apperr.Validation(bf+".url", "A URL has at most one variable, at its very end (e.g. https://shop.example/{{1}}).")
	}
	var ex []string
	if json.Unmarshal(b.Example, &ex) != nil || len(ex) == 0 || strings.TrimSpace(ex[0]) == "" {
		return apperr.Validation(bf+".example", "Meta needs a sample value for the URL's variable.")
	}
	return nil
}

func (ck templateCheck) limitedTimeOffer(f string, c tplComponent) error {
	o := c.LimitedTimeOffer
	if o == nil || strings.TrimSpace(o.Text) == "" {
		return apperr.Validation(f+".limited_time_offer.text", "The offer needs its short text (e.g. \"Expiring offer!\").")
	}
	if chars(o.Text) > maxLTOText {
		return apperr.Validation(f+".limited_time_offer.text", fmt.Sprintf("The offer text is at most %d characters.", maxLTOText))
	}
	return nil
}

// carousel checks a media card carousel: 2–10 cards of the same shape, each
// with an image or video header, an optional body (on all cards or none)
// and up to two quick reply, URL or phone number buttons.
func (ck templateCheck) carousel(f string, c tplComponent) error {
	if len(c.Cards) < minCards || len(c.Cards) > maxCards {
		return apperr.Validation(f+".cards", fmt.Sprintf("A carousel has %d to %d cards.", minCards, maxCards))
	}
	type shape struct {
		header  string
		body    bool
		buttons string
	}
	var first shape
	for k, card := range c.Cards {
		cf := fmt.Sprintf("%s.cards.%d", f, k)
		var sh shape
		seen := map[string]bool{}
		for m, cc := range card.Components {
			mf := fmt.Sprintf("%s.components.%d", cf, m)
			t := strings.ToUpper(cc.Type)
			if seen[t] {
				return apperr.Validation(mf+".type", fmt.Sprintf("A card has at most one %s component.", t))
			}
			seen[t] = true
			switch t {
			case "HEADER":
				sh.header = strings.ToUpper(cc.Format)
				if sh.header != "IMAGE" && sh.header != "VIDEO" {
					return apperr.Validation(mf+".format", "A card's header is an image or a video.")
				}
				if !headerHandle(cc.Example) {
					return apperr.Validation(mf+".example", "Upload a sample image or video for the card.")
				}
			case "BODY":
				if err := ck.body(mf, cc, maxCardBodyText, true); err != nil {
					return err
				}
				sh.body = true
			case "BUTTONS":
				if len(cc.Buttons) == 0 || len(cc.Buttons) > maxCardButtons {
					return apperr.Validation(mf+".buttons", fmt.Sprintf("A card has 1 to %d buttons (or none).", maxCardButtons))
				}
				var types []string
				for j, b := range cc.Buttons {
					bt := strings.ToUpper(b.Type)
					bf := fmt.Sprintf("%s.buttons.%d", mf, j)
					if bt != "QUICK_REPLY" && bt != "URL" && bt != "PHONE_NUMBER" {
						return apperr.Validation(bf+".type", "Card buttons are quick reply, URL or phone number buttons.")
					}
					if err := ck.button(bf, bt, b, false); err != nil {
						return err
					}
					types = append(types, bt)
				}
				sh.buttons = strings.Join(types, ",")
			default:
				return apperr.Validation(mf+".type", "A card has a HEADER, an optional BODY and optional BUTTONS.")
			}
		}
		if sh.header == "" {
			return apperr.Validation(cf, "Every card needs an image or video header.")
		}
		if k == 0 {
			first = sh
			continue
		}
		switch {
		case sh.header != first.header:
			return apperr.Validation(cf, "All cards use the same header type (all images or all videos).")
		case sh.body != first.body:
			return apperr.Validation(cf, "Card body text goes on every card or on none.")
		case sh.buttons != first.buttons:
			return apperr.Validation(cf, "All cards have the same buttons, in the same order.")
		}
	}
	return nil
}

// authentication checks a one-time password template: Meta writes the body
// ("<code> is your verification code.") and the footer itself; the template
// only chooses the security line, the expiry and the OTP button.
func (ck templateCheck) authentication(comps []tplComponent, seen map[string]int) error {
	for _, t := range []string{"HEADER", "CAROUSEL", "LIMITED_TIME_OFFER"} {
		if i, ok := seen[t]; ok {
			return apperr.Validation(fmt.Sprintf("components.%d", i), fmt.Sprintf("Authentication templates have no %s.", strings.ToLower(t)))
		}
	}
	bi, ok := seen["BUTTONS"]
	if !ok {
		return apperr.Validation("components", "An authentication template needs its one-time password button.")
	}
	b := comps[seen["BODY"]]
	if b.Text != nil {
		return apperr.Validation(fmt.Sprintf("components.%d.text", seen["BODY"]), "Meta writes the body of authentication templates; remove the text.")
	}
	if i, ok := seen["FOOTER"]; ok {
		ft := comps[i]
		if ft.Text != nil {
			return apperr.Validation(fmt.Sprintf("components.%d.text", i), "Meta writes the footer of authentication templates; set code_expiration_minutes instead.")
		}
		if m := ft.CodeExpirationMinutes; m != nil && (*m < 1 || *m > 90) {
			return apperr.Validation(fmt.Sprintf("components.%d.code_expiration_minutes", i), "The code expires after 1 to 90 minutes.")
		}
	}
	bs := comps[bi].Buttons
	f := fmt.Sprintf("components.%d.buttons", bi)
	if len(bs) != 1 || !strings.EqualFold(bs[0].Type, "OTP") {
		return apperr.Validation(f, "An authentication template has exactly one button, of type OTP.")
	}
	o := bs[0]
	bf := f + ".0"
	if err := label(bf, o, maxButtonText, false); err != nil {
		return err
	}
	switch strings.ToUpper(o.OTPType) {
	case "COPY_CODE":
		if o.AutofillText != nil || len(o.SupportedApps) > 0 {
			return apperr.Validation(bf+".otp_type", "Autofill text and apps belong to one-tap and zero-tap buttons.")
		}
		return nil
	case "ONE_TAP", "ZERO_TAP":
	default:
		return apperr.Validation(bf+".otp_type", "otp_type is COPY_CODE, ONE_TAP or ZERO_TAP.")
	}
	if o.AutofillText != nil && chars(*o.AutofillText) > maxButtonText {
		return apperr.Validation(bf+".autofill_text", fmt.Sprintf("The autofill label is at most %d characters.", maxButtonText))
	}
	if len(o.SupportedApps) == 0 || len(o.SupportedApps) > maxSupportedApps {
		return apperr.Validation(bf+".supported_apps", fmt.Sprintf("Name 1 to %d Android apps (package name and signature hash).", maxSupportedApps))
	}
	for k, a := range o.SupportedApps {
		af := fmt.Sprintf("%s.supported_apps.%d", bf, k)
		if chars(a.PackageName) > maxPackageName || !packageName.MatchString(a.PackageName) {
			return apperr.Validation(af+".package_name", "Use the Android package name, e.g. com.example.app.")
		}
		if !signatureHash.MatchString(a.SignatureHash) {
			return apperr.Validation(af+".signature_hash", "The app signature hash is 11 characters (letters, digits, + / =).")
		}
	}
	if strings.EqualFold(o.OTPType, "ZERO_TAP") && (o.ZeroTapTermsAccepted == nil || !*o.ZeroTapTermsAccepted) {
		return apperr.Validation(bf+".zero_tap_terms_accepted", "Zero-tap needs Meta's zero-tap terms to be accepted.")
	}
	return nil
}
