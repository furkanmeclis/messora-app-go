package settings

import (
	"errors"
	"strings"
	"testing"
)

func TestThemeColours(t *testing.T) {
	ok := []string{"#fff", "#0a7cff", "#0a7cff80", "oklch(0.62 0.19 259)", "oklch(62% 0.19 259.5 / 0.8)", "oklch( 0.2 0 0 )"}
	bad := []string{
		"red", "#ggg", "#12345", "rgb(1,2,3)", "url(x)", "oklch(0.6 0.1 200); color: red",
		"#fff;}", "oklch(0.6 0.1 200)\n", "var(--x)", "expression(alert(1))", "oklch(1 2 3 4)", "",
	}
	for _, c := range ok {
		th := DefaultTheme()
		th.Light["primary"] = c
		if err := th.Validate(); err != nil {
			t.Errorf("%q rejected: %v", c, err)
		}
	}
	for _, c := range bad {
		th := DefaultTheme()
		th.Dark["primary"] = c
		if err := th.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q accepted", c)
		}
	}
}

func FuzzThemeColourNeverBreaksOut(f *testing.F) {
	for _, s := range []string{"#fff", "oklch(0.5 0.1 100)", "}", ";", "</style>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, c string) {
		th := DefaultTheme()
		th.Light["background"] = c
		if th.Validate() == nil && strings.ContainsAny(c, ";{}<>\"'\\\n:") {
			t.Fatalf("unsafe colour accepted: %q", c)
		}
	})
}

func TestThemeChoices(t *testing.T) {
	th := DefaultTheme()
	if err := th.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Theme){
		func(t *Theme) { t.Preset = "neon" },
		func(t *Theme) { t.Mode = "auto" },
		func(t *Theme) { t.Font = "comic-sans" },
		func(t *Theme) { t.Density = "tiny" },
		func(t *Theme) { t.Wallpaper = "cats" },
		func(t *Theme) { t.Radius = 3 },
		func(t *Theme) { t.Light = map[string]string{"made-up": "#fff"} },
	} {
		th := DefaultTheme()
		mutate(&th)
		if err := th.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted %+v", th)
		}
	}
}

func TestOverridesLockFields(t *testing.T) {
	s := NewStore(nil, 0)
	if err := s.SetOverrides(map[string]string{"mfa_required_roles": "none", "time_format": "12h"}); err != nil {
		t.Fatal(err)
	}
	v := s.withOverrides(Defaults())
	if v.TimeFormat != "12h" || len(v.MFARequiredRoles) != 0 || len(v.Locked) != 2 {
		t.Fatalf("%+v", v)
	}
	locked := Defaults()
	locked.Locked = v.Locked
	if _, err := locked.Apply(Patch{TimeFormat: ptr("24h")}); !errors.Is(err, ErrLocked) {
		t.Fatalf("locked field changed: %v", err)
	}
	for _, bad := range []map[string]string{{"mfa_required_roles": "owner,root"}, {"date_format": "yy"}, {"theme": "x"}} {
		if err := s.SetOverrides(bad); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func TestPWAValidation(t *testing.T) {
	ok := DefaultPWA()
	ok.Name, ok.StartURL = "  Acme Inbox  ", "/inbox"
	if err := ok.Validate(); err != nil || ok.Name != "Acme Inbox" {
		t.Fatalf("valid PWA rejected: %v %q", err, ok.Name)
	}
	for _, mutate := range []func(*PWA){
		func(p *PWA) { p.ThemeColor = "red" },
		func(p *PWA) { p.BackgroundColor = "#fff" },
		func(p *PWA) { p.Display = "browser" },
		func(p *PWA) { p.Icon = "photo" },
		func(p *PWA) { p.StartURL = "https://evil.example" },
		func(p *PWA) { p.StartURL = "//evil.example" },
		func(p *PWA) { p.StartURL = "/inbox?x=<script>" },
		func(p *PWA) { p.ShortName = strings.Repeat("a", 21) },
	} {
		p := DefaultPWA()
		mutate(&p)
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted %+v", p)
		}
	}
}
