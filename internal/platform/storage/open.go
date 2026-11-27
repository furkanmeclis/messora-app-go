package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Drivers.
const (
	DriverLocal = "local"
	DriverS3    = "s3"
)

// Options select and configure the backend (see internal/config).
type Options struct {
	Driver string
	// LocalDir holds private objects (local driver), public ones under
	// LocalDir/_public, and the scratch directory LocalDir/_tmp (both
	// drivers: the containers' root filesystem is read-only).
	LocalDir string
	// AppURL is PUBLIC_APP_URL: signed local links and the public asset
	// fallback route are served there.
	AppURL string
	// PublicAssetBaseURL is the CDN in front of the public bucket; empty
	// serves public assets from AppURL+PublicPrefix.
	PublicAssetBaseURL string
	// SigningKey signs local download links.
	SigningKey []byte

	S3            S3Options
	BucketPrivate string
	BucketPublic  string
}

// Stores are the two buckets plus a scratch directory.
type Stores struct {
	Private Store
	Public  Store
	Driver  string
	// Scratch is a writable local directory for temporary files (XLSX
	// conversion and generation).
	Scratch string
	// CDN reports whether public URLs point at PUBLIC_ASSET_BASE_URL.
	CDN bool

	signer *signer // set for the local driver
}

// Open builds the stores of o.
func Open(ctx context.Context, o Options) (*Stores, error) {
	if o.LocalDir == "" {
		return nil, errors.New("storage: STORAGE_LOCAL_DIR is required (scratch space for every driver)")
	}
	appURL := strings.TrimRight(o.AppURL, "/")
	publicBase := strings.TrimRight(o.PublicAssetBaseURL, "/")
	st := &Stores{Driver: o.Driver, Scratch: filepath.Join(o.LocalDir, "_tmp"), CDN: publicBase != ""}
	if publicBase == "" {
		publicBase = appURL + strings.TrimSuffix(PublicPrefix, "/")
	}
	switch o.Driver {
	case DriverLocal, "":
		st.Driver = DriverLocal
		priv, err := NewLocal(o.LocalDir, Private, o.SigningKey, appURL, "")
		if err != nil {
			return nil, err
		}
		pub, err := NewLocal(filepath.Join(o.LocalDir, "_public"), Public, nil, appURL, publicBase)
		if err != nil {
			return nil, err
		}
		st.Private, st.Public, st.signer = priv, pub, priv.signer
	case DriverS3:
		if o.BucketPrivate == "" || o.BucketPublic == "" {
			return nil, errors.New("storage: S3_BUCKET_PRIVATE and S3_BUCKET_PUBLIC are required with STORAGE_DRIVER=s3")
		}
		client, err := NewS3Client(ctx, o.S3)
		if err != nil {
			return nil, err
		}
		priv, err := NewS3(client, o.BucketPrivate, Private, o.S3, "")
		if err != nil {
			return nil, err
		}
		pub, err := NewS3(client, o.BucketPublic, Public, o.S3, publicBase)
		if err != nil {
			return nil, err
		}
		st.Private, st.Public = priv, pub
	default:
		return nil, fmt.Errorf("storage: unknown STORAGE_DRIVER %q (local or s3)", o.Driver)
	}
	if err := os.MkdirAll(st.Scratch, 0o750); err != nil {
		return nil, fmt.Errorf("storage: scratch directory: %w", err)
	}
	return st, nil
}

// Ping checks both buckets (readiness).
func (s *Stores) Ping(ctx context.Context) error {
	if err := s.Private.Ping(ctx); err != nil {
		return fmt.Errorf("private: %w", err)
	}
	if err := s.Public.Ping(ctx); err != nil {
		return fmt.Errorf("public: %w", err)
	}
	return nil
}

// Check is a readiness check (health.Check signature).
func (s *Stores) Check(ctx context.Context) (any, error) {
	if err := s.Ping(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"driver": s.Driver, "cdn": s.CDN}, nil
}

// PutFunc streams what write produces into key without a temporary file
// (through a pipe) and returns the number of bytes written. When write
// fails the upload is abandoned and nothing is stored.
func PutFunc(ctx context.Context, st Store, key, contentType string, opts PutOptions, write func(w io.Writer) error) (int64, error) {
	pr, pw := io.Pipe()
	cw := &countWriter{w: pw}
	done := make(chan error, 1)
	go func() {
		err := write(cw)
		_ = pw.CloseWithError(err) // nil closes normally (EOF)
		done <- err
	}()
	err := st.Put(ctx, key, pr, -1, contentType, opts)
	_ = pr.CloseWithError(errUploadEnded) // unblocks a writer after a failed Put
	werr := <-done
	if werr != nil && !errors.Is(werr, errUploadEnded) {
		return cw.n, werr // the cause; Put only saw the broken pipe
	}
	if err != nil {
		return cw.n, err
	}
	return cw.n, werr
}

var errUploadEnded = errors.New("storage: upload ended")

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
