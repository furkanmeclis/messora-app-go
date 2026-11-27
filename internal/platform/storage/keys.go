package storage

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultTenant owns everything stored before multi-tenancy, and every
// call made without a tenant in its context.
const DefaultTenant = "default"

type tenantCtxKey struct{}

// WithTenant returns a context whose objects belong to tenantID.
//
// Until the tenancy work package wires the real tenant into request and
// job contexts, nothing calls this outside tests and every key falls under
// DefaultTenant.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenantID)
}

// TenantKey returns the tenant id the context carries, or DefaultTenant
// when it carries none. A malformed id is returned as is, so Key and
// CheckTenantKey fail closed instead of falling back to the default tenant.
func TenantKey(ctx context.Context) string {
	if v, ok := ctx.Value(tenantCtxKey{}).(string); ok {
		return v
	}
	return DefaultTenant
}

// Key builds a new tenant-scoped object key:
//
//	t/<tenant>/<kind>/<yyyy>/<mm>/<uuid>[.<ext>]
//
// ext may be given with or without its dot, or empty.
func Key(tenantID, kind, ext string) (string, error) {
	return keyAt(tenantID, kind, ext, time.Now().UTC())
}

func keyAt(tenantID, kind, ext string, now time.Time) (string, error) {
	if !validSegment(tenantID) {
		return "", fmt.Errorf("%w: tenant %q", ErrInvalidKey, tenantID)
	}
	if !validSegment(kind) {
		return "", fmt.Errorf("%w: kind %q", ErrInvalidKey, kind)
	}
	ext = strings.TrimPrefix(ext, ".")
	if ext != "" && (!validSegment(ext) || len(ext) > 10) {
		return "", fmt.Errorf("%w: extension %q", ErrInvalidKey, ext)
	}
	k := fmt.Sprintf("t/%s/%s/%04d/%02d/%s", tenantID, kind, now.Year(), int(now.Month()), uuid.New())
	if ext != "" {
		k += "." + ext
	}
	return k, nil
}

// MustKey is Key for constant arguments.
func MustKey(tenantID, kind, ext string) string {
	k, err := Key(tenantID, kind, ext)
	if err != nil {
		panic(err)
	}
	return k
}

// TenantPrefix is the key prefix of a tenant's objects.
func TenantPrefix(tenantID string) string { return "t/" + tenantID + "/" }

// CheckTenantKey rejects a key that does not belong to tenantID. Keys
// stored before tenant-scoped keys existed (no "t/" prefix) belong to
// DefaultTenant only.
func CheckTenantKey(tenantID, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if !validSegment(tenantID) {
		return fmt.Errorf("%w: tenant %q", ErrInvalidKey, tenantID)
	}
	if strings.HasPrefix(key, "t/") {
		if !strings.HasPrefix(key, TenantPrefix(tenantID)) || len(key) == len(TenantPrefix(tenantID)) {
			return fmt.Errorf("%w: key outside the tenant's prefix", ErrInvalidKey)
		}
		return nil
	}
	if tenantID != DefaultTenant {
		return fmt.Errorf("%w: key outside the tenant's prefix", ErrInvalidKey)
	}
	return nil
}

// ValidateKey checks a key is a clean, relative, slash-separated path
// that cannot escape a bucket or directory: no "..", no ".", no empty or
// absolute segments, no backslashes or control characters, no segment
// starting with "_" or "." and none ending in "._meta" (reserved by the
// local backend).
func ValidateKey(key string) error {
	if key == "" || len(key) > 512 {
		return fmt.Errorf("%w: empty or too long", ErrInvalidKey)
	}
	if strings.HasPrefix(key, "/") || strings.ContainsAny(key, "\\\x00") || path.Clean(key) != key {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	for seg := range strings.SplitSeq(key, "/") {
		if seg == "" || seg == "." || seg == ".." || seg[0] == '_' || seg[0] == '.' || strings.HasSuffix(seg, metaSuffix) {
			return fmt.Errorf("%w: %q", ErrInvalidKey, key)
		}
		for _, r := range seg {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("%w: %q", ErrInvalidKey, key)
			}
		}
	}
	return nil
}

// validSegment allows [A-Za-z0-9-] only (tenant ids, kinds, extensions).
func validSegment(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		ok := r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}

var extensions = map[string]string{
	"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif", "image/svg+xml": "svg",
	"video/mp4": "mp4", "video/3gpp": "3gp",
	"audio/ogg": "ogg", "audio/mpeg": "mp3", "audio/aac": "aac", "audio/mp4": "m4a", "audio/amr": "amr", "audio/opus": "opus",
	"application/pdf": "pdf", "text/plain": "txt", "text/csv": "csv",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "docx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/msword": "doc", "application/vnd.ms-excel": "xls", "application/vnd.ms-powerpoint": "ppt",
}

// ExtFor returns a file extension (without the dot) for a media type, or
// "" when none is known.
func ExtFor(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return extensions[ct]
}
