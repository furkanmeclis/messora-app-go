package tenant

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

// Slug limits of new workspaces (frontend/src/lib/auth/slug.ts has the same).
const (
	SlugMin = 3
	SlugMax = 30
)

// ReservedSlugs are sub-domain labels the platform keeps for itself. Keep
// in sync with tenant_reserved_slugs() (migration 00031) and RESERVED_SLUGS
// in frontend/src/lib/auth/slug.ts; tests compare all three.
var ReservedSlugs = []string{
	"www", "admin", "api", "app", "hooks", "dokploy", "cdn", "mail", "docs", "status", "help", "support",
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Slug validation errors.
var (
	ErrSlugFormat   = errors.New("tenant: the address must be 3–30 lowercase letters, digits or inner hyphens")
	ErrSlugReserved = errors.New("tenant: this address is reserved")
)

// ValidateSlug checks a new tenant's sub-domain label: lowercase ASCII
// letters, digits and hyphens, 3 to 30 characters, no leading or trailing
// hyphen, no consecutive hyphens (they read as IDNA prefixes), not
// reserved.
func ValidateSlug(s string) error {
	if len(s) < SlugMin || len(s) > SlugMax || !slugRe.MatchString(s) ||
		strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") || strings.Contains(s, "--") {
		return ErrSlugFormat
	}
	if IsReserved(s) {
		return ErrSlugReserved
	}
	return nil
}

// IsReserved reports whether label is kept for the platform.
func IsReserved(label string) bool { return slices.Contains(ReservedSlugs, label) }
