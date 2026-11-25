package campaigns

import (
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

var placeholder = regexp.MustCompile(`\{\{\s*([a-z0-9_]+(?::[A-Za-z0-9_ .-]+)?)\s*\}\}`)

// value resolves one placeholder for a contact.
func value(key string, c db.Contact) string {
	switch key {
	case "name":
		if c.Name != "" {
			return c.Name
		}
		return c.ProfileName
	case "first_name":
		n := c.Name
		if n == "" {
			n = c.ProfileName
		}
		if f := strings.Fields(n); len(f) > 0 {
			return f[0]
		}
		return ""
	case "phone":
		return c.PhoneE164
	case "email":
		return deref(c.Email)
	case "var1":
		return deref(c.Var1)
	case "var2":
		return deref(c.Var2)
	case "var3":
		return deref(c.Var3)
	case "var4":
		return deref(c.Var4)
	case "var5":
		return deref(c.Var5)
	}
	if attr, ok := strings.CutPrefix(key, "attr:"); ok {
		var m map[string]string
		_ = json.Unmarshal(c.Attributes, &m)
		return m[attr]
	}
	return ""
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// fill substitutes placeholders in s; it returns the first placeholder that
// stayed empty (after fallbacks).
func fill(s string, c db.Contact, fallback map[string]string) (string, string) {
	missing := ""
	out := placeholder.ReplaceAllStringFunc(s, func(m string) string {
		key := placeholder.FindStringSubmatch(m)[1]
		v := strings.TrimSpace(value(key, c))
		if v == "" {
			v = fallback[key]
		}
		if v == "" && missing == "" {
			missing = key
		}
		return v
	})
	if strings.TrimSpace(out) == "" && missing == "" {
		missing = "value"
	}
	return out, missing
}

func parameters(raw json.RawMessage, c db.Contact, fallback map[string]string) ([]whatsapp.TemplateParameter, string) {
	if len(raw) == 0 {
		return nil, ""
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		ps := make([]whatsapp.TemplateParameter, 0, len(list))
		for _, v := range list {
			t, miss := fill(v, c, fallback)
			if miss != "" {
				return nil, miss
			}
			ps = append(ps, whatsapp.TemplateParameter{Type: "text", Text: t})
		}
		return ps, ""
	}
	var named map[string]string
	_ = json.Unmarshal(raw, &named)
	keys := make([]string, 0, len(named))
	for k := range named {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ps := make([]whatsapp.TemplateParameter, 0, len(keys))
	for _, k := range keys {
		t, miss := fill(named[k], c, fallback)
		if miss != "" {
			return nil, miss
		}
		ps = append(ps, whatsapp.TemplateParameter{Type: "text", ParameterName: k, Text: t})
	}
	return ps, ""
}

// mediaParameter is the header parameter of an image, video or document.
func mediaParameter(hm *HeaderMedia) whatsapp.TemplateParameter {
	m := &whatsapp.Media{ID: hm.ID, Link: hm.Link}
	param := whatsapp.TemplateParameter{Type: hm.Type}
	switch hm.Type {
	case "image":
		param.Image = m
	case "video":
		param.Video = m
	case "document":
		m.Filename = hm.Filename
		param.Document = m
	}
	return param
}

type renderer struct {
	c        db.Contact
	fallback map[string]string
	missing  string
}

// text fills placeholders, remembering the first one without a value.
func (r *renderer) text(s string) string {
	v, miss := fill(s, r.c, r.fallback)
	if miss != "" && r.missing == "" {
		r.missing = miss
	}
	return v
}

func (r *renderer) params(raw json.RawMessage) []whatsapp.TemplateParameter {
	ps, miss := parameters(raw, r.c, r.fallback)
	if miss != "" && r.missing == "" {
		r.missing = miss
	}
	return ps
}

// urlButtons fills the dynamic suffixes: buttons[i] belongs to the
// template's button i; an empty entry is a button without a dynamic part
// (quick reply, static URL, phone).
func (r *renderer) urlButtons(buttons []string) []whatsapp.TemplateComponent {
	var out []whatsapp.TemplateComponent
	for i, b := range buttons {
		if strings.TrimSpace(b) == "" {
			continue
		}
		out = append(out, button("url", i, whatsapp.TemplateParameter{Type: "text", Text: r.text(b)}))
	}
	return out
}

func button(subType string, index int, p whatsapp.TemplateParameter) whatsapp.TemplateComponent {
	return whatsapp.TemplateComponent{Type: "button", SubType: subType, Index: strconv.Itoa(index), Parameters: []whatsapp.TemplateParameter{p}}
}

// Render builds the template payload for one contact; sh is the template's
// shape (it places copy code, Flow and one-time password buttons, the
// limited-time offer and the carousel). A non-empty second result names a
// placeholder that had no value.
//
// Components follow the order of Meta's examples: header, body,
// limited_time_offer, buttons by index, carousel.
func Render(sh Shape, p Params, c db.Contact, name, language string) (whatsapp.Template, string) {
	t := whatsapp.Template{Name: name, Language: whatsapp.TemplateLanguage{Code: language}}
	r := &renderer{c: c, fallback: p.Fallback}
	add := func(cs ...whatsapp.TemplateComponent) { t.Components = append(t.Components, cs...) }
	if hm := p.HeaderMedia; hm != nil {
		add(whatsapp.TemplateComponent{Type: "header", Parameters: []whatsapp.TemplateParameter{mediaParameter(hm)}})
	}
	if l := p.Location; l != nil {
		loc := &whatsapp.TemplateLocation{Latitude: strings.TrimSpace(l.Latitude), Longitude: strings.TrimSpace(l.Longitude),
			Name: FillText(l.Name, c, p.Fallback), Address: FillText(l.Address, c, p.Fallback)}
		add(whatsapp.TemplateComponent{Type: "header", Parameters: []whatsapp.TemplateParameter{{Type: "location", Location: loc}}})
	}
	if len(sh.OTP) > 0 {
		// Authentication: the code goes into the body and the button.
		code := r.text(p.OTPCode)
		add(whatsapp.TemplateComponent{Type: "body", Parameters: []whatsapp.TemplateParameter{{Type: "text", Text: code}}},
			button("url", sh.OTP[0], whatsapp.TemplateParameter{Type: "text", Text: code}))
		return t, r.missing
	}
	if ps := r.params(p.Header); len(ps) > 0 {
		add(whatsapp.TemplateComponent{Type: "header", Parameters: ps})
	}
	if ps := r.params(p.Body); len(ps) > 0 {
		add(whatsapp.TemplateComponent{Type: "body", Parameters: ps})
	}
	if sh.LTO && p.ExpirationTimeMS != nil {
		add(whatsapp.TemplateComponent{Type: "limited_time_offer", Parameters: []whatsapp.TemplateParameter{{Type: "limited_time_offer",
			LimitedTimeOffer: &whatsapp.LimitedTimeOffer{ExpirationTimeMS: *p.ExpirationTimeMS}}}})
	}
	buttons := r.urlButtons(p.Buttons)
	if len(sh.CopyCode) > 0 && strings.TrimSpace(p.CouponCode) != "" {
		buttons = append(buttons, button("copy_code", sh.CopyCode[0], whatsapp.TemplateParameter{Type: "coupon_code", CouponCode: r.text(p.CouponCode)}))
	}
	if len(sh.Flow) > 0 {
		action := &whatsapp.FlowAction{FlowToken: "unused"}
		if p.Flow != nil {
			if tok := strings.TrimSpace(p.Flow.Token); tok != "" {
				action.FlowToken = r.text(tok)
			}
			action.FlowActionData = p.Flow.ActionData
		}
		buttons = append(buttons, button("flow", sh.Flow[0], whatsapp.TemplateParameter{Type: "action", Action: action}))
	}
	slices.SortStableFunc(buttons, func(a, b whatsapp.TemplateComponent) int {
		x, _ := strconv.Atoi(a.Index)
		y, _ := strconv.Atoi(b.Index)
		return x - y
	})
	add(buttons...)
	if len(p.Cards) > 0 {
		cards := make([]whatsapp.TemplateCard, len(p.Cards))
		for k, card := range p.Cards {
			cc := whatsapp.TemplateCard{CardIndex: k}
			if card.HeaderMedia != nil {
				cc.Components = append(cc.Components, whatsapp.TemplateComponent{Type: "header",
					Parameters: []whatsapp.TemplateParameter{mediaParameter(card.HeaderMedia)}})
			}
			if ps := r.params(card.Body); len(ps) > 0 {
				cc.Components = append(cc.Components, whatsapp.TemplateComponent{Type: "body", Parameters: ps})
			}
			var qr []int
			if k < len(sh.Cards) {
				qr = sh.Cards[k].QuickReplies
			}
			for i, b := range card.Buttons {
				switch {
				case strings.TrimSpace(b) == "":
				case slices.Contains(qr, i):
					// A quick reply's value is the payload its tap reports.
					cc.Components = append(cc.Components, button("quick_reply", i, whatsapp.TemplateParameter{Type: "payload", Payload: r.text(b)}))
				default:
					cc.Components = append(cc.Components, button("url", i, whatsapp.TemplateParameter{Type: "text", Text: r.text(b)}))
				}
			}
			cards[k] = cc
		}
		add(whatsapp.TemplateComponent{Type: "carousel", Cards: cards})
	}
	return t, r.missing
}

// FillText substitutes contact placeholders in free text; unknown or empty
// placeholders become empty strings. Extra values (e.g. {{message}}) are
// looked up in extra first.
func FillText(s string, c db.Contact, extra map[string]string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		key := placeholder.FindStringSubmatch(m)[1]
		if v, ok := extra[key]; ok {
			return v
		}
		return value(key, c)
	})
}
