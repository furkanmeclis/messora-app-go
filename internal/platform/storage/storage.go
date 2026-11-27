// Package storage is Messora's object store for files: inbound WhatsApp
// media, contact import files, export files and branding assets.
//
// Two classes of objects exist. Private objects (tenant media, imports,
// exports) are only ever handed out through short-lived presigned URLs or
// streamed by an authenticated handler. Public objects (logos, template and
// campaign header images) are served from PUBLIC_ASSET_BASE_URL, a CDN in
// front of the public bucket, or from the app's /media/p/ route when no CDN
// is configured.
//
// Two backends implement Store: "local" (a directory, the default for
// self-hosted installations) and "s3" (AWS S3, Cloudflare R2, MinIO).
// See docs/STORAGE.md.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// Errors returned by every backend.
var (
	// ErrNotFound means the object does not exist.
	ErrNotFound = errors.New("storage: object not found")
	// ErrInvalidKey means the key is malformed or outside the caller's scope.
	ErrInvalidKey = errors.New("storage: invalid key")
)

// Class tells a private object from a public one.
type Class string

// Object classes.
const (
	Private Class = "private"
	Public  Class = "public"
)

// PutOptions are optional object attributes.
type PutOptions struct {
	// CacheControl is sent with the object (public assets mostly).
	CacheControl string
	// ContentDisposition is stored with the object (S3 returns it on GET).
	ContentDisposition string
}

// Info describes a stored object.
type Info struct {
	Key         string
	Size        int64
	ContentType string
	ModTime     time.Time
	ETag        string
}

// PresignOptions shape the response a presigned URL produces.
type PresignOptions struct {
	// ContentType overrides the stored type ("" keeps it).
	ContentType string
	// Disposition is "inline" or "attachment" ("" sends none).
	Disposition string
	// Filename is offered to the browser with the disposition.
	Filename string
	// ContentDisposition is a complete header value; it wins over
	// Disposition and Filename.
	ContentDisposition string
}

func (o PresignOptions) header() string {
	if o.ContentDisposition != "" {
		return o.ContentDisposition
	}
	return dispositionHeader(o.Disposition, o.Filename)
}

// Store is one bucket (one class of objects).
type Store interface {
	// Put stores r under key. size may be -1 when unknown; the body is
	// streamed, never held in memory.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string, opts PutOptions) error
	// Get opens an object. The caller closes the reader.
	Get(ctx context.Context, key string) (io.ReadCloser, Info, error)
	// Stat describes an object without reading it.
	Stat(ctx context.Context, key string) (Info, error)
	// Delete removes an object; a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// PresignGet returns a URL that downloads the object until ttl passes.
	PresignGet(ctx context.Context, key string, ttl time.Duration, opts PresignOptions) (string, error)
	// PublicURL returns the public address of an object of the public
	// class; ok is false for private stores.
	PublicURL(key string) (url string, ok bool)
	// Ping checks the store is reachable and writable (readiness).
	Ping(ctx context.Context) error
}

// fileOpener is implemented by stores whose objects are local files; the
// serving helpers then answer range requests themselves instead of
// redirecting to a presigned URL.
type fileOpener interface {
	openSeeker(key string) (io.ReadSeekCloser, Info, error)
}

// rangeGetter is implemented by remote stores that can read one byte
// range ("bytes=a-b"); contentRange is the response's Content-Range.
type rangeGetter interface {
	getRange(ctx context.Context, key, rng string) (body io.ReadCloser, info Info, contentRange string, err error)
}

var errRangeNotSatisfiable = errors.New("storage: range not satisfiable")
