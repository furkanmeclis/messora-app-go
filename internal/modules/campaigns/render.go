package campaigns

import (
	"encoding/json"
	"regexp"
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

// Render builds the template payload for one contact. A non-empty second
// result names a placeholder that had no value.
func Render(p Params, c db.Contact, name, language string) (whatsapp.Template, string) {
	t := whatsapp.Template{Name: name, Language: whatsapp.TemplateLanguage{Code: language}}
	if ps, miss := parameters(p.Header, c, p.Fallback); miss != "" {
		return t, miss
	} else if len(ps) > 0 {
		t.Components = append(t.Components, whatsapp.TemplateComponent{Type: "header", Parameters: ps})
	}
	if ps, miss := parameters(p.Body, c, p.Fallback); miss != "" {
		return t, miss
	} else if len(ps) > 0 {
		t.Components = append(t.Components, whatsapp.TemplateComponent{Type: "body", Parameters: ps})
	}
	for i, b := range p.Buttons {
		v, miss := fill(b, c, p.Fallback)
		if miss != "" {
			return t, miss
		}
		t.Components = append(t.Components, whatsapp.TemplateComponent{Type: "button", SubType: "url", Index: strconv.Itoa(i),
			Parameters: []whatsapp.TemplateParameter{{Type: "text", Text: v}}})
	}
	return t, ""
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
