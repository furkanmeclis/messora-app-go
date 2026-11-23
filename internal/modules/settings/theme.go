package settings

import (
	"fmt"
	"regexp"
	"slices"
)

// Theme is the panel's look, chosen by the owner. A preset supplies every
// colour; Light and Dark override individual tokens on top of it.
type Theme struct {
	Preset    string            `json:"preset"`
	Mode      string            `json:"mode"` // default colour scheme: light, dark or system
	Light     map[string]string `json:"light"`
	Dark      map[string]string `json:"dark"`
	Radius    float64           `json:"radius"` // rem
	Font      string            `json:"font"`
	Density   string            `json:"density"`
	Wallpaper string            `json:"wallpaper"` // desktop mode background
}

// Theme choices. The panel defines what each one looks like.
var (
	ThemePresets    = []string{"messora", "midnight", "sand", "forest", "graphite", "ocean", "rose"}
	ThemeModes      = []string{"light", "dark", "system"}
	ThemeFonts      = []string{"geist", "inter", "manrope", "plus-jakarta", "ibm-plex", "space-grotesk"}
	ThemeDensities  = []string{"compact", "comfortable", "spacious"}
	ThemeWallpapers = []string{"aurora", "dunes", "grid", "mesh", "plain", "waves"}
	// ThemeTokens are the colours a theme may override.
	ThemeTokens = []string{
		"background", "foreground", "card", "card-foreground", "popover", "popover-foreground",
		"primary", "primary-foreground", "secondary", "secondary-foreground", "muted", "muted-foreground",
		"accent", "accent-foreground", "destructive", "success", "warning", "border", "input", "ring",
		"sidebar", "sidebar-foreground", "sidebar-primary", "sidebar-primary-foreground",
		"sidebar-accent", "sidebar-accent-foreground", "sidebar-border",
		"chart-1", "chart-2", "chart-3", "chart-4", "chart-5",
	}
)

// Colours are hex (#rgb, #rrggbb, #rrggbbaa) or oklch(L C H [/ A]) only:
// the values are written into a style sheet, so nothing else may pass.
var colour = regexp.MustCompile(`^(#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|#[0-9a-fA-F]{8}|oklch\(\s*[0-9]{1,3}(\.[0-9]{1,4})?%?\s+[0-9](\.[0-9]{1,4})?\s+[0-9]{1,3}(\.[0-9]{1,4})?(\s*/\s*[0-9]{1,3}(\.[0-9]{1,4})?%?)?\s*\))$`)

// DefaultTheme is Messora's own look.
func DefaultTheme() Theme {
	return Theme{
		Preset: "messora", Mode: "system", Light: map[string]string{}, Dark: map[string]string{},
		Radius: 0.75, Font: "geist", Density: "comfortable", Wallpaper: "aurora",
	}
}

// Validate checks every field against the allowed values.
func (t *Theme) Validate() error {
	oneOf := func(field, v string, allowed []string) error {
		if !slices.Contains(allowed, v) {
			return fmt.Errorf("%w: theme %s %q is not one of %v", ErrInvalid, field, v, allowed)
		}
		return nil
	}
	for _, err := range []error{
		oneOf("preset", t.Preset, ThemePresets),
		oneOf("mode", t.Mode, ThemeModes),
		oneOf("font", t.Font, ThemeFonts),
		oneOf("density", t.Density, ThemeDensities),
		oneOf("wallpaper", t.Wallpaper, ThemeWallpapers),
	} {
		if err != nil {
			return err
		}
	}
	if t.Radius < 0 || t.Radius > 2 {
		return fmt.Errorf("%w: theme radius must be between 0 and 2 rem", ErrInvalid)
	}
	for scheme, tokens := range map[string]map[string]string{"light": t.Light, "dark": t.Dark} {
		for k, v := range tokens {
			if !slices.Contains(ThemeTokens, k) {
				return fmt.Errorf("%w: unknown theme colour %q", ErrInvalid, k)
			}
			if !colour.MatchString(v) {
				return fmt.Errorf("%w: theme %s colour %q must be #hex or oklch()", ErrInvalid, scheme, k)
			}
		}
	}
	if t.Light == nil {
		t.Light = map[string]string{}
	}
	if t.Dark == nil {
		t.Dark = map[string]string{}
	}
	return nil
}
