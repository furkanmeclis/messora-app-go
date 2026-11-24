package campaigns

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// templateVar matches Meta's template variables: {{1}} or {{first_name}}.
var templateVar = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// Shape is what a template needs filled in at send time.
type Shape struct {
	Named       bool
	HeaderMedia string   // image | video | document | location, "" for text or none
	Header      []string // variables of a text header
	Body        []string
	URLButtons  []int // positions of URL buttons with a variable
}

func vars(s string) []string {
	var out []string
	for _, m := range templateVar.FindAllStringSubmatch(s, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// ShapeOf reads a template's components (Meta's JSON, untrusted).
func ShapeOf(components json.RawMessage, parameterFormat string) Shape {
	sh := Shape{Named: strings.EqualFold(parameterFormat, "NAMED")}
	var comps []struct {
		Type    string `json:"type"`
		Format  string `json:"format"`
		Text    string `json:"text"`
		Buttons []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"buttons"`
	}
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
		case "BUTTONS":
			for i, b := range c.Buttons {
				if strings.EqualFold(b.Type, "URL") && templateVar.MatchString(b.URL) {
					sh.URLButtons = append(sh.URLButtons, i)
				}
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

// CheckParams verifies that p fills everything the template needs, so a
// mismatch is reported before sending instead of failing at Meta for every
// recipient.
func CheckParams(sh Shape, p Params) error {
	switch sh.HeaderMedia {
	case "":
		if p.HeaderMedia != nil {
			return apperr.Validation("template_params.header_media", "This template has no media header; remove the header media.")
		}
	case "image", "video", "document":
		if p.HeaderMedia == nil {
			return apperr.Validation("template_params.header_media", fmt.Sprintf("This template has a %s header: add the %s (upload or https link).", sh.HeaderMedia, sh.HeaderMedia))
		}
		if p.HeaderMedia.Type != sh.HeaderMedia {
			return apperr.Validation("template_params.header_media.type", fmt.Sprintf("This template's header is a %s, not a %s.", sh.HeaderMedia, p.HeaderMedia.Type))
		}
	default:
		return apperr.Validation("template_name", fmt.Sprintf("Templates with a %s header are not supported yet.", sh.HeaderMedia))
	}
	if err := checkValues("template_params.header", sh.Header, sh.Named, p.Header); err != nil {
		return err
	}
	if err := checkValues("template_params.body", sh.Body, sh.Named, p.Body); err != nil {
		return err
	}
	for _, i := range sh.URLButtons {
		if i >= len(p.Buttons) || strings.TrimSpace(p.Buttons[i]) == "" {
			return apperr.Validation(fmt.Sprintf("template_params.buttons.%d", i), fmt.Sprintf("Button %d needs the end of its web address.", i+1))
		}
	}
	return nil
}
