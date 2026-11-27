//go:build integration

package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

// MinIO no longer publishes community images on Docker Hub or Quay; the
// Chainguard build is pinned by digest.
const minioImage = "cgr.dev/chainguard/minio@sha256:b288eaa709ba2f799a39e9a8ceda71a08fabf90109b12c90badba9afff29485c"

const (
	minioUser = "messora"
	minioPass = "messora-test-secret" // gitleaks:allow (test fixture)
	// A static KMS key so SSE-S3 (AES256) works as on AWS.
	minioKMS = "messora-key:MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // gitleaks:allow (test fixture)
)

func startMinIO(t *testing.T) (endpoint string) {
	t.Helper()
	ctx := context.Background()
	c, err := tcminio.Run(ctx, minioImage, tcminio.WithUsername(minioUser), tcminio.WithPassword(minioPass),
		testcontainers.WithEnv(map[string]string{"MINIO_KMS_SECRET_KEY": minioKMS}))
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	hostPort, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return "http://" + hostPort
}

func s3Options(endpoint string) S3Options {
	return S3Options{Region: "eu-central-1", Endpoint: endpoint, ForcePathStyle: true, AccessKeyID: minioUser,
		SecretAccessKey: minioPass, PartSize: 5 << 20}
}

func TestS3Backend(t *testing.T) {
	ctx := context.Background()
	endpoint := startMinIO(t)
	o := s3Options(endpoint)
	client, err := NewS3Client(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"messora-private", "messora-public"} {
		if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(b)}); err != nil {
			t.Fatalf("create bucket %s: %v", b, err)
		}
	}
	st, err := Open(ctx, Options{Driver: DriverS3, LocalDir: t.TempDir(), AppURL: "https://app.messora.test",
		PublicAssetBaseURL: "https://cdn.messora.test", S3: o, BucketPrivate: "messora-private", BucketPublic: "messora-public"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ping", func(t *testing.T) {
		if err := st.Ping(ctx); err != nil {
			t.Fatal(err)
		}
		missing, _ := NewS3(client, "no-such-bucket", Private, o, "")
		if missing.Ping(ctx) == nil {
			t.Fatal("missing bucket reported healthy")
		}
	})

	t.Run("put get stat delete", func(t *testing.T) {
		key := MustKey("acme", "media", "pdf")
		body := "%PDF-1.7 fake"
		if err := st.Private.Put(ctx, key, strings.NewReader(body), int64(len(body)), "application/pdf", PutOptions{}); err != nil {
			t.Fatal(err)
		}
		rc, info, err := st.Private.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(b) != body || info.Size != int64(len(body)) || info.ContentType != "application/pdf" || info.ETag == "" {
			t.Fatalf("get: %q %+v", b, info)
		}
		head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String("messora-private"), Key: aws.String(key)})
		if err != nil || head.ServerSideEncryption != "AES256" {
			t.Fatalf("server-side encryption: %v %v", head.ServerSideEncryption, err)
		}
		if in, err := st.Private.Stat(ctx, key); err != nil || in.Size != int64(len(body)) {
			t.Fatalf("stat: %+v %v", in, err)
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
			t.Fatalf("delete missing: %v", err)
		}
		if err := st.Private.Put(ctx, "../escape", strings.NewReader("x"), 1, "", PutOptions{}); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("invalid key: %v", err)
		}
	})

	t.Run("multipart of unknown size", func(t *testing.T) {
		key := MustKey("acme", "exports", "bin")
		data := make([]byte, 12<<20+123) // three parts of 5 MiB
		_, _ = rand.Read(data)
		n, err := PutFunc(ctx, st.Private, key, "application/octet-stream", PutOptions{}, func(w io.Writer) error {
			_, err := io.Copy(w, bytes.NewReader(data))
			return err
		})
		if err != nil || n != int64(len(data)) {
			t.Fatalf("PutFunc: %d %v", n, err)
		}
		rc, info, err := st.Private.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(rc)
		_ = rc.Close()
		if !bytes.Equal(got, data) || info.Size != int64(len(data)) || !strings.Contains(info.ETag, "-") {
			t.Fatalf("multipart object: %d bytes, etag %s", len(got), info.ETag)
		}
		// A generation failure mid-way stores nothing (the upload is aborted).
		key2 := MustKey("acme", "exports", "bin")
		boom := errors.New("generation failed")
		if _, err := PutFunc(ctx, st.Private, key2, "", PutOptions{}, func(w io.Writer) error {
			_, _ = w.Write(data[:7<<20])
			return boom
		}); !errors.Is(err, boom) {
			t.Fatalf("writer error: %v", err)
		}
		if _, err := st.Private.Stat(ctx, key2); !errors.Is(err, ErrNotFound) {
			t.Fatalf("aborted upload visible: %v", err)
		}
		ups, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{Bucket: aws.String("messora-private")})
		if err != nil || len(ups.Uploads) != 0 {
			t.Fatalf("orphaned multipart uploads: %d %v", len(ups.Uploads), err)
		}
	})

	t.Run("presign", func(t *testing.T) {
		key := MustKey("acme", "media", "ogg")
		body := "0123456789"
		if err := st.Private.Put(ctx, key, strings.NewReader(body), int64(len(body)), "audio/ogg", PutOptions{}); err != nil {
			t.Fatal(err)
		}
		u, err := st.Private.PresignGet(ctx, key, time.Minute, PresignOptions{ContentType: "audio/ogg", Disposition: "attachment", Filename: "ses kaydı.ogg"})
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.Get(u) //nolint:noctx // test
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != 200 || string(b) != body || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment; filename=") ||
			res.Header.Get("Content-Type") != "audio/ogg" {
			t.Fatalf("presigned get: %d %q %v", res.StatusCode, b, res.Header)
		}
		// Range requests are answered by the bucket.
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("Range", "bytes=2-5")
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ = io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusPartialContent || string(b) != "2345" {
			t.Fatalf("range: %d %q", res.StatusCode, b)
		}
		// A tampered URL is refused.
		pu, _ := url.Parse(u)
		q := pu.Query()
		q.Set("response-content-type", "text/html")
		pu.RawQuery = q.Encode()
		res, err = http.Get(pu.String()) //nolint:noctx // test
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("tampered presign: %d", res.StatusCode)
		}
	})

	t.Run("serve", func(t *testing.T) {
		key := MustKey("acme", "media", "mp4")
		if err := st.Private.Put(ctx, key, strings.NewReader("0123456789"), 10, "video/mp4", PutOptions{}); err != nil {
			t.Fatal(err)
		}
		var redirect bool
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := Serve(w, r, st.Private, r.URL.Query().Get("k"), ServeOptions{ContentDisposition: "inline", Redirect: redirect}); err != nil {
				http.NotFound(w, r)
			}
		})
		do := func(path, rng string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if rng != "" {
				req.Header.Set("Range", rng)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			return w
		}
		path := "/x?k=" + url.QueryEscape(key)
		// Streamed through the API by default, ranges passed to the bucket.
		if w := do(path, ""); w.Code != 200 || w.Body.String() != "0123456789" || w.Header().Get("Content-Type") != "video/mp4" ||
			w.Header().Get("Content-Length") != "10" || w.Header().Get("Content-Disposition") != "inline" {
			t.Fatalf("stream: %d %q %v", w.Code, w.Body.String(), w.Header())
		}
		if w := do(path, "bytes=3-6"); w.Code != http.StatusPartialContent || w.Body.String() != "3456" ||
			w.Header().Get("Content-Range") != "bytes 3-6/10" || w.Header().Get("Content-Length") != "4" {
			t.Fatalf("range: %d %q %v", w.Code, w.Body.String(), w.Header())
		}
		if w := do(path, "bytes=50-60"); w.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Fatalf("unsatisfiable range: %d", w.Code)
		}
		if w := do("/x?k=t/acme/missing", ""); w.Code != 404 {
			t.Fatalf("missing: %d", w.Code)
		}
		// Redirect mode for routes browsers call directly.
		redirect = true
		if w := do(path, ""); w.Code != http.StatusFound || !strings.HasPrefix(w.Header().Get("Location"), endpoint+"/messora-private/") {
			t.Fatalf("redirect: %d %v", w.Code, w.Header())
		}
		if w := do("/x?k=t/acme/missing", ""); w.Code != 404 {
			t.Fatalf("missing (redirect): %d", w.Code)
		}
	})

	t.Run("public", func(t *testing.T) {
		key := "t/acme/branding/logo-1"
		if err := st.Public.Put(ctx, key, strings.NewReader("png"), 3, "image/png",
			PutOptions{CacheControl: "public, max-age=31536000, immutable"}); err != nil {
			t.Fatal(err)
		}
		if u, ok := st.Public.PublicURL(key); !ok || u != "https://cdn.messora.test/"+key {
			t.Fatalf("public url %q", u)
		}
		if _, ok := st.Private.PublicURL(key); ok {
			t.Fatal("private bucket has public URLs")
		}
		head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String("messora-public"), Key: aws.String(key)})
		if err != nil || aws.ToString(head.CacheControl) != "public, max-age=31536000, immutable" {
			t.Fatalf("cache control: %v", err)
		}
		// Without a CDN the app route redirects to the bucket.
		w := httptest.NewRecorder()
		st.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, PublicPrefix+key, nil))
		if w.Code != http.StatusFound || !strings.HasPrefix(w.Header().Get("Location"), endpoint+"/messora-public/") {
			t.Fatalf("public route: %d %v", w.Code, w.Header())
		}
		// Signed local links do not exist with the s3 driver.
		w = httptest.NewRecorder()
		st.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, SignedPrefix+key+"?exp=1&sig=x", nil))
		if w.Code != 404 {
			t.Fatalf("signed route with s3: %d", w.Code)
		}
	})
}
