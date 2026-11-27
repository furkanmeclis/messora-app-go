package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Signed local URLs: /media/s/<key>?exp=<unix>&ct=<type>&cd=<disposition>&sig=<mac>.
// The MAC covers the object's tenant, the key, the expiry and every
// response parameter, so none of them can be changed without invalidating
// the link. A link answers only at an address of that tenant (Handler
// compares it with the request host's tenant), and points there: the host
// of the request that created it (WithRequestHost), else PUBLIC_APP_URL.

// SignedPrefix is the route of presigned local downloads.
const SignedPrefix = "/media/s/"

// PublicPrefix is the route of public assets when no CDN is configured.
const PublicPrefix = "/media/p/"

// MinSigningKey is the shortest accepted signing key.
const MinSigningKey = 32

// signer creates and verifies presigned local URLs.
type signer struct {
	key    []byte
	base   string // PUBLIC_APP_URL without a trailing slash
	scheme string // of base, for links on a request's own host
	now    func() time.Time
}

func newSigner(key []byte, baseURL string) (*signer, error) {
	if len(key) < MinSigningKey {
		return nil, errors.New("storage: the media signing key must be at least 32 bytes")
	}
	scheme := "https"
	if u, err := url.Parse(baseURL); err == nil && u.Scheme == "http" {
		scheme = "http"
	}
	return &signer{key: key, base: strings.TrimRight(baseURL, "/"), scheme: scheme, now: time.Now}, nil
}

type hostCtxKey struct{}

// WithRequestHost records the host a request was addressed to (a verified
// address of its tenant), so presigned local links created while serving
// it point back at that address. tenant.Middleware sets it.
func WithRequestHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, hostCtxKey{}, host)
}

// RequestHost returns the host stored by WithRequestHost, "" without one.
func RequestHost(ctx context.Context) string { return requestHost(ctx) }

func requestHost(ctx context.Context) string {
	h, _ := ctx.Value(hostCtxKey{}).(string)
	return h
}

// keyTenant is the tenant an object belongs to: the t/<tenant>/ segment of
// its key, or LegacyTenant for keys from before tenants.
func keyTenant(key string) string {
	if rest, ok := strings.CutPrefix(key, "t/"); ok {
		t, _, _ := strings.Cut(rest, "/")
		return t
	}
	return LegacyTenant
}

func (s *signer) mac(key, exp, ct, cd string) string {
	m := hmac.New(sha256.New, s.key)
	for _, p := range []string{"GET", keyTenant(key), key, exp, ct, cd} {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// sign returns the URL of key valid for ttl, on the host of ctx's request
// when there is one.
func (s *signer) sign(ctx context.Context, key string, ttl time.Duration, o PresignOptions) string {
	exp := strconv.FormatInt(s.now().Add(ttl).Unix(), 10)
	cd := o.header()
	q := url.Values{"exp": {exp}}
	if o.ContentType != "" {
		q.Set("ct", o.ContentType)
	}
	if cd != "" {
		q.Set("cd", cd)
	}
	q.Set("sig", s.mac(key, exp, o.ContentType, cd))
	base := s.base
	if h := requestHost(ctx); h != "" {
		base = s.scheme + "://" + h
	}
	return base + SignedPrefix + escapeKey(key) + "?" + q.Encode()
}

// Errors of verify.
var (
	errExpired   = errors.New("storage: link expired")
	errSignature = errors.New("storage: bad signature")
)

// verify checks a signed request and returns its content type and
// disposition.
func (s *signer) verify(key string, q url.Values) (ct, cd string, err error) {
	exp, ct, cd, sig := q.Get("exp"), q.Get("ct"), q.Get("cd"), q.Get("sig")
	want := s.mac(key, exp, ct, cd)
	if sig == "" || !hmac.Equal([]byte(sig), []byte(want)) {
		return "", "", errSignature
	}
	n, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return "", "", errSignature
	}
	if s.now().Unix() > n {
		return "", "", errExpired
	}
	return ct, cd, nil
}

// escapeKey percent-encodes each key segment for a URL path.
func escapeKey(key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// dispositionHeader builds a Content-Disposition value with an ASCII
// fallback and an RFC 5987 UTF-8 file name.
func dispositionHeader(disposition, filename string) string {
	if disposition == "" {
		return ""
	}
	if disposition != "inline" {
		disposition = "attachment"
	}
	if filename == "" {
		return disposition
	}
	ascii := make([]rune, 0, len(filename))
	for _, r := range filename {
		switch {
		case r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == '/' || r == ';':
			ascii = append(ascii, '_')
		default:
			ascii = append(ascii, r)
		}
	}
	return disposition + `; filename="` + string(ascii) + `"; filename*=UTF-8''` + url.PathEscape(filename)
}

// DispositionHeader is the Content-Disposition value the serving helpers
// send; exported for handlers that stream objects themselves.
func DispositionHeader(disposition, filename string) string {
	return dispositionHeader(disposition, filename)
}
