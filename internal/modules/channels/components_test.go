package channels

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The creation examples of Meta's documentation (templates/components,
// limited-time-offer and media-card-carousel pages) must pass as they are.
const (
	docSeasonalPromotion = `[
	  {"type":"HEADER","format":"TEXT","text":"Our {{1}} is on!","example":{"header_text":["Summer Sale"]}},
	  {"type":"BODY","text":"Shop now through {{1}} and use code {{2}} to get {{3}} off of all merchandise.",
	   "example":{"body_text":[["the end of August","25OFF","25%"]]}},
	  {"type":"FOOTER","text":"Use the buttons below to manage your marketing subscriptions"},
	  {"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Unsubscribe from Promos"},{"type":"QUICK_REPLY","text":"Unsubscribe from All"}]}]`
	docOrderConfirmation = `[
	  {"type":"HEADER","format":"DOCUMENT","example":{"header_handle":["4::YX..."]}},
	  {"type":"BODY","text":"Thank you for your order, {{1}}! Your order number is {{2}}.","example":{"body_text":[["Pablo","860198-230332"]]}},
	  {"type":"BUTTONS","buttons":[{"type":"PHONE_NUMBER","text":"Call","phone_number":"15550051310"},
	   {"type":"URL","text":"Contact Support","url":"https://www.luckyshrub.com/support"}]}]`
	docDeliveryUpdate = `[
	  {"type":"HEADER","format":"LOCATION"},
	  {"type":"BODY","text":"Good news {{1}}! Your order #{{2}} is on its way to the location above.","example":{"body_text":[["Mark","566701"]]}},
	  {"type":"FOOTER","text":"To stop receiving delivery updates, tap the button below."},
	  {"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Stop Delivery Updates"}]}]`
	docNamedBody = `[{"type":"body","text":"Thank you, {{first_name}}! Your order number is {{order_number}}.",
	  "example":{"body_text_named_params":[{"param_name":"first_name","example":"Pablo"},{"param_name":"order_number","example":"860198-230332"}]}}]`
	docLTO = `[
	  {"type":"header","format":"image","example":{"header_handle":["4::aW..."]}},
	  {"type":"limited_time_offer","limited_time_offer":{"text":"Expiring offer!","has_expiration":true}},
	  {"type":"body","text":"Good news, {{1}}! Use code {{2}} to get 25% off all Caribbean Destination packages!","example":{"body_text":[["Pablo","CARIBE25"]]}},
	  {"type":"buttons","buttons":[{"type":"copy_code","example":"CARIBE25"},
	   {"type":"url","text":"Book now!","url":"https://awesomedestinations.com/offers?code={{1}}","example":["https://awesomedestinations.com/offers?ref=n3mtql"]}]}]`
	docCarouselCard = `{"components":[{"type":"header","format":"image","example":{"header_handle":["4::an..."]}},
	  {"type":"buttons","buttons":[{"type":"quick_reply","text":"Send me more like this!"},
	   {"type":"url","text":"Shop","url":"https://www.luckyshrub.com/rare-succulents/{{1}}","example":["BLUE_ELF"]}]}]}`
	docCarouselBody = `{"type":"body","text":"Rare succulents for sale! {{1}}, add these unique plants to your collection. Each of these rare succulents are {{2}} if you checkout using code {{3}}.",
	  "example":{"body_text":[["Pablo","30%","30OFF"]]}}`
)

func carousel(cards ...string) string {
	return `[` + docCarouselBody + `,{"type":"carousel","cards":[` + strings.Join(cards, ",") + `]}]`
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func otp(button string, extra ...string) string {
	parts := append([]string{`{"type":"body","add_security_recommendation":true}`}, extra...)
	parts = append(parts, `{"type":"buttons","buttons":[`+button+`]}`)
	return "[" + strings.Join(parts, ",") + "]"
}

func body(text string) string {
	b, _ := json.Marshal(text)
	return `{"type":"BODY","text":` + string(b) + `}`
}

func buttons(bs ...string) string {
	return `{"type":"BUTTONS","buttons":[` + strings.Join(bs, ",") + `]}`
}

const qr = `{"type":"QUICK_REPLY","text":"Yes"}`
const urlBtn = `{"type":"URL","text":"Shop","url":"https://shop.test/"}`
const phoneBtn = `{"type":"PHONE_NUMBER","text":"Call","phone_number":"15550051310"}`

func TestValidateComponents(t *testing.T) {
	long := func(n int) string { return strings.Repeat("ç", n) } // characters, not bytes
	app := `{"package_name":"com.example.app","signature_hash":"K8a/AINcGX7"}`
	cases := []struct {
		name, category, format, comps string
		field                         string // "" = valid
	}{
		{"doc seasonal promotion", "MARKETING", "", docSeasonalPromotion, ""},
		{"doc order confirmation", "UTILITY", "", docOrderConfirmation, ""},
		{"doc delivery update", "UTILITY", "", docDeliveryUpdate, ""},
		{"doc named body", "UTILITY", "NAMED", docNamedBody, ""},
		{"doc limited-time offer", "MARKETING", "", docLTO, ""},
		{"doc carousel", "MARKETING", "", carousel(docCarouselCard, docCarouselCard, docCarouselCard), ""},

		{"not an array", "UTILITY", "", `{"type":"BODY"}`, "components"},
		{"empty", "UTILITY", "", `[]`, "components"},
		{"no body", "UTILITY", "", `[{"type":"FOOTER","text":"x"}]`, "components"},
		{"unknown type", "UTILITY", "", `[` + body("x") + `,{"type":"STICKER"}]`, "components.1.type"},
		{"two bodies", "UTILITY", "", `[` + body("x") + `,` + body("y") + `]`, "components.1.type"},
		{"body 1024 ok", "UTILITY", "", `[` + body(long(1024)) + `]`, ""},
		{"body 1025", "UTILITY", "", `[` + body(long(1025)) + `]`, "components.0.text"},
		{"body var without example", "UTILITY", "", `[` + body("Hi {{1}}") + `]`, "components.0.example"},
		{"body example count", "UTILITY", "", `[{"type":"BODY","text":"{{1}} {{2}}","example":{"body_text":[["a"]]}}]`, "components.0.example"},
		{"positional gap", "UTILITY", "", `[{"type":"BODY","text":"{{1}} {{3}}","example":{"body_text":[["a","b"]]}}]`, "components.0.text"},
		{"named in positional", "UTILITY", "", `[{"type":"BODY","text":"{{name}}","example":{"body_text":[["a"]]}}]`, "components.0.text"},
		{"positional in named", "UTILITY", "NAMED", `[{"type":"BODY","text":"{{1}}","example":{"body_text_named_params":[{"param_name":"1","example":"a"}]}}]`, "components.0.text"},
		{"named example missing", "UTILITY", "NAMED", `[{"type":"BODY","text":"{{a}} {{b}}","example":{"body_text_named_params":[{"param_name":"a","example":"x"}]}}]`, "components.0.example"},

		{"text header 60", "UTILITY", "", `[{"type":"HEADER","format":"TEXT","text":"` + long(60) + `"},` + body("x") + `]`, ""},
		{"text header 61", "UTILITY", "", `[{"type":"HEADER","format":"TEXT","text":"` + long(61) + `"},` + body("x") + `]`, "components.0.text"},
		{"text header two vars", "UTILITY", "", `[{"type":"HEADER","format":"TEXT","text":"{{1}} {{2}}","example":{"header_text":["a","b"]}},` + body("x") + `]`, "components.0.text"},
		{"text header example", "UTILITY", "", `[{"type":"HEADER","format":"TEXT","text":"Hi {{1}}"},` + body("x") + `]`, "components.0.example"},
		{"media header without sample", "MARKETING", "", `[{"type":"HEADER","format":"IMAGE"},` + body("x") + `]`, "components.0.example"},
		{"unknown header", "MARKETING", "", `[{"type":"HEADER","format":"AUDIO"},` + body("x") + `]`, "components.0.format"},
		{"gif header", "MARKETING", "", `[{"type":"HEADER","format":"GIF","example":{"header_handle":["h"]}},` + body("x") + `]`, "components.0.format"},
		{"location header marketing", "MARKETING", "", `[{"type":"HEADER","format":"LOCATION"},` + body("x") + `]`, ""},

		{"footer 60", "UTILITY", "", `[` + body("x") + `,{"type":"FOOTER","text":"` + long(60) + `"}]`, ""},
		{"footer 61", "UTILITY", "", `[` + body("x") + `,{"type":"FOOTER","text":"` + long(61) + `"}]`, "components.1.text"},
		{"footer variable", "UTILITY", "", `[` + body("x") + `,{"type":"FOOTER","text":"Hi {{1}}"}]`, "components.1.text"},

		{"10 quick replies", "MARKETING", "", `[` + body("x") + `,` + buttons(repeat(qr, 10)...) + `]`, ""},
		{"11 buttons", "MARKETING", "", `[` + body("x") + `,` + buttons(repeat(qr, 11)...) + `]`, "components.1.buttons"},
		{"no buttons", "MARKETING", "", `[` + body("x") + `,` + buttons() + `]`, "components.1.buttons"},
		{"quick replies grouped first", "MARKETING", "", `[` + body("x") + `,` + buttons(qr, qr, urlBtn, phoneBtn) + `]`, ""},
		{"quick replies grouped last", "MARKETING", "", `[` + body("x") + `,` + buttons(urlBtn, phoneBtn, qr, qr) + `]`, ""},
		{"quick replies split", "MARKETING", "", `[` + body("x") + `,` + buttons(qr, urlBtn, qr) + `]`, "components.1.buttons"},
		{"url qr url", "MARKETING", "", `[` + body("x") + `,` + buttons(urlBtn, qr, urlBtn) + `]`, "components.1.buttons"},
		{"three urls", "MARKETING", "", `[` + body("x") + `,` + buttons(urlBtn, urlBtn, urlBtn) + `]`, "components.1.buttons.2.type"},
		{"two phones", "MARKETING", "", `[` + body("x") + `,` + buttons(phoneBtn, phoneBtn) + `]`, "components.1.buttons.1.type"},
		{"button label 26", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"QUICK_REPLY","text":"`+long(26)+`"}`) + `]`, "components.1.buttons.0.text"},
		{"url variable not at end", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"URL","text":"a","url":"https://a.test/{{1}}/x","example":["v"]}`) + `]`, "components.1.buttons.0.url"},
		{"url two variables", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"URL","text":"a","url":"https://a.test/{{1}}{{2}}","example":["v"]}`) + `]`, "components.1.buttons.0.url"},
		{"url variable without example", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"URL","text":"a","url":"https://a.test/{{1}}"}`) + `]`, "components.1.buttons.0.example"},
		{"url not web", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"URL","text":"a","url":"ftp://a.test/"}`) + `]`, "components.1.buttons.0.url"},
		{"phone too long", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"PHONE_NUMBER","text":"a","phone_number":"123456789012345678901"}`) + `]`, "components.1.buttons.0.phone_number"},
		{"copy code", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"COPY_CODE","example":"250FF"}`) + `]`, ""},
		{"copy code 21", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"COPY_CODE","example":"`+long(21)+`"}`) + `]`, "components.1.buttons.0.example"},
		{"two copy codes", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"COPY_CODE","example":"A"}`, `{"type":"COPY_CODE","example":"B"}`) + `]`, "components.1.buttons.1.type"},
		{"voice call", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"VOICE_CALL","text":"Call","ttl_minutes":2880}`) + `]`, ""},
		{"voice call ttl", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"VOICE_CALL","ttl_minutes":60}`) + `]`, "components.1.buttons.0.ttl_minutes"},
		{"voice call label 21", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"VOICE_CALL","text":"`+long(21)+`"}`) + `]`, "components.1.buttons.0.text"},
		{"flow by id", "UTILITY", "", `[` + body("x") + `,` + buttons(`{"type":"FLOW","text":"Book","flow_id":"123","flow_action":"navigate"}`) + `]`, ""},
		{"flow by name", "UTILITY", "", `[` + body("x") + `,` + buttons(`{"type":"FLOW","text":"Book","flow_name":"booking","flow_action":"data_exchange"}`) + `]`, ""},
		{"flow two refs", "UTILITY", "", `[` + body("x") + `,` + buttons(`{"type":"FLOW","text":"Book","flow_id":"1","flow_name":"b"}`) + `]`, "components.1.buttons.0.flow_id"},
		{"flow no ref", "UTILITY", "", `[` + body("x") + `,` + buttons(`{"type":"FLOW","text":"Book"}`) + `]`, "components.1.buttons.0.flow_id"},
		{"flow bad action", "UTILITY", "", `[` + body("x") + `,` + buttons(`{"type":"FLOW","text":"Book","flow_id":"1","flow_action":"open"}`) + `]`, "components.1.buttons.0.flow_action"},
		{"mpm", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"MPM","text":"View"}`) + `]`, "components.1.buttons.0.type"},
		{"catalog", "MARKETING", "", `[` + body("x") + `,` + buttons(`{"type":"CATALOG","text":"View"}`) + `]`, "components.1.buttons.0.type"},
		{"otp outside authentication", "UTILITY", "", `[` + body("x") + `,` + buttons(`{"type":"OTP","otp_type":"COPY_CODE"}`) + `]`, "components.1.buttons.0.type"},

		{"location header in authentication", "AUTHENTICATION", "", `[{"type":"HEADER","format":"LOCATION"},{"type":"BODY"},` + buttons(`{"type":"OTP","otp_type":"COPY_CODE"}`) + `]`, "components.0"},
		{"otp copy code", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"copy_code","text":"Copy code"}`, `{"type":"footer","code_expiration_minutes":5}`), ""},
		{"otp one tap", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"one_tap","text":"Copy","autofill_text":"Autofill","supported_apps":[` + app + `]}`), ""},
		{"otp zero tap", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"zero_tap","zero_tap_terms_accepted":true,"supported_apps":[` + app + `]}`), ""},
		{"otp zero tap terms", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"zero_tap","supported_apps":[` + app + `]}`), "components.1.buttons.0.zero_tap_terms_accepted"},
		{"otp one tap no apps", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"one_tap"}`), "components.1.buttons.0.supported_apps"},
		{"otp six apps", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"one_tap","supported_apps":[` + strings.Join(repeat(app, 6), ",") + `]}`), "components.1.buttons.0.supported_apps"},
		{"otp bad package", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"one_tap","supported_apps":[{"package_name":"app","signature_hash":"K8a/AINcGX7"}]}`), "components.1.buttons.0.supported_apps.0.package_name"},
		{"otp bad hash", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"one_tap","supported_apps":[{"package_name":"com.a.b","signature_hash":"short"}]}`), "components.1.buttons.0.supported_apps.0.signature_hash"},
		{"otp bad type", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"magic"}`), "components.1.buttons.0.otp_type"},
		{"otp expiry 91", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"copy_code"}`, `{"type":"footer","code_expiration_minutes":91}`), "components.1.code_expiration_minutes"},
		{"otp body text", "AUTHENTICATION", "", `[{"type":"body","text":"Your code"},` + buttons(`{"type":"otp","otp_type":"copy_code"}`) + `]`, "components.0.text"},
		{"otp footer text", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"copy_code"}`, `{"type":"footer","text":"bye"}`), "components.1.text"},
		{"otp two buttons", "AUTHENTICATION", "", otp(`{"type":"otp","otp_type":"copy_code"},{"type":"QUICK_REPLY","text":"x"}`), "components.1.buttons"},
		{"otp no button", "AUTHENTICATION", "", `[{"type":"body"}]`, "components"},

		{"lto utility", "UTILITY", "", docLTO, "category"},
		{"lto footer", "MARKETING", "", strings.Replace(docLTO, `{"type":"buttons"`, `{"type":"footer","text":"x"},{"type":"buttons"`, 1), "components.3"},
		{"lto text header", "MARKETING", "", strings.Replace(docLTO, `{"type":"header","format":"image","example":{"header_handle":["4::aW..."]}}`, `{"type":"header","format":"text","text":"Hi"}`, 1), "components.0.format"},
		{"lto no header", "MARKETING", "", strings.Replace(docLTO, `{"type":"header","format":"image","example":{"header_handle":["4::aW..."]}},`, ``, 1), ""},
		{"lto body 601", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"Hurry"}},` + body(long(601)) + `,` + buttons(urlBtn) + `]`, "components.1.text"},
		{"lto text 17", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"` + long(17) + `"}},` + body("x") + `,` + buttons(urlBtn) + `]`, "components.0.limited_time_offer.text"},
		{"lto coupon 16", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"Hurry"}},` + body("x") + `,` + buttons(`{"type":"COPY_CODE","example":"`+long(16)+`"}`, urlBtn) + `]`, "components.2.buttons.0.example"},
		{"lto quick reply", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"Hurry"}},` + body("x") + `,` + buttons(qr, urlBtn) + `]`, "components.2.buttons.0.type"},
		{"lto no url", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"Hurry"}},` + body("x") + `,` + buttons(`{"type":"COPY_CODE","example":"A"}`) + `]`, "components.2.buttons"},
		{"lto no buttons", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"Hurry"}},` + body("x") + `]`, "components"},

		{"carousel utility", "UTILITY", "", carousel(docCarouselCard, docCarouselCard), "category"},
		{"carousel one card", "MARKETING", "", carousel(docCarouselCard), "components.1.cards"},
		{"carousel eleven cards", "MARKETING", "", carousel(repeat(docCarouselCard, 11)...), "components.1.cards"},
		{"carousel ten cards", "MARKETING", "", carousel(repeat(docCarouselCard, 10)...), ""},
		{"carousel mixed headers", "MARKETING", "", carousel(docCarouselCard, strings.Replace(docCarouselCard, `"format":"image"`, `"format":"video"`, 1)), "components.1.cards.1"},
		{"carousel body on one card", "MARKETING", "", carousel(docCarouselCard, strings.Replace(docCarouselCard, `{"type":"buttons"`, `{"type":"body","text":"Nice"},{"type":"buttons"`, 1)), "components.1.cards.1"},
		{"carousel card body 161", "MARKETING", "", carousel(repeat(strings.Replace(docCarouselCard, `{"type":"buttons"`, `{"type":"body","text":"`+long(161)+`"},{"type":"buttons"`, 1), 2)...), "components.1.cards.0.components.1.text"},
		{"carousel different buttons", "MARKETING", "", carousel(docCarouselCard, strings.Replace(docCarouselCard, `{"type":"quick_reply","text":"Send me more like this!"},`, ``, 1)), "components.1.cards.1"},
		{"carousel three buttons", "MARKETING", "", carousel(repeat(strings.Replace(docCarouselCard, `{"type":"quick_reply"`, `{"type":"quick_reply","text":"a"},{"type":"quick_reply"`, 1), 2)...), "components.1.cards.0.components.1.buttons"},
		{"carousel copy code card button", "MARKETING", "", carousel(repeat(strings.Replace(docCarouselCard, `{"type":"quick_reply","text":"Send me more like this!"}`, `{"type":"copy_code","example":"A"}`, 1), 2)...), "components.1.cards.0.components.1.buttons.0.type"},
		{"carousel text header card", "MARKETING", "", carousel(repeat(`{"components":[{"type":"header","format":"text","text":"x"}]}`, 2)...), "components.1.cards.0.components.0.format"},
		{"carousel card without header", "MARKETING", "", carousel(repeat(`{"components":[{"type":"body","text":"x"}]}`, 2)...), "components.1.cards.0"},
		{"carousel top-level buttons", "MARKETING", "", strings.TrimSuffix(carousel(docCarouselCard, docCarouselCard), "]") + `,` + buttons(qr) + `]`, "components.2"},
		{"carousel and lto", "MARKETING", "", `[{"type":"limited_time_offer","limited_time_offer":{"text":"x"}},` + strings.TrimPrefix(carousel(docCarouselCard, docCarouselCard), "["), "components"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateComponents(c.category, c.format, json.RawMessage(c.comps))
			if c.field == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			e, ok := apperr.As(err)
			if !ok || e.Code != "VALIDATION_ERROR" {
				t.Fatalf("want a validation error on %s, got %v", c.field, err)
			}
			if e.Field != c.field {
				t.Fatalf("field = %q (%s), want %q", e.Field, e.Message, c.field)
			}
		})
	}
}
