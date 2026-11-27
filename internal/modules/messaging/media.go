package messaging

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// MediaStore keeps inbound media in the private object store. New keys are
// tenant-scoped (t/<tenant>/media/<yyyy>/<mm>/<uuid>.<ext>); keys written
// before (aa/bb/<uuid>-<hex> under MEDIA_DIR) still open with the local
// driver.
type MediaStore struct{ st storage.Store }

// NewMediaStore returns a media store on st.
func NewMediaStore(st storage.Store) *MediaStore { return &MediaStore{st: st} }

// Store returns the underlying object store.
func (m *MediaStore) Store() storage.Store { return m.st }

// Open returns a stored file of the context's tenant.
func (m *MediaStore) Open(ctx context.Context, key string) (io.ReadCloser, storage.Info, error) {
	if err := storage.CheckTenantKey(storage.TenantKey(ctx), key); err != nil {
		return nil, storage.Info{}, err
	}
	return m.st.Get(ctx, key)
}

// Check rejects a key outside the context's tenant.
func (m *MediaStore) Check(ctx context.Context, key string) error {
	return storage.CheckTenantKey(storage.TenantKey(ctx), key)
}

// mediaType picks the stored content type: the declared one, unless the
// bytes are markup or text that a browser would render while the declared
// type is not text (HTML sent as an "image", say).
func mediaType(declared string, head []byte) string {
	declared = strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	sniffed := strings.SplitN(http.DetectContentType(head), ";", 2)[0]
	switch {
	case declared == "":
		return sniffed
	case strings.HasPrefix(sniffed, "text/") && sniffed != "text/plain" && !strings.HasPrefix(declared, "text/"):
		return sniffed
	}
	return declared
}

// DownloadMedia fetches inbound media from Meta into the object store,
// streaming it (the file is never held in memory). Permanent problems mark
// the file failed; transient ones are returned so the task is retried.
func (s *Service) DownloadMedia(ctx context.Context, id uuid.UUID) error {
	f, err := s.q.GetMediaFile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && f.Status == "stored") {
		return nil
	}
	if err != nil {
		return err
	}
	if f.MessageID == nil || f.MetaMediaID == nil {
		return s.q.MarkMediaFailed(ctx, db.MarkMediaFailedParams{ID: id, LastError: ptr("no Meta media id")})
	}
	msg, err := s.q.GetMessage(ctx, *f.MessageID)
	if err != nil {
		return err
	}
	sender, err := s.ch.SenderFor(ctx, msg.PhoneNumberID)
	if err != nil {
		return s.q.MarkMediaFailed(ctx, db.MarkMediaFailedParams{ID: id, LastError: ptr(err.Error())})
	}
	client := s.ch.Client()
	info, err := client.GetMedia(ctx, *f.MetaMediaID, sender.Token)
	if err != nil {
		var api *whatsapp.APIError
		if errors.As(err, &api) && api.HTTPStatus < 500 && api.HTTPStatus != 429 {
			return s.q.MarkMediaFailed(ctx, db.MarkMediaFailedParams{ID: id, LastError: ptr(truncate(api.Error(), 500))})
		}
		return err
	}
	declared := f.MimeType
	if declared == "" {
		declared = info.MimeType
	}
	key, err := storage.Key(storage.TenantKey(ctx), "media", storage.ExtFor(declared))
	if err != nil {
		return err
	}
	// The object is uploaded while it downloads. Its content type comes
	// from the declared type and the first 512 bytes, peeked before the
	// upload starts.
	h := sha256.New()
	var n int64
	var tooBig bool
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		var derr error
		n, derr = client.DownloadMedia(ctx, info.URL, sender.Token, io.MultiWriter(pw, h), s.maxMedia)
		tooBig = derr != nil && n > s.maxMedia
		_ = pw.CloseWithError(derr)
	}()
	br := bufio.NewReaderSize(pr, 512)
	head, _ := br.Peek(512) // the first bytes (or all of a smaller file)
	perr := s.media.st.Put(ctx, key, br, -1, mediaType(declared, head), storage.PutOptions{})
	_ = pr.CloseWithError(errors.New("upload ended")) // unblocks the download after a failed upload
	<-done
	if perr != nil {
		_ = s.media.st.Delete(context.WithoutCancel(ctx), key)
		if tooBig {
			return s.q.MarkMediaFailed(ctx, db.MarkMediaFailedParams{ID: id, LastError: ptr("file exceeds the size limit")})
		}
		return perr
	}
	// Meta documents sha256 as hex; compare when both sides have one.
	if want := strings.ToLower(f.Sha256); want != "" && len(want) == 64 && want != hex.EncodeToString(h.Sum(nil)) {
		_ = s.media.st.Delete(context.WithoutCancel(ctx), key)
		return s.q.MarkMediaFailed(ctx, db.MarkMediaFailedParams{ID: id, LastError: ptr("checksum mismatch")})
	}
	return s.q.MarkMediaStored(ctx, db.MarkMediaStoredParams{ID: id, StorageKey: &key, SizeBytes: &n})
}

// Outbound media limits (Meta's Cloud API media reference).
var uploadLimits = map[string]int64{
	"image/jpeg": 5 << 20, "image/png": 5 << 20, "image/webp": 500 << 10,
	"video/mp4": 16 << 20, "video/3gpp": 16 << 20,
	"audio/aac": 16 << 20, "audio/amr": 16 << 20, "audio/mpeg": 16 << 20, "audio/mp4": 16 << 20, "audio/ogg": 16 << 20,
	"text/plain": 100 << 20, "application/pdf": 100 << 20,
	"application/msword": 100 << 20, "application/vnd.openxmlformats-officedocument.wordprocessingml.document": 100 << 20,
	"application/vnd.ms-excel": 100 << 20, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": 100 << 20,
	"application/vnd.ms-powerpoint": 100 << 20, "application/vnd.openxmlformats-officedocument.presentationml.presentation": 100 << 20,
}

// MaxUpload is the largest file Meta accepts (documents).
const MaxUpload = 100 << 20

// UploadedMedia is a file stored on Meta, usable in messages for 30 days.
type UploadedMedia struct {
	ID        string
	MimeType  string
	Filename  string
	Size      int64
	ExpiresAt time.Time
}

// Upload sends a file to Meta through the given (or default) number and
// returns its media ID. The type and size are checked against Meta's limits
// before anything is sent.
func (s *Service) Upload(ctx context.Context, phoneNumberID *uuid.UUID, filename, mimeType string, size int64, body io.Reader) (UploadedMedia, error) {
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	limit, ok := uploadLimits[mimeType]
	if !ok {
		return UploadedMedia{}, apperr.Validation("content_type", "WhatsApp does not accept this file type.")
	}
	if size > limit {
		return UploadedMedia{}, apperr.Validation("file", fmt.Sprintf("This file type is limited to %d MB.", limit>>20))
	}
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" || filename == "." || len(filename) > 240 {
		return UploadedMedia{}, apperr.Validation("filename", "A file name is required.")
	}
	pn, err := s.resolveNumber(ctx, phoneNumberID)
	if err != nil {
		return UploadedMedia{}, err
	}
	sender, err := s.ch.SenderFor(ctx, pn.ID)
	if err != nil {
		return UploadedMedia{}, err
	}
	counted := &countingReader{r: io.LimitReader(body, limit+1)}
	id, err := s.ch.Client().UploadMedia(ctx, sender.PhoneNumber.PhoneNumberID, sender.Token, filename, mimeType, counted)
	if counted.n > limit {
		return UploadedMedia{}, apperr.Validation("file", fmt.Sprintf("This file type is limited to %d MB.", limit>>20))
	}
	if err != nil {
		var api *whatsapp.APIError
		if errors.As(err, &api) {
			return UploadedMedia{}, &apperr.Error{Status: 502, Code: "META_API_ERROR", Message: "Meta rejected the file: " + api.Error()}
		}
		return UploadedMedia{}, apperr.Unavailable("META_UNREACHABLE", "Meta's API could not be reached; try again.")
	}
	return UploadedMedia{ID: id, MimeType: mimeType, Filename: filename, Size: counted.n, ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UTC()}, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
