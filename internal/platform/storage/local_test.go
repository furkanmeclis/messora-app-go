package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testAppURL = "https://app.messora.test"

var testSigningKey = bytes.Repeat([]byte{42}, 32)

func newTestStores(t *testing.T, cdn string) *Stores {
	t.Helper()
	st, err := Open(context.Background(), Options{Driver: DriverLocal, LocalDir: t.TempDir(), AppURL: testAppURL,
		PublicAssetBaseURL: cdn, SigningKey: testSigningKey})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func put(t *testing.T, st Store, key, body, ct string) {
	t.Helper()
	if err := st.Put(context.Background(), key, strings.NewReader(body), int64(len(body)), ct, PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestLocalPutGetStatDelete(t *testing.T) {
	ctx := context.Background()
	st := newTestStores(t, "")
	key := MustKey("acme", "media", "txt")
	put(t, st.Private, key, "hello world", "text/plain")

	rc, info, err := st.Private.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hello world" || info.Size != 11 || info.ContentType != "text/plain" || info.Key != key || info.ETag == "" {
		t.Fatalf("get: %q %+v", b, info)
	}
	if in, err := st.Private.Stat(ctx, key); err != nil || in.Size != 11 || in.ContentType != "text/plain" {
		t.Fatalf("stat: %+v %v", in, err)
	}
	// Overwrite without a content type drops the old metadata.
	put(t, st.Private, key, "x", "")
	if in, _ := st.Private.Stat(ctx, key); in.ContentType != "application/octet-stream" || in.Size != 1 {
		t.Fatalf("overwrite: %+v", in)
	}
	if err := st.Private.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Private.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stat after delete: %v", err)
	}
	if _, _, err := st.Private.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	if err := st.Private.Delete(ctx, key); err != nil {
		t.Fatalf("deleting a missing object: %v", err)
	}
	// A directory is not an object.
	if _, err := st.Private.Stat(ctx, "t/acme"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("directory: %v", err)
	}
}

func TestLocalPutFailuresLeaveNothing(t *testing.T) {
	ctx := context.Background()
	st := newTestStores(t, "")
	key := MustKey("acme", "media", "")
	// Size mismatch.
	if err := st.Private.Put(ctx, key, strings.NewReader("abc"), 5, "", PutOptions{}); err == nil {
		t.Fatal("size mismatch accepted")
	}
	// A failing reader.
	r := io.MultiReader(strings.NewReader("partial"), errReader{})
	if err := st.Private.Put(ctx, key, r, -1, "", PutOptions{}); err == nil {
		t.Fatal("reader error ignored")
	}
	// A cancelled context.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := st.Private.Put(cctx, key, strings.NewReader("abc"), -1, "", PutOptions{}); err == nil {
		t.Fatal("cancelled put stored")
	}
	if _, err := st.Private.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial object visible: %v", err)
	}
	ents, _ := os.ReadDir(filepath.Join(st.Private.(*Local).Root(), "_staging"))
	if len(ents) != 0 {
		t.Fatalf("staging files left: %d", len(ents))
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestLocalTraversal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	secret := filepath.Join(filepath.Dir(dir), "secret-"+filepath.Base(dir))
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })
	st, err := NewLocal(dir, Private, testSigningKey, testAppURL, "")
	if err != nil {
		t.Fatal(err)
	}
	put(t, st, "t/acme/media/x", "x", "image/png")
	for _, k := range []string{"../" + filepath.Base(secret), "/etc/passwd", "t/acme/../../" + filepath.Base(secret),
		"t/acme/media/x._meta", "_staging/put-1", "t\\..\\x", ""} {
		if _, _, err := st.Get(ctx, k); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("get %q: %v", k, err)
		}
		if err := st.Put(ctx, k, strings.NewReader("x"), 1, "", PutOptions{}); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("put %q: %v", k, err)
		}
		if err := st.Delete(ctx, k); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("delete %q: %v", k, err)
		}
		if _, err := st.PresignGet(ctx, k, time.Minute, PresignOptions{}); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("presign %q: %v", k, err)
		}
	}
	if b, _ := os.ReadFile(secret); string(b) != "secret" {
		t.Fatal("file outside the root was changed")
	}
}

func TestLocalLegacyLayout(t *testing.T) {
	// MEDIA_DIR files written before the object store keep opening.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ab", "cd"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ab", "cd", "abcd-1234"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Open(context.Background(), Options{Driver: DriverLocal, LocalDir: dir, AppURL: testAppURL, SigningKey: testSigningKey})
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTenantKey(LegacyTenant, "ab/cd/abcd-1234"); err != nil {
		t.Fatal(err)
	}
	rc, _, err := st.Private.Get(context.Background(), "ab/cd/abcd-1234")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "old" {
		t.Fatalf("legacy file: %q", b)
	}
}

func TestLocalPing(t *testing.T) {
	st := newTestStores(t, "")
	if err := st.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	l, _ := NewLocal(ro, Private, testSigningKey, testAppURL, "")
	if os.Geteuid() != 0 && l.Ping(context.Background()) == nil {
		t.Fatal("read-only directory reported writable")
	}
}

func TestOpenValidates(t *testing.T) {
	ctx := context.Background()
	if _, err := Open(ctx, Options{Driver: DriverLocal, LocalDir: t.TempDir(), SigningKey: []byte("short")}); err == nil {
		t.Fatal("short signing key accepted")
	}
	if _, err := Open(ctx, Options{Driver: "ftp", LocalDir: t.TempDir()}); err == nil {
		t.Fatal("unknown driver accepted")
	}
	if _, err := Open(ctx, Options{Driver: DriverS3, LocalDir: t.TempDir(), S3: S3Options{Region: "eu-central-1"}}); err == nil {
		t.Fatal("s3 without buckets accepted")
	}
	if _, err := Open(ctx, Options{Driver: DriverLocal, SigningKey: testSigningKey}); err == nil {
		t.Fatal("empty local dir accepted")
	}
}

func TestPublicURL(t *testing.T) {
	key := "t/acme/branding/logo-1"
	st := newTestStores(t, "")
	if u, ok := st.Public.PublicURL(key); !ok || u != testAppURL+"/media/p/"+key {
		t.Fatalf("fallback url %q", u)
	}
	if _, ok := st.Private.PublicURL(key); ok {
		t.Fatal("private store has public URLs")
	}
	cdn := newTestStores(t, "https://cdn.messora.tech/")
	if u, ok := cdn.Public.PublicURL("t/acme/a b.png"); !ok || u != "https://cdn.messora.tech/t/acme/a%20b.png" || !cdn.CDN {
		t.Fatalf("cdn url %q", u)
	}
	if _, ok := cdn.Public.PublicURL("../x"); ok {
		t.Fatal("public URL for an invalid key")
	}
}

func TestPutFunc(t *testing.T) {
	ctx := context.Background()
	st := newTestStores(t, "")
	key := MustKey("acme", "exports", "csv")
	n, err := PutFunc(ctx, st.Private, key, "text/csv", PutOptions{}, func(w io.Writer) error {
		for range 1000 {
			if _, err := io.WriteString(w, "a,b,c\n"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || n != 6000 {
		t.Fatalf("PutFunc: %d %v", n, err)
	}
	if in, _ := st.Private.Stat(ctx, key); in.Size != 6000 || in.ContentType != "text/csv" {
		t.Fatalf("stored %+v", in)
	}
	boom := errors.New("generation failed")
	key2 := MustKey("acme", "exports", "csv")
	if _, err := PutFunc(ctx, st.Private, key2, "", PutOptions{}, func(w io.Writer) error {
		_, _ = io.WriteString(w, "partial")
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("writer error: %v", err)
	}
	if _, err := st.Private.Stat(ctx, key2); !errors.Is(err, ErrNotFound) {
		t.Fatal("failed generation left an object")
	}
	// A failing store unblocks the writer and reports the store's error.
	if _, err := PutFunc(ctx, st.Private, "../bad", "", PutOptions{}, func(w io.Writer) error {
		_, err := w.Write(make([]byte, 1<<20))
		return err
	}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("store error: %v", err)
	}
}

// --- presigned URLs and the /media handler ------------------------------

func get(t *testing.T, h http.Handler, rawURL string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, u.RequestURI(), nil)
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	// The request host's tenant (tenant.Middleware in the app): acme.
	r = r.WithContext(WithTenant(r.Context(), "acme"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// getAs is get at an address of tenant tid ("" = no tenant).
func getAs(t *testing.T, h http.Handler, rawURL, tid string) *httptest.ResponseRecorder {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, u.RequestURI(), nil)
	if tid != "" {
		r = r.WithContext(WithTenant(r.Context(), tid))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// A signed link answers only at its tenant's addresses, points at the
// address it was created for, and cannot be re-signed for another tenant.
func TestSignedLinksAreBoundToTheTenant(t *testing.T) {
	ctx := WithTenant(context.Background(), "acme")
	st := newTestStores(t, "")
	key := MustKey("acme", "media", "")
	put(t, st.Private, key, "x", "")
	u, err := st.Private.PresignGet(ctx, key, time.Minute, PresignOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if w := getAs(t, st.Handler(), u, "acme"); w.Code != 200 {
		t.Fatalf("own tenant: %d", w.Code)
	}
	for _, other := range []string{"evil", ""} {
		if w := getAs(t, st.Handler(), u, other); w.Code != http.StatusForbidden {
			t.Fatalf("at tenant %q: %d", other, w.Code)
		}
	}
	// Another tenant's context cannot sign acme's object.
	if _, err := st.Private.PresignGet(WithTenant(context.Background(), "evil"), key, time.Minute, PresignOptions{}); err == nil {
		t.Fatal("tenant evil signed acme's object")
	}
	// The link points at the request's host.
	u, _ = st.Private.PresignGet(WithRequestHost(ctx, "acme.messora.test"), key, time.Minute, PresignOptions{})
	if !strings.HasPrefix(u, "https://acme.messora.test/media/s/t/acme/") {
		t.Fatalf("link %s", u)
	}
	// Legacy keys (no t/ prefix) belong to tenant #1 only.
	legacy := "media/2024/01/legacy"
	put(t, st.Private, legacy, "y", "")
	u, _ = st.Private.PresignGet(context.Background(), legacy, time.Minute, PresignOptions{})
	if w := getAs(t, st.Handler(), u, LegacyTenant); w.Code != 200 {
		t.Fatalf("legacy at tenant #1: %d", w.Code)
	}
	if w := getAs(t, st.Handler(), u, "acme"); w.Code != http.StatusForbidden {
		t.Fatalf("legacy elsewhere: %d", w.Code)
	}
}

func TestPresignAndServe(t *testing.T) {
	ctx := context.Background()
	st := newTestStores(t, "")
	key := MustKey("acme", "media", "pdf")
	put(t, st.Private, key, "0123456789", "application/pdf")
	u, err := st.Private.PresignGet(ctx, key, time.Minute, PresignOptions{Disposition: "attachment", Filename: "Fatura Ödeme.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, testAppURL+SignedPrefix+key+"?") {
		t.Fatalf("url %q", u)
	}
	h := st.Handler()

	w := get(t, h, u, nil)
	if w.Code != 200 || w.Body.String() != "0123456789" {
		t.Fatalf("download: %d %q", w.Code, w.Body.String())
	}
	hd := w.Header()
	if hd.Get("Content-Type") != "application/pdf" || hd.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(hd.Get("Content-Security-Policy"), "sandbox") || hd.Get("Cache-Control") != "private, max-age=60" ||
		hd.Get("Content-Disposition") != `attachment; filename="Fatura _deme.pdf"; filename*=UTF-8''Fatura%20%C3%96deme.pdf` {
		t.Fatalf("headers: %v", hd)
	}

	// Range requests (audio and video seeking).
	w = get(t, h, u, map[string]string{"Range": "bytes=2-5"})
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" || w.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range: %d %q %v", w.Code, w.Body.String(), w.Header())
	}

	// Tampering with any signed part is refused.
	parsed, _ := url.Parse(u)
	for name, mutate := range map[string]func(q url.Values, p *string){
		"content type": func(q url.Values, _ *string) { q.Set("ct", "text/html") },
		"disposition":  func(q url.Values, _ *string) { q.Set("cd", "inline") },
		"expiry":       func(q url.Values, _ *string) { q.Set("exp", "99999999999") },
		"signature":    func(q url.Values, _ *string) { q.Set("sig", strings.Repeat("A", 43)) },
		"no signature": func(q url.Values, _ *string) { q.Del("sig") },
		"other key":    func(_ url.Values, p *string) { *p = strings.Replace(*p, "/t/acme/", "/t/evil/", 1) },
	} {
		q := parsed.Query()
		p := parsed.Path
		mutate(q, &p)
		w := get(t, h, p+"?"+q.Encode(), nil)
		if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", name, w.Code)
		}
	}

	// A traversal attempt through the signed route.
	if w := get(t, h, "/media/s/../../etc/passwd?exp=1&sig=x", nil); w.Code != 404 && w.Code != 403 {
		t.Fatalf("traversal: %d", w.Code)
	}
	// Methods other than GET/HEAD.
	r := httptest.NewRequest(http.MethodPost, parsed.RequestURI(), nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post: %d", rw.Code)
	}
}

func TestPresignExpiry(t *testing.T) {
	ctx := context.Background()
	st := newTestStores(t, "")
	key := MustKey("acme", "media", "")
	put(t, st.Private, key, "x", "")
	u, _ := st.Private.PresignGet(ctx, key, time.Minute, PresignOptions{})
	if w := get(t, st.Handler(), u, nil); w.Code != 200 {
		t.Fatalf("fresh link: %d", w.Code)
	}
	st.signer.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if w := get(t, st.Handler(), u, nil); w.Code != http.StatusForbidden || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expired link: %d", w.Code)
	}
	if _, err := st.Private.PresignGet(ctx, key, 0, PresignOptions{}); err == nil {
		t.Fatal("zero ttl accepted")
	}
	// A link signed with another key is refused.
	other, _ := newSigner(bytes.Repeat([]byte{1}, 32), testAppURL)
	if w := get(t, st.Handler(), other.sign(ctx, key, time.Minute, PresignOptions{}), nil); w.Code != http.StatusForbidden {
		t.Fatalf("foreign signature: %d", w.Code)
	}
	// A deleted object answers 404 even with a valid link.
	u, _ = st.Private.PresignGet(ctx, key, time.Hour, PresignOptions{})
	_ = st.Private.Delete(ctx, key)
	st.signer.now = time.Now
	if w := get(t, st.Handler(), u, nil); w.Code != http.StatusNotFound {
		t.Fatalf("deleted object: %d", w.Code)
	}
}

func TestPublicRoute(t *testing.T) {
	st := newTestStores(t, "")
	key := "t/acme/branding/logo-abc"
	put(t, st.Public, key, "<svg/>", "image/svg+xml")
	u, _ := st.Public.PublicURL(key)
	w := get(t, st.Handler(), u, nil)
	if w.Code != 200 || w.Body.String() != "<svg/>" || w.Header().Get("Content-Type") != "image/svg+xml" ||
		w.Header().Get("Cache-Control") != "public, max-age=300" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") ||
		w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("public asset: %d %v", w.Code, w.Header())
	}
	// Private objects are never reachable through the public route.
	priv := MustKey("acme", "media", "")
	put(t, st.Private, priv, "secret", "")
	if w := get(t, st.Handler(), PublicPrefix+priv, nil); w.Code != 404 {
		t.Fatalf("private through public route: %d", w.Code)
	}
	for _, p := range []string{PublicPrefix + "ab/cd/x", PublicPrefix + "../x", PublicPrefix + "t/acme/missing", "/media/other"} {
		if w := get(t, st.Handler(), p, nil); w.Code != 404 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	// The public class presigns to its public URL.
	if pu, err := st.Public.PresignGet(context.Background(), key, time.Minute, PresignOptions{}); err != nil || pu != u {
		t.Fatalf("public presign %q %v", pu, err)
	}
}

func TestServeLocal(t *testing.T) {
	st := newTestStores(t, "")
	key := MustKey("acme", "media", "ogg")
	put(t, st.Private, key, "OggS-audio-bytes", "audio/ogg")
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := Serve(w, r, st.Private, r.URL.Query().Get("k"), ServeOptions{ContentType: "audio/ogg", ContentDisposition: "inline"})
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidKey) {
			http.NotFound(w, r)
		}
	})
	w := get(t, h, "/x?k="+url.QueryEscape(key), map[string]string{"Range": "bytes=0-3"})
	if w.Code != http.StatusPartialContent || w.Body.String() != "OggS" || w.Header().Get("Content-Disposition") != "inline" ||
		w.Header().Get("Cache-Control") != "private, max-age=300" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("serve: %d %q %v", w.Code, w.Body.String(), w.Header())
	}
	if w := get(t, h, "/x?k=t/acme/missing", nil); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
	if w := get(t, h, "/x?k=../../etc/passwd", nil); w.Code != 404 {
		t.Fatalf("traversal: %d", w.Code)
	}
}

func TestServeRedirectsForRemoteStores(t *testing.T) {
	st := newTestStores(t, "")
	key := MustKey("acme", "exports", "xlsx")
	put(t, st.Private, key, "xlsx", "")
	var remote Store = struct{ Store }{st.Private} // hides openSeeker
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Serve(w, r, remote, r.URL.Query().Get("k"), ServeOptions{ContentDisposition: "attachment", Redirect: true}); err != nil {
			http.NotFound(w, r)
		}
	})
	w := get(t, h, "/x?k="+url.QueryEscape(key), nil)
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.HasPrefix(loc, testAppURL+SignedPrefix) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("redirect: %d %v", w.Code, w.Header())
	}
	// The presigned link carries the disposition.
	if d := get(t, st.Handler(), loc, nil); d.Code != 200 || d.Header().Get("Content-Disposition") != "attachment" {
		t.Fatalf("follow: %d %v", d.Code, d.Header())
	}
	if w := get(t, h, "/x?k=t/acme/missing", nil); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
}

func TestServeStreamsRemoteStores(t *testing.T) {
	st := newTestStores(t, "")
	key := MustKey("acme", "exports", "csv")
	put(t, st.Private, key, "a,b\n1,2\n", "text/csv")
	var remote Store = struct{ Store }{st.Private} // neither a file opener nor a range getter
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Serve(w, r, remote, r.URL.Query().Get("k"), ServeOptions{ContentDisposition: "attachment"}); err != nil {
			http.NotFound(w, r)
		}
	})
	w := get(t, h, "/x?k="+url.QueryEscape(key), nil)
	if w.Code != 200 || w.Body.String() != "a,b\n1,2\n" || w.Header().Get("Content-Type") != "text/csv" ||
		w.Header().Get("Content-Length") != "8" || w.Header().Get("Content-Disposition") != "attachment" ||
		w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Location") != "" {
		t.Fatalf("stream: %d %q %v", w.Code, w.Body.String(), w.Header())
	}
	r := httptest.NewRequest(http.MethodHead, "/x?k="+url.QueryEscape(key), nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, r)
	if rw.Code != 200 || rw.Body.Len() != 0 || rw.Header().Get("Content-Length") != "8" {
		t.Fatalf("head: %d %v", rw.Code, rw.Header())
	}
	if w := get(t, h, "/x?k=t/acme/missing", nil); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
}

func TestSingleRange(t *testing.T) {
	for in, want := range map[string]string{"bytes=0-9": "bytes=0-9", " bytes=5- ": "bytes=5-", "bytes=0-1,4-5": "", "items=0-1": "", "": ""} {
		if got := singleRange(in); got != want {
			t.Errorf("singleRange(%q) = %q, want %q", in, got, want)
		}
	}
}
