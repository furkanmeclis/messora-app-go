package campaigns

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The "template" objects of Meta's send examples (limited-time offer,
// media card carousel and location header pages, the authentication and
// Flow send references). Meta's examples write button indexes both as
// numbers (LTO) and as strings (carousel); Messora always sends strings,
// so the comparison normalises them.
const (
	goldenLTOComponents = `[
	  {"type":"header","format":"image","example":{"header_handle":["4::aW..."]}},
	  {"type":"limited_time_offer","limited_time_offer":{"text":"Expiring offer!","has_expiration":true}},
	  {"type":"body","text":"Good news, {{1}}! Use code {{2}} to get 25% off all Caribbean Destination packages!","example":{"body_text":[["Pablo","CARIBE25"]]}},
	  {"type":"buttons","buttons":[{"type":"copy_code","example":"CARIBE25"},
	   {"type":"url","text":"Book now!","url":"https://awesomedestinations.com/offers?code={{1}}","example":["https://awesomedestinations.com/offers?ref=n3mtql"]}]}]`
	goldenLTOSend = `{"name":"limited_time_offer_caribbean_pkg_2023","language":{"code":"en_US"},"components":[
	  {"type":"header","parameters":[{"type":"image","image":{"id":"1602186516975000"}}]},
	  {"type":"body","parameters":[{"type":"text","text":"Pablo"},{"type":"text","text":"CARIBE25"}]},
	  {"type":"limited_time_offer","parameters":[{"type":"limited_time_offer","limited_time_offer":{"expiration_time_ms":1209600000}}]},
	  {"type":"button","sub_type":"copy_code","index":0,"parameters":[{"type":"coupon_code","coupon_code":"CARIBE25"}]},
	  {"type":"button","sub_type":"url","index":1,"parameters":[{"type":"text","text":"n3mtql"}]}]}`

	goldenCarouselCard = `{"components":[{"type":"header","format":"image","example":{"header_handle":["4::an..."]}},
	  {"type":"buttons","buttons":[{"type":"quick_reply","text":"Send me more like this!"},
	   {"type":"url","text":"Shop","url":"https://www.luckyshrub.com/rare-succulents/{{1}}","example":["BLUE_ELF"]}]}]}`
	goldenCarouselComponents = `[{"type":"body","text":"Rare succulents for sale! {{1}}, add these unique plants to your collection. Each of these rare succulents are {{2}} if you checkout using code {{3}}.","example":{"body_text":[["Pablo","30%","30OFF"]]}},
	  {"type":"carousel","cards":[` + goldenCarouselCard + `,` + goldenCarouselCard + `,` + goldenCarouselCard + `]}]`
	goldenCarouselSend = `{"name":"carousel_template_media_cards_v1","language":{"code":"en_US"},"components":[
	  {"type":"body","parameters":[{"type":"text","text":"Pablo"},{"type":"text","text":"20%"},{"type":"text","text":"20OFF"}]},
	  {"type":"carousel","cards":[
	    {"card_index":0,"components":[{"type":"header","parameters":[{"type":"image","image":{"id":"1558081531584829"}}]},
	      {"type":"button","sub_type":"quick_reply","index":"0","parameters":[{"type":"payload","payload":"more-aloes"}]},
	      {"type":"button","sub_type":"url","index":"1","parameters":[{"type":"text","text":"blue-elf"}]}]},
	    {"card_index":1,"components":[{"type":"header","parameters":[{"type":"image","image":{"id":"861236878885705"}}]},
	      {"type":"button","sub_type":"quick_reply","index":"0","parameters":[{"type":"payload","payload":"more-crassulas"}]},
	      {"type":"button","sub_type":"url","index":"1","parameters":[{"type":"text","text":"buddhas-temple"}]}]},
	    {"card_index":2,"components":[{"type":"header","parameters":[{"type":"image","image":{"id":"1587064918516321"}}]},
	      {"type":"button","sub_type":"quick_reply","index":"0","parameters":[{"type":"payload","payload":"more-echeverias"}]},
	      {"type":"button","sub_type":"url","index":"1","parameters":[{"type":"text","text":"black-prince"}]}]}]}]}`

	goldenLocationComponents = `[{"type":"HEADER","format":"LOCATION"},
	  {"type":"BODY","text":"Good news {{1}}! Your order #{{2}} is on its way to the location above.","example":{"body_text":[["Mark","566701"]]}},
	  {"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Stop Delivery Updates"}]}]`
	goldenLocationSend = `{"name":"order_delivery_update","language":{"code":"en_US"},"components":[
	  {"type":"header","parameters":[{"type":"location","location":{"latitude":"37.44211676562361","longitude":"-122.16155960083124","name":"Philz Coffee","address":"101 Forest Ave, Palo Alto, CA 94301"}}]},
	  {"type":"body","parameters":[{"type":"text","text":"Mark"},{"type":"text","text":"566701"}]}]}`

	// As Meta returns an authentication template after creation: the OTP
	// button comes back as a URL button.
	goldenOTPComponents = `[{"type":"BODY","text":"*{{1}}* is your verification code. For your security, do not share this code.","add_security_recommendation":true},
	  {"type":"FOOTER","text":"This code expires in 5 minutes.","code_expiration_minutes":5},
	  {"type":"BUTTONS","buttons":[{"type":"URL","text":"Copy code","url":"https://www.whatsapp.com/otp/code/?otp_type=COPY_CODE&code_expiration_minutes=5&code=otp{{1}}"}]}]`
	goldenOTPSend = `{"name":"verification_code","language":{"code":"en_US"},"components":[
	  {"type":"body","parameters":[{"type":"text","text":"J$FpnYnP"}]},
	  {"type":"button","sub_type":"url","index":"0","parameters":[{"type":"text","text":"J$FpnYnP"}]}]}`

	goldenFlowComponents = `[{"type":"BODY","text":"Book your appointment"},
	  {"type":"BUTTONS","buttons":[{"type":"FLOW","text":"Book now","flow_id":"1234567890","flow_action":"navigate"}]}]`
	goldenFlowSend = `{"name":"book_appointment","language":{"code":"en_US"},"components":[
	  {"type":"button","sub_type":"flow","index":"0","parameters":[{"type":"action","action":{"flow_token":"FLOW_TOKEN","flow_action_data":{"product_name":"name","product_price":5}}}]}]}`
)

// normalise decodes JSON and turns button indexes into strings.
func normalise(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("bad json: %v\n%s", err, raw)
	}
	var walk func(any)
	walk = func(x any) {
		switch x := x.(type) {
		case map[string]any:
			if n, ok := x["index"].(float64); ok {
				x["index"] = strconv.Itoa(int(n))
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(v)
	return v
}

func golden(t *testing.T, sh Shape, p Params, name, want string) {
	t.Helper()
	if err := CheckParams(sh, p); err != nil {
		t.Fatalf("CheckParams: %v", err)
	}
	tpl, miss := Render(sh, p, db.Contact{Name: "Pablo Diaz"}, name, "en_US")
	if miss != "" {
		t.Fatalf("missing %s", miss)
	}
	got, _ := json.Marshal(tpl)
	if g, w := normalise(t, got), normalise(t, []byte(want)); !reflect.DeepEqual(g, w) {
		t.Fatalf("payload differs from Meta's example\n got: %s\nwant: %s", got, want)
	}
}

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestRenderMatchesMetaExamples(t *testing.T) {
	t.Run("limited-time offer", func(t *testing.T) {
		sh := ShapeOf(json.RawMessage(goldenLTOComponents), "POSITIONAL", "MARKETING")
		if !sh.LTO || len(sh.CopyCode) != 1 || sh.CopyCode[0] != 0 || len(sh.URLButtons) != 1 || sh.URLButtons[0] != 1 {
			t.Fatalf("shape %+v", sh)
		}
		ms := int64(1209600000)
		golden(t, sh, Params{HeaderMedia: &HeaderMedia{Type: "image", ID: "1602186516975000"}, Body: raw([]string{"{{first_name}}", "CARIBE25"}),
			Buttons: []string{"", "n3mtql"}, Extras: Extras{CouponCode: "CARIBE25", ExpirationTimeMS: &ms}},
			"limited_time_offer_caribbean_pkg_2023", goldenLTOSend)
	})
	t.Run("carousel", func(t *testing.T) {
		sh := ShapeOf(json.RawMessage(goldenCarouselComponents), "POSITIONAL", "MARKETING")
		if len(sh.Cards) != 3 || sh.Cards[0].HeaderMedia != "image" || len(sh.Cards[0].URLButtons) != 1 || len(sh.Cards[0].QuickReplies) != 1 {
			t.Fatalf("shape %+v", sh)
		}
		card := func(id, payload, slug string) CardParams {
			return CardParams{HeaderMedia: &HeaderMedia{Type: "image", ID: id}, Buttons: []string{payload, slug}}
		}
		golden(t, sh, Params{Body: raw([]string{"Pablo", "20%", "20OFF"}), Extras: Extras{Cards: []CardParams{
			card("1558081531584829", "more-aloes", "blue-elf"), card("861236878885705", "more-crassulas", "buddhas-temple"),
			card("1587064918516321", "more-echeverias", "black-prince")}}},
			"carousel_template_media_cards_v1", goldenCarouselSend)
	})
	t.Run("location header", func(t *testing.T) {
		sh := ShapeOf(json.RawMessage(goldenLocationComponents), "POSITIONAL", "UTILITY")
		golden(t, sh, Params{Body: raw([]string{"Mark", "566701"}), Extras: Extras{Location: &Location{Latitude: "37.44211676562361",
			Longitude: "-122.16155960083124", Name: "Philz Coffee", Address: "101 Forest Ave, Palo Alto, CA 94301"}}},
			"order_delivery_update", goldenLocationSend)
	})
	t.Run("authentication", func(t *testing.T) {
		sh := ShapeOf(json.RawMessage(goldenOTPComponents), "POSITIONAL", "AUTHENTICATION")
		if len(sh.OTP) != 1 || len(sh.URLButtons) != 0 {
			t.Fatalf("shape %+v", sh)
		}
		golden(t, sh, Params{Extras: Extras{OTPCode: "J$FpnYnP"}}, "verification_code", goldenOTPSend)
	})
	t.Run("flow button", func(t *testing.T) {
		sh := ShapeOf(json.RawMessage(goldenFlowComponents), "POSITIONAL", "UTILITY")
		golden(t, sh, Params{Extras: Extras{Flow: &FlowParams{Token: "FLOW_TOKEN",
			ActionData: map[string]any{"product_name": "name", "product_price": 5}}}}, "book_appointment", goldenFlowSend)
	})
	t.Run("flow button default token", func(t *testing.T) {
		sh := ShapeOf(json.RawMessage(goldenFlowComponents), "POSITIONAL", "UTILITY")
		golden(t, sh, Params{}, "book_appointment", `{"name":"book_appointment","language":{"code":"en_US"},"components":[
		  {"type":"button","sub_type":"flow","index":"0","parameters":[{"type":"action","action":{"flow_token":"unused"}}]}]}`)
	})
}

func TestCheckParamsNewComponents(t *testing.T) {
	lto := ShapeOf(json.RawMessage(goldenLTOComponents), "POSITIONAL", "MARKETING")
	carousel := ShapeOf(json.RawMessage(goldenCarouselComponents), "POSITIONAL", "MARKETING")
	loc := ShapeOf(json.RawMessage(goldenLocationComponents), "POSITIONAL", "UTILITY")
	otp := ShapeOf(json.RawMessage(goldenOTPComponents), "POSITIONAL", "AUTHENTICATION")
	plain := ShapeOf(json.RawMessage(`[{"type":"BODY","text":"Hi"}]`), "POSITIONAL", "UTILITY")
	ms := int64(1)
	img := &HeaderMedia{Type: "image", Link: "https://cdn.test/a.jpg"}
	ltoOK := Params{HeaderMedia: img, Body: raw([]string{"a", "b"}), Buttons: []string{"", "x"}, Extras: Extras{CouponCode: "CODE", ExpirationTimeMS: &ms}}
	card := CardParams{HeaderMedia: img, Buttons: []string{"", "slug"}}
	cards := []CardParams{card, card, card}
	cases := []struct {
		name  string
		sh    Shape
		p     Params
		field string
	}{
		{"lto ok", lto, ltoOK, ""},
		{"lto no expiry", lto, Params{HeaderMedia: img, Body: ltoOK.Body, Buttons: ltoOK.Buttons, Extras: Extras{CouponCode: "CODE"}}, "template_params.expiration_time_ms"},
		{"lto no coupon", lto, Params{HeaderMedia: img, Body: ltoOK.Body, Buttons: ltoOK.Buttons, Extras: Extras{ExpirationTimeMS: &ms}}, "template_params.coupon_code"},
		{"lto coupon 16", lto, Params{HeaderMedia: img, Body: ltoOK.Body, Buttons: ltoOK.Buttons, Extras: Extras{CouponCode: "ABCDEFGHIJKLMNOP", ExpirationTimeMS: &ms}}, "template_params.coupon_code"},
		{"lto coupon placeholder", lto, Params{HeaderMedia: img, Body: ltoOK.Body, Buttons: ltoOK.Buttons, Extras: Extras{CouponCode: "{{attr:coupon_code_long}}", ExpirationTimeMS: &ms}}, ""},
		{"expiry on plain", plain, Params{Extras: Extras{ExpirationTimeMS: &ms}}, "template_params.expiration_time_ms"},
		{"coupon on plain", plain, Params{Extras: Extras{CouponCode: "X"}}, "template_params.coupon_code"},
		{"carousel ok", carousel, Params{Body: raw([]string{"a", "b", "c"}), Extras: Extras{Cards: cards}}, ""},
		{"carousel too few cards", carousel, Params{Body: raw([]string{"a", "b", "c"}), Extras: Extras{Cards: cards[:2]}}, "template_params.cards"},
		{"carousel card without media", carousel, Params{Body: raw([]string{"a", "b", "c"}), Extras: Extras{Cards: []CardParams{card, {Buttons: card.Buttons}, card}}}, "template_params.cards.1.header_media"},
		{"carousel card video", carousel, Params{Body: raw([]string{"a", "b", "c"}), Extras: Extras{Cards: []CardParams{card, card, {HeaderMedia: &HeaderMedia{Type: "video", Link: "https://v"}, Buttons: card.Buttons}}}}, "template_params.cards.2.header_media.type"},
		{"carousel card url missing", carousel, Params{Body: raw([]string{"a", "b", "c"}), Extras: Extras{Cards: []CardParams{card, {HeaderMedia: img}, card}}}, "template_params.cards.1.buttons.1"},
		{"cards on plain", plain, Params{Extras: Extras{Cards: cards}}, "template_params.cards"},
		{"location ok", loc, Params{Body: raw([]string{"a", "b"}), Extras: Extras{Location: &Location{Latitude: "41.0082", Longitude: "28.9784"}}}, ""},
		{"location missing", loc, Params{Body: raw([]string{"a", "b"})}, "template_params.location"},
		{"location bad latitude", loc, Params{Body: raw([]string{"a", "b"}), Extras: Extras{Location: &Location{Latitude: "91", Longitude: "0"}}}, "template_params.location.latitude"},
		{"location bad longitude", loc, Params{Body: raw([]string{"a", "b"}), Extras: Extras{Location: &Location{Latitude: "1", Longitude: "east"}}}, "template_params.location.longitude"},
		{"location on plain", plain, Params{Extras: Extras{Location: &Location{Latitude: "1", Longitude: "1"}}}, "template_params.location"},
		{"otp ok", otp, Params{Extras: Extras{OTPCode: "123456"}}, ""},
		{"otp missing", otp, Params{}, "template_params.otp_code"},
		{"otp 16", otp, Params{Extras: Extras{OTPCode: "1234567890123456"}}, "template_params.otp_code"},
		{"otp on plain", plain, Params{Extras: Extras{OTPCode: "1"}}, "template_params.otp_code"},
		{"flow on plain", plain, Params{Extras: Extras{Flow: &FlowParams{Token: "t"}}}, "template_params.flow"},
	}
	for _, c := range cases {
		err := CheckParams(c.sh, c.p)
		if c.field == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if e, ok := asApp(err); !ok || e != c.field {
			t.Errorf("%s: want error on %s, got %v", c.name, c.field, err)
		}
	}
}

func asApp(err error) (string, bool) {
	e, ok := apperr.As(err)
	if !ok {
		return "", false
	}
	return e.Field, true
}

func TestApplyOffer(t *testing.T) {
	ends := time.Date(2026, 12, 31, 21, 0, 0, 0, time.UTC)
	coupon, img, cta := "XMAS25", "https://cdn.test/xmas.jpg", "https://shop.test/xmas"
	o := db.Offer{ID: uuid.New(), Title: "Yılbaşı", DiscountText: "%25", EndsAt: ends, CouponCode: &coupon, ImageUrl: &img, CtaUrl: &cta}

	lto := ShapeOf(json.RawMessage(goldenLTOComponents), "POSITIONAL", "MARKETING")
	early := int64(5)
	p := ApplyOffer(Params{Extras: Extras{ExpirationTimeMS: &early}}, lto, o)
	if p.ExpirationTimeMS == nil || *p.ExpirationTimeMS != ends.UnixMilli() {
		t.Fatalf("expiration = %v, want the offer's end", p.ExpirationTimeMS)
	}
	if p.CouponCode != coupon || p.HeaderMedia == nil || p.HeaderMedia.Link != img {
		t.Fatalf("coupon/media not filled: %+v", p)
	}
	if p = ApplyOffer(Params{Extras: Extras{CouponCode: "OWN"}}, lto, o); p.CouponCode != "OWN" {
		t.Fatalf("the campaign's own coupon must win, got %q", p.CouponCode)
	}

	// A normal image template: no countdown, the offer flows through
	// placeholders and the image.
	image := ShapeOf(json.RawMessage(`[{"type":"HEADER","format":"IMAGE"},{"type":"BODY","text":"{{1}}: {{2}} until {{3}}"}]`), "POSITIONAL", "MARKETING")
	p = ApplyOffer(Params{Body: raw([]string{"{{offer_title}}", "{{offer_discount}}", "{{offer_ends}}"}), Fallback: map[string]string{"offer_title": "mine"}}, image, o)
	if p.ExpirationTimeMS != nil || p.CouponCode != "" {
		t.Fatalf("a normal template gets no LTO values: %+v", p)
	}
	tpl, miss := Render(image, p, db.Contact{}, "img", "tr")
	if miss != "" {
		t.Fatalf("missing %s", miss)
	}
	got, _ := json.Marshal(tpl.Components)
	want := `[{"type":"header","parameters":[{"type":"image","image":{"link":"https://cdn.test/xmas.jpg"}}]},{"type":"body","parameters":[{"type":"text","text":"mine"},{"type":"text","text":"%25"},{"type":"text","text":"2026-12-31 21:00 UTC"}]}]`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
