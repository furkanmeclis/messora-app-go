package settings

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// PWA configures the installable web app (manifest and service worker).
type PWA struct {
	Enabled         bool   `json:"enabled"`
	Name            string `json:"name"`
	ShortName       string `json:"short_name"`
	Description     string `json:"description"`
	ThemeColor      string `json:"theme_color"`
	BackgroundColor string `json:"background_color"`
	Display         string `json:"display"`
	StartURL        string `json:"start_url"`
	Icon            string `json:"icon"` // "logo" or "brand"
}

var (
	pwaDisplays = []string{"standalone", "fullscreen", "minimal-ui"}
	pwaIcons    = []string{"brand", "logo"}
	hexColour   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	startURL    = regexp.MustCompile(`^/[A-Za-z0-9/_\-]*$`)
)

// DefaultPWA is on, named after the organisation (empty names follow it).
func DefaultPWA() PWA {
	return PWA{Enabled: true, ThemeColor: "#4f46e5", BackgroundColor: "#ffffff", Display: "standalone", StartURL: "/", Icon: "brand"}
}

// Validate checks every field.
func (p *PWA) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	p.ShortName = strings.TrimSpace(p.ShortName)
	p.Description = strings.TrimSpace(p.Description)
	switch {
	case utf8.RuneCountInString(p.Name) > 60:
		return fmt.Errorf("%w: the app name is limited to 60 characters", ErrInvalid)
	case utf8.RuneCountInString(p.ShortName) > 20:
		return fmt.Errorf("%w: the short name is limited to 20 characters", ErrInvalid)
	case utf8.RuneCountInString(p.Description) > 200:
		return fmt.Errorf("%w: the description is limited to 200 characters", ErrInvalid)
	case !hexColour.MatchString(p.ThemeColor) || !hexColour.MatchString(p.BackgroundColor):
		return fmt.Errorf("%w: app colours must be #rrggbb", ErrInvalid)
	case !slices.Contains(pwaDisplays, p.Display):
		return fmt.Errorf("%w: display must be one of %v", ErrInvalid, pwaDisplays)
	case !slices.Contains(pwaIcons, p.Icon):
		return fmt.Errorf("%w: icon must be logo or brand", ErrInvalid)
	case len(p.StartURL) > 200 || !startURL.MatchString(p.StartURL) || strings.Contains(p.StartURL, "//"):
		return fmt.Errorf("%w: the start page must be a panel path such as /inbox", ErrInvalid)
	}
	return nil
}
