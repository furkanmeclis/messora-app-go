package tenant

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

// ReservedSlugs are sub-domain labels the platform keeps for itself. Keep
// in sync with tenant_reserved_slugs() (migration 00024); a test compares
// both lists.
var ReservedSlugs = []string{
	"www", "api", "admin", "app", "hooks", "webhooks", "mail", "smtp", "imap", "pop", "mx",
	"status", "docs", "static", "assets", "cdn", "media", "files", "help", "support", "blog",
	"platform", "console", "dashboard", "auth", "login", "signup", "register", "account",
	"billing", "ns1", "ns2", "ftp", "root", "messora", "system", "realtime", "ws", "connection",
	"internal", "test", "dev", "staging", "demo",
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)

// Slug validation errors.
var (
	ErrSlugFormat   = errors.New("tenant: the address must be 3–40 lowercase letters, digits or inner hyphens")
	ErrSlugReserved = errors.New("tenant: this address is reserved")
)

// ValidateSlug checks a tenant's sub-domain label: lowercase ASCII letters,
// digits and hyphens, 3 to 40 characters, no leading or trailing hyphen, no
// consecutive hyphens (they read as IDNA prefixes), not reserved.
func ValidateSlug(s string) error {
	if !slugRe.MatchString(s) || strings.Contains(s, "--") {
		return ErrSlugFormat
	}
	if slices.Contains(ReservedSlugs, s) {
		return ErrSlugReserved
	}
	return nil
}
