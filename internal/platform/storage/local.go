package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Local keeps objects as files under a directory. Keys map to paths
// one-to-one (key "a/b/c" is <root>/a/b/c), so the directory layout of
// older installations (MEDIA_DIR) keeps working. Each object may have a
// "<name>._meta" sidecar holding its content type; keys can never name it
// (ValidateKey refuses that suffix, and segments starting with "_" or ".",
// which keeps the _staging directory out of reach too).
type Local struct {
	root    string
	class   Class
	signer  *signer // presigned URLs (private class)
	public  string  // base URL of public objects (public class)
	staging string
}

const metaSuffix = "._meta"

type localMeta struct {
	ContentType        string `json:"content_type,omitempty"`
	CacheControl       string `json:"cache_control,omitempty"`
	ContentDisposition string `json:"content_disposition,omitempty"`
}

// NewLocal returns a local store rooted at dir. A private store signs
// download URLs with signingKey against appURL; a public store serves
// objects at publicBase (a CDN or appURL+PublicPrefix).
func NewLocal(dir string, class Class, signingKey []byte, appURL, publicBase string) (*Local, error) {
	if dir == "" {
		return nil, errors.New("storage: STORAGE_LOCAL_DIR is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	l := &Local{root: abs, class: class, staging: filepath.Join(abs, "_staging")}
	if class == Private {
		if l.signer, err = newSigner(signingKey, appURL); err != nil {
			return nil, err
		}
	} else {
		l.public = strings.TrimRight(publicBase, "/")
	}
	return l, nil
}

// Root returns the directory objects live in.
func (l *Local) Root() string { return l.root }

func (l *Local) path(key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	p := filepath.Join(l.root, filepath.FromSlash(key))
	// Belt and braces: the validated key cannot escape the root.
	if rel, err := filepath.Rel(l.root, p); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}
	return p, nil
}

// Put implements Store. The body is written to a staging file and renamed
// into place, so readers never see a partial object.
func (l *Local) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string, opts PutOptions) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(l.staging, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(l.staging, "put-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after the rename
	n, err := io.Copy(tmp, ctxReader{ctx, r})
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if size >= 0 && n != size {
		return fmt.Errorf("storage: wrote %d bytes, expected %d", n, size)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	m := localMeta{ContentType: contentType, CacheControl: opts.CacheControl, ContentDisposition: opts.ContentDisposition}
	if m != (localMeta{}) {
		b, _ := json.Marshal(m)
		if err := os.WriteFile(p+metaSuffix, b, 0o640); err != nil { //nolint:gosec // path from a validated key
			return err
		}
	} else {
		_ = os.Remove(p + metaSuffix)
	}
	return os.Rename(tmp.Name(), p)
}

func (l *Local) info(key, p string, st os.FileInfo) Info {
	in := Info{Key: key, Size: st.Size(), ModTime: st.ModTime(), ContentType: "application/octet-stream",
		ETag: fmt.Sprintf(`"%x-%x"`, st.ModTime().UnixNano(), st.Size())}
	if b, err := os.ReadFile(p + metaSuffix); err == nil { //nolint:gosec // path from a validated key
		var m localMeta
		if json.Unmarshal(b, &m) == nil && m.ContentType != "" {
			in.ContentType = m.ContentType
		}
	}
	return in
}

func (l *Local) openSeeker(key string) (io.ReadSeekCloser, Info, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, Info{}, err
	}
	f, err := os.Open(p) //nolint:gosec // path from a validated key
	if errors.Is(err, os.ErrNotExist) {
		return nil, Info{}, ErrNotFound
	}
	if err != nil {
		return nil, Info{}, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, Info{}, err
	}
	if st.IsDir() {
		_ = f.Close()
		return nil, Info{}, ErrNotFound
	}
	return f, l.info(key, p, st), nil
}

// Get implements Store; the reader is an *os.File (seekable).
func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, Info, error) {
	return l.openSeeker(key)
}

// Stat implements Store.
func (l *Local) Stat(_ context.Context, key string) (Info, error) {
	p, err := l.path(key)
	if err != nil {
		return Info{}, err
	}
	st, err := os.Stat(p) //nolint:gosec // path from a validated key under the root
	if errors.Is(err, os.ErrNotExist) || (err == nil && st.IsDir()) {
		return Info{}, ErrNotFound
	}
	if err != nil {
		return Info{}, err
	}
	return l.info(key, p, st), nil
}

// Delete implements Store.
func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	_ = os.Remove(p + metaSuffix)
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// PresignGet implements Store: an HMAC-signed app URL served by Handler.
// For the public class it returns the public URL.
func (l *Local) PresignGet(_ context.Context, key string, ttl time.Duration, opts PresignOptions) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if l.class == Public {
		u, _ := l.PublicURL(key)
		return u, nil
	}
	if ttl <= 0 {
		return "", errors.New("storage: presign ttl must be positive")
	}
	return l.signer.sign(key, ttl, opts), nil
}

// PublicURL implements Store.
func (l *Local) PublicURL(key string) (string, bool) {
	if l.class != Public || ValidateKey(key) != nil {
		return "", false
	}
	return l.public + "/" + escapeKey(key), true
}

// Ping implements Store: the directory must be writable.
func (l *Local) Ping(_ context.Context) error {
	if err := os.MkdirAll(l.staging, 0o750); err != nil {
		return fmt.Errorf("storage directory not writable: %w", err)
	}
	f, err := os.CreateTemp(l.staging, "ping-*")
	if err != nil {
		return fmt.Errorf("storage directory not writable: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

// verifyURL checks a signed request for this store (Handler).
func (l *Local) verifyURL(key string, q url.Values) (ct, cd string, err error) {
	if l.signer == nil {
		return "", "", errSignature
	}
	return l.signer.verify(key, q)
}

// ctxReader stops a copy when the context ends.
type ctxReader struct {
	ctx context.Context //nolint:containedctx // scoped to one copy
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
