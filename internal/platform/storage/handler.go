package storage

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ServeOptions are the response headers of a served object.
type ServeOptions struct {
	// ContentType overrides the stored type.
	ContentType string
	// ContentDisposition is the complete header value ("" sends none).
	ContentDisposition string
	// CacheControl of a directly served object (default "private, max-age=300").
	CacheControl string
	// ContentSecurityPolicy of a directly served object
	// (default "default-src 'none'; sandbox").
	ContentSecurityPolicy string
	// Redirect answers objects of remote stores with a redirect to a
	// presigned URL instead of streaming them. Only for routes a browser
	// calls directly: the panel's BFF follows redirects server-side and
	// would carry its own headers to the bucket.
	Redirect bool
	// PresignTTL is the lifetime of the redirect URL (default 5 minutes).
	PresignTTL time.Duration
}

// Serve answers r with the object at key. Local objects are served with
// http.ServeContent (Content-Length, Range and conditional requests).
// Objects of remote stores are streamed, passing a single byte range on
// to the bucket, or, with Redirect, answered with a redirect to a
// short-lived presigned URL. A missing object returns ErrNotFound before
// anything is written.
func Serve(w http.ResponseWriter, r *http.Request, st Store, key string, o ServeOptions) error {
	if o.PresignTTL <= 0 {
		o.PresignTTL = 5 * time.Minute
	}
	h := w.Header()
	if fo, ok := st.(fileOpener); ok {
		f, info, err := fo.openSeeker(key)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		setObjectHeaders(h, o, info)
		// The content type is fixed, so ServeContent does not sniff.
		http.ServeContent(w, r, "", info.ModTime, f)
		return nil
	}
	if o.Redirect {
		if _, err := st.Stat(r.Context(), key); err != nil {
			return err
		}
		u, err := st.PresignGet(r.Context(), key, o.PresignTTL, PresignOptions{ContentType: o.ContentType, ContentDisposition: o.ContentDisposition})
		if err != nil {
			return err
		}
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		http.Redirect(w, r, u, http.StatusFound) //nolint:gosec // u is a presigned URL of our own bucket, not request input
		return nil
	}
	if r.Method == http.MethodHead {
		info, err := st.Stat(r.Context(), key)
		if err != nil {
			return err
		}
		setObjectHeaders(h, o, info)
		h.Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.WriteHeader(http.StatusOK)
		return nil
	}
	var (
		body   io.ReadCloser
		info   Info
		cr     string
		err    error
		ranged bool
	)
	if rg, ok := st.(rangeGetter); ok {
		if rng := singleRange(r.Header.Get("Range")); rng != "" {
			body, info, cr, err = rg.getRange(r.Context(), key, rng)
			ranged = true
			if errors.Is(err, errRangeNotSatisfiable) {
				h.Set("Content-Range", "bytes */*")
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return nil
			}
		}
	}
	if !ranged {
		body, info, err = st.Get(r.Context(), key)
	}
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	setObjectHeaders(h, o, info)
	h.Set("Content-Length", strconv.FormatInt(info.Size, 10))
	status := http.StatusOK
	if cr != "" {
		h.Set("Content-Range", cr)
		status = http.StatusPartialContent
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, body)
	return nil
}

// singleRange returns a "bytes=a-b" header with one range, or "".
func singleRange(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "bytes=") || strings.Contains(v, ",") || len(v) > 64 {
		return ""
	}
	return v
}

func setObjectHeaders(h http.Header, o ServeOptions, info Info) {
	ct := o.ContentType
	if ct == "" {
		ct = info.ContentType
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	h.Set("Content-Type", ct)
	if o.ContentDisposition != "" {
		h.Set("Content-Disposition", o.ContentDisposition)
	}
	h.Set("Accept-Ranges", "bytes")
	if info.ETag != "" {
		h.Set("ETag", info.ETag)
	}
	if !info.ModTime.IsZero() {
		h.Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
	}
	setSafetyHeaders(h, o.CacheControl, o.ContentSecurityPolicy, "private, max-age=300")
}

func setSafetyHeaders(h http.Header, cache, csp, defCache string) {
	if cache == "" {
		cache = defCache
	}
	if csp == "" {
		csp = "default-src 'none'; sandbox"
	}
	h.Set("Cache-Control", cache)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", csp)
	h.Set("Referrer-Policy", "no-referrer")
}

// Handler serves the /media/ routes:
//
//   - GET /media/s/<key>?exp&ct&cd&sig: presigned downloads of the local
//     driver (signature and expiry checked; 403 otherwise). The route must
//     run behind the tenant middleware: a link answers only at an address
//     of the object's tenant (403 elsewhere, and without a tenant).
//   - GET /media/p/<key>: public assets when PUBLIC_ASSET_BASE_URL is not
//     set (served from the local public directory, or redirected to a
//     presigned URL of the public bucket).
//
// Every response carries nosniff, a sandboxing CSP and a short cache.
func (s *Stores) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch p := r.URL.Path; {
		case strings.HasPrefix(p, SignedPrefix):
			s.serveSigned(w, r, strings.TrimPrefix(p, SignedPrefix))
		case strings.HasPrefix(p, PublicPrefix):
			s.servePublic(w, r, strings.TrimPrefix(p, PublicPrefix))
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *Stores) serveSigned(w http.ResponseWriter, r *http.Request, key string) {
	loc, ok := s.Private.(*Local)
	if !ok || ValidateKey(key) != nil {
		http.NotFound(w, r)
		return
	}
	ct, cd, err := loc.verifyURL(key, r.URL.Query())
	if err == nil && CheckTenantKey(TenantKey(r.Context()), key) != nil {
		err = errSignature // another tenant's link, or no tenant at all
	}
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "this link is invalid or has expired", http.StatusForbidden)
		return
	}
	f, info, err := loc.openSeeker(key)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	if ct == "" {
		ct = info.ContentType
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	if cd != "" {
		h.Set("Content-Disposition", cd)
	}
	setSafetyHeaders(h, "private, max-age=60", "", "")
	http.ServeContent(w, r, "", info.ModTime, f)
}

func (s *Stores) servePublic(w http.ResponseWriter, r *http.Request, key string) {
	// Public assets are tenant-scoped; nothing else is reachable here.
	if ValidateKey(key) != nil || !strings.HasPrefix(key, "t/") {
		http.NotFound(w, r)
		return
	}
	// Browsers call this route directly, so remote assets may redirect.
	err := Serve(w, r, s.Public, key, ServeOptions{CacheControl: "public, max-age=300", Redirect: true, PresignTTL: time.Hour})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidKey) {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "storage unavailable", http.StatusBadGateway)
	}
}
