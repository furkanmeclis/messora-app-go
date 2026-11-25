package campaigns

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// templateVar matches Meta's template variables: {{1}} or {{first_name}}.
var templateVar = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// Shape is what a template needs filled in at send time. Button fields hold
// the template's button positions (Meta's "index").
type Shape struct {
	Named       bool
	HeaderMedia string   // image | video | document | location, "" for text or none
	Header      []string // variables of a text header
	Body        []string
	URLButtons  []int // URL buttons with a variable
	CopyCode    []int // copy code button (at most one)
	Flow        []int // Flow button (at most one)
	OTP         []int // one-time password button of an authentication template
	// LTO marks a limited-time offer template: sending needs the offer's
	// expiration_time_ms.
	LTO bool
	// Cards of a media card carousel, in order.
	Cards []CardShape
}

// CardShape is what one carousel card needs.
type CardShape struct {
	HeaderMedia string // image | video
	Body        []string
	URLButtons  []int
	// QuickReplies are positions of quick reply buttons; a value given for
	// one is sent as its payload.
	QuickReplies []int
}

// Copy code limits: 15 characters in limited-time offers (Meta's send
// reference), 20 elsewhere (the creation reference's maximum).
const (
	maxOfferCode = 15
	maxCopyCode  = 20
	maxOTPCode   = 15
)

func vars(s string) []string {
	var out []string
	for _, m := range templateVar.FindAllStringSubmatch(s, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

type shapeButton struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type shapeComponent struct {
	Type    string        `json:"type"`
	Format  string        `json:"format"`
	Text    string        `json:"text"`
	Buttons []shapeButton `json:"buttons"`
	Cards   []struct {
		Components []shapeComponent `json:"components"`
	} `json:"cards"`
}

// ShapeOf reads a template's components (Meta's JSON, untrusted). Meta
// stores an authentication template's OTP button as a URL button, so the
// category decides what a URL button of such a template is.
func ShapeOf(components json.RawMessage, parameterFormat, category string) Shape {
	sh := Shape{Named: strings.EqualFold(parameterFormat, "NAMED")}
	auth := strings.EqualFold(category, "AUTHENTICATION")
	var comps []shapeComponent
	_ = json.Unmarshal(components, &comps)
	for _, c := range comps {
		switch strings.ToUpper(c.Type) {
		case "HEADER":
			switch f := strings.ToUpper(c.Format); f {
			case "", "TEXT":
				sh.Header = vars(c.Text)
			default:
				sh.HeaderMedia = strings.ToLower(f)
			}
		case "BODY":
			sh.Body = vars(c.Text)
		case "LIMITED_TIME_OFFER":
			sh.LTO = true
		case "BUTTONS":
			for i, b := range c.Buttons {
				switch t := strings.ToUpper(b.Type); {
				case t == "OTP", auth && t == "URL":
					sh.OTP = append(sh.OTP, i)
				case t == "URL" && templateVar.MatchString(b.URL):
					sh.URLButtons = append(sh.URLButtons, i)
				case t == "COPY_CODE":
					sh.CopyCode = append(sh.CopyCode, i)
				case t == "FLOW":
					sh.Flow = append(sh.Flow, i)
				}
			}
		case "CAROUSEL":
			for _, card := range c.Cards {
				var cs CardShape
				for _, cc := range card.Components {
					switch strings.ToUpper(cc.Type) {
					case "HEADER":
						cs.HeaderMedia = strings.ToLower(cc.Format)
					case "BODY":
						cs.Body = vars(cc.Text)
					case "BUTTONS":
						for i, b := range cc.Buttons {
							if strings.EqualFold(b.Type, "URL") && templateVar.MatchString(b.URL) {
								cs.URLButtons = append(cs.URLButtons, i)
							} else if strings.EqualFold(b.Type, "QUICK_REPLY") {
								cs.QuickReplies = append(cs.QuickReplies, i)
							}
						}
					}
				}
				sh.Cards = append(sh.Cards, cs)
			}
		}
	}
	return sh
}

// checkValues reports the first variable of want with no value in raw.
func checkValues(field string, want []string, named bool, raw json.RawMessage) error {
	if len(want) == 0 {
		return nil
	}
	if named {
		var m map[string]string
		_ = json.Unmarshal(raw, &m)
		for _, v := range want {
			if strings.TrimSpace(m[v]) == "" {
				return apperr.Validation(field+"."+v, fmt.Sprintf("The template needs a value for {{%s}}.", v))
			}
		}
		return nil
	}
	var list []string
	_ = json.Unmarshal(raw, &list)
	for _, v := range want {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > len(list) || strings.TrimSpace(list[n-1]) == "" {
			return apperr.Validation(field+"."+v, fmt.Sprintf("The template needs a value for {{%s}}.", v))
		}
	}
	return nil
}

func checkMedia(field, want string, hm *HeaderMedia) error {
	switch want {
	case "":
		if hm != nil {
			return apperr.Validation(field, "This template has no media header; remove the header media.")
		}
	case "image", "video", "document":
		if hm == nil {
			return apperr.Validation(field, fmt.Sprintf("This template has a %s header: add the %s (upload or https link).", want, want))
		}
		if hm.Type != want {
			return apperr.Validation(field+".type", fmt.Sprintf("This template's header is a %s, not a %s.", want, hm.Type))
		}
	default:
		return apperr.Validation(field, fmt.Sprintf("Templates with a %s header are not supported.", want))
	}
	return nil
}

func checkButtons(field string, want []int, have []string) error {
	for _, i := range want {
		if i >= len(have) || strings.TrimSpace(have[i]) == "" {
			return apperr.Validation(fmt.Sprintf("%s.%d", field, i), fmt.Sprintf("Button %d needs the end of its web address.", i+1))
		}
	}
	return nil
}

// checkLocation accepts decimal degrees within range.
func checkLocation(l *Location) error {
	if l == nil {
		return apperr.Validation("template_params.location", "This template has a location header: give the latitude and longitude.")
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(l.Latitude), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(l.Longitude), 64)
	if err1 != nil || lat < -90 || lat > 90 {
		return apperr.Validation("template_params.location.latitude", "Latitude is a number between -90 and 90.")
	}
	if err2 != nil || lng < -180 || lng > 180 {
		return apperr.Validation("template_params.location.longitude", "Longitude is a number between -180 and 180.")
	}
	return nil
}

// CheckParams verifies that p fills everything the template needs, so a
// mismatch is reported before sending instead of failing at Meta for every
// recipient.
func CheckParams(sh Shape, p Params) error {
	if sh.HeaderMedia == "location" {
		if p.HeaderMedia != nil {
			return apperr.Validation("template_params.header_media", "This template's header is a location, not media.")
		}
		if err := checkLocation(p.Location); err != nil {
			return err
		}
	} else {
		if p.Location != nil {
			return apperr.Validation("template_params.location", "This template has no location header; remove the location.")
		}
		if err := checkMedia("template_params.header_media", sh.HeaderMedia, p.HeaderMedia); err != nil {
			return err
		}
	}
	if len(sh.OTP) > 0 {
		// Meta writes the body ("{{1}} is your verification code") and the
		// button: both take the one code.
		code := strings.TrimSpace(p.OTPCode)
		if code == "" {
			return apperr.Validation("template_params.otp_code", "This authentication template needs the one-time code.")
		}
		if utf8.RuneCountInString(code) > maxOTPCode {
			return apperr.Validation("template_params.otp_code", fmt.Sprintf("The code is at most %d characters.", maxOTPCode))
		}
		return nil
	}
	if p.OTPCode != "" {
		return apperr.Validation("template_params.otp_code", "Only authentication templates take a one-time code.")
	}
	if err := checkValues("template_params.header", sh.Header, sh.Named, p.Header); err != nil {
		return err
	}
	if err := checkValues("template_params.body", sh.Body, sh.Named, p.Body); err != nil {
		return err
	}
	if err := checkButtons("template_params.buttons", sh.URLButtons, p.Buttons); err != nil {
		return err
	}
	if len(sh.CopyCode) > 0 {
		limit := maxCopyCode
		if sh.LTO {
			limit = maxOfferCode
		}
		code := strings.TrimSpace(p.CouponCode)
		if code == "" {
			return apperr.Validation("template_params.coupon_code", "This template has a copy code button: give the code.")
		}
		// Placeholders are resolved per contact; only literal codes are
		// measured here.
		if !placeholder.MatchString(code) && utf8.RuneCountInString(code) > limit {
			return apperr.Validation("template_params.coupon_code", fmt.Sprintf("The code is at most %d characters.", limit))
		}
	} else if p.CouponCode != "" {
		return apperr.Validation("template_params.coupon_code", "This template has no copy code button; remove the code.")
	}
	if sh.LTO {
		if p.ExpirationTimeMS == nil || *p.ExpirationTimeMS <= 0 {
			return apperr.Validation("template_params.expiration_time_ms", "A limited-time offer needs the time the offer ends.")
		}
	} else if p.ExpirationTimeMS != nil {
		return apperr.Validation("template_params.expiration_time_ms", "Only limited-time offer templates take an end time.")
	}
	if p.Flow != nil && len(sh.Flow) == 0 {
		return apperr.Validation("template_params.flow", "This template has no Flow button.")
	}
	if len(sh.Cards) != len(p.Cards) {
		if len(sh.Cards) == 0 {
			return apperr.Validation("template_params.cards", "This template is not a carousel; remove the cards.")
		}
		return apperr.Validation("template_params.cards", fmt.Sprintf("This carousel has %d cards: fill each of them.", len(sh.Cards)))
	}
	for k, cs := range sh.Cards {
		f := fmt.Sprintf("template_params.cards.%d", k)
		card := p.Cards[k]
		if err := checkMedia(f+".header_media", cs.HeaderMedia, card.HeaderMedia); err != nil {
			return err
		}
		if err := checkValues(f+".body", cs.Body, sh.Named, card.Body); err != nil {
			return err
		}
		if err := checkButtons(f+".buttons", cs.URLButtons, card.Buttons); err != nil {
			return err
		}
	}
	return nil
}
