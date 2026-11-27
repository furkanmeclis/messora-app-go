package campaigns

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Exports of a campaign: the recipients table (CSV or XLSX, honouring the
// table's filters) and the report (XLSX workbook or PDF summary). Small
// recipient exports stream straight to the browser; large ones and reports
// run as a background job in the bulk queue, whose file is kept on the
// data volume for ExportRetention and downloaded from the panel. Every
// request and download is audited.

// TypeExport generates a campaign export file.
const TypeExport = "campaign:export"

// Export limits.
const (
	// ExportSyncLimit is the largest recipient export streamed directly;
	// beyond it the panel asks for a background export.
	ExportSyncLimit = 20000
	// ExportRetention is how long a generated file stays downloadable.
	ExportRetention = 7 * 24 * time.Hour
)

// Export kinds and formats.
const (
	ExportRecipients = "recipients"
	ExportReport     = "report"
	FormatCSV        = "csv"
	FormatXLSX       = "xlsx"
	FormatPDF        = "pdf"
)

// ExportStore keeps generated export files in the private object store
// (t/<tenant>/exports/…); keys written before (exports/<id>.<format>
// under MEDIA_DIR) still open with the local driver.
type ExportStore struct {
	st      storage.Store
	scratch string
}

// NewExportStore returns an export store on the private bucket.
func NewExportStore(stores *storage.Stores) *ExportStore {
	return &ExportStore{st: stores.Private, scratch: stores.Scratch}
}

// SetExportStore enables exports.
func (s *Service) SetExportStore(st *ExportStore) { s.exports = st }

// tmpDir is a writable scratch directory (XLSX assembly).
func (e *ExportStore) tmpDir() string {
	d := filepath.Join(e.scratch, "exports")
	_ = os.MkdirAll(d, 0o750)
	return d
}

func (e *ExportStore) remove(ctx context.Context, key string) {
	if storage.CheckTenantKey(storage.TenantKey(ctx), key) == nil {
		_ = e.st.Delete(ctx, key)
	}
}

// ContentType returns the media type of an export format.
func ContentType(format string) string {
	switch format {
	case FormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatPDF:
		return "application/pdf"
	}
	return "text/csv; charset=utf-8"
}

// rowWriter writes table rows (CSV or a streamed worksheet).
type rowWriter interface {
	Write([]string) error
	Close() error
}

type csvWriter struct{ w *csv.Writer }

func (c csvWriter) Write(r []string) error { return c.w.Write(safeCells(r)) }
func (c csvWriter) Close() error           { c.w.Flush(); return c.w.Error() }

// safeCells neutralises spreadsheet formulas (CSV injection); phone
// numbers keep their leading +.
func safeCells(row []string) []string {
	out := make([]string, len(row))
	for i, v := range row {
		if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) && !e164Like(v) {
			v = "'" + v
		}
		out[i] = v
	}
	return out
}

func e164Like(v string) bool {
	if len(v) < 8 || v[0] != '+' {
		return false
	}
	for _, r := range v[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// sheetWriter streams rows into one worksheet of a workbook.
type sheetWriter struct {
	sw *excelize.StreamWriter
	r  int
}

func (x *sheetWriter) Write(row []string) error {
	x.r++
	cells := make([]any, len(row))
	for i, v := range row {
		cells[i] = v
	}
	cell, _ := excelize.CoordinatesToCellName(1, x.r)
	return x.sw.SetRow(cell, cells)
}

func (x *sheetWriter) Close() error { return x.sw.Flush() }

// RecipientColumns is the header of a recipients export.
var RecipientColumns = []string{"phone", "name", "contact_type", "state", "outcome", "message_status", "unreached_class",
	"failure_class", "error_code", "sent_at", "delivered_at", "read_at", "failed_at", "replied_at", "reaction", "reaction_source",
	"opted_out_at", "cost", "currency"}

func ts(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func recipientRecord(x RecipientRow) []string {
	return []string{x.PhoneE164, x.Name, x.ContactType, x.State, Outcome(x.State, x.MsgStatus), str(x.MsgStatus), x.UnreachedClass,
		str(x.FailureClass), str(x.ErrorCode), ts(x.SentAt), ts(x.DeliveredAt), ts(x.ReadAt), ts(x.FailedAt), ts(x.RepliedAt),
		str(x.Reaction), str(x.ReactionSource), ts(x.OptedOutAt), str(x.Cost), str(x.CostCurrency)}
}

func (s *Service) writeRecipients(ctx context.Context, id uuid.UUID, f RecipientFilter, out rowWriter) (int64, error) {
	if err := out.Write(RecipientColumns); err != nil {
		return 0, err
	}
	var n int64
	err := s.EachRecipient(ctx, id, f, func(x RecipientRow) error {
		n++
		return out.Write(recipientRecord(x))
	})
	return n, err
}

// StreamRecipients writes the matching recipients to w as CSV or XLSX,
// row by row (an XLSX is assembled in a temporary file and copied at the
// end). It refuses more than ExportSyncLimit rows (use a background export).
func (s *Service) StreamRecipients(ctx context.Context, id uuid.UUID, f RecipientFilter, format string, w io.Writer, before func()) error {
	if _, err := s.q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("Campaign not found.")
	}
	n, err := s.CountRecipients(ctx, id, f)
	if err != nil {
		return err
	}
	if n > ExportSyncLimit {
		return apperr.Conflict("EXPORT_TOO_LARGE", fmt.Sprintf("%d recipients match; exports above %d rows run in the background.", n, ExportSyncLimit))
	}
	if err := auditExport(ctx, s, id, ExportRecipients, format, f, "campaign.export"); err != nil {
		return err
	}
	before()
	_, err = s.writeRecipientsTo(ctx, id, f, format, w)
	return err
}

func (s *Service) writeRecipientsTo(ctx context.Context, id uuid.UUID, f RecipientFilter, format string, w io.Writer) (int64, error) {
	if format != FormatXLSX {
		cw := csvWriter{csv.NewWriter(w)}
		n, err := s.writeRecipients(ctx, id, f, cw)
		if cerr := cw.Close(); err == nil {
			err = cerr
		}
		return n, err
	}
	tmp := os.TempDir()
	if s.exports != nil {
		tmp = s.exports.tmpDir()
	}
	book := excelize.NewFile(excelize.Options{TmpDir: tmp})
	defer func() { _ = book.Close() }()
	_ = book.SetSheetName("Sheet1", "Alıcılar")
	sw, err := book.NewStreamWriter("Alıcılar")
	if err != nil {
		return 0, err
	}
	x := &sheetWriter{sw: sw}
	n, err := s.writeRecipients(ctx, id, f, x)
	if err != nil {
		return n, err
	}
	if err := x.Close(); err != nil {
		return n, err
	}
	_, err = book.WriteTo(w)
	return n, err
}

func auditExport(ctx context.Context, s *Service, id uuid.UUID, kind, format string, f RecipientFilter, action string) error {
	return audit.Record(ctx, s.q, audit.Entry{Action: action, EntityType: "campaign", EntityID: &id,
		Changes: map[string]any{"kind": kind, "format": format, "filters": f}})
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// RequestExport queues a background export.
func (s *Service) RequestExport(ctx context.Context, id uuid.UUID, kind, format string, f RecipientFilter, actor *uuid.UUID) (db.CampaignExport, error) {
	if s.exports == nil {
		return db.CampaignExport{}, apperr.Conflict("EXPORTS_UNAVAILABLE", "Exports are not configured.")
	}
	switch {
	case kind == ExportRecipients && (format == FormatCSV || format == FormatXLSX):
	case kind == ExportReport && (format == FormatXLSX || format == FormatPDF):
	default:
		return db.CampaignExport{}, apperr.Validation("format", "Recipients export as csv or xlsx, the report as xlsx or pdf.")
	}
	if _, err := s.q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return db.CampaignExport{}, apperr.NotFound("Campaign not found.")
	}
	if _, _, err := s.recipientWhere(ctx, id, f); err != nil {
		return db.CampaignExport{}, err
	}
	fj, _ := json.Marshal(f)
	var e db.CampaignExport
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		var err error
		e, err = q.InsertCampaignExport(ctx, db.InsertCampaignExportParams{CampaignID: id, Kind: kind, Format: format, Filters: fj, RequestedBy: actor})
		if err != nil {
			return err
		}
		if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeExport), map[string]string{"export_id": e.ID.String()}, 0); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "campaign.export_requested", EntityType: "campaign", EntityID: &id,
			Changes: map[string]any{"export_id": e.ID, "kind": kind, "format": format, "filters": f}})
	})
	return e, err
}

// RunExport generates a queued export's file (the bulk queue's job).
func (s *Service) RunExport(ctx context.Context, exportID uuid.UUID) error {
	e, err := s.q.ClaimCampaignExport(ctx, exportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // done, failed or running elsewhere
	}
	if err != nil {
		return err
	}
	fail := func(cause error) error {
		msg := cause.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		_ = s.q.FailCampaignExport(context.WithoutCancel(ctx), db.FailCampaignExportParams{ID: e.ID, Error: &msg})
		return nil
	}
	if s.exports == nil {
		return fail(errors.New("exports are not configured"))
	}
	var f RecipientFilter
	_ = json.Unmarshal(e.Filters, &f)
	key, err := storage.Key(storage.TenantKey(ctx), "exports", e.Format)
	if err != nil {
		return fail(err)
	}
	// Streamed into the store as it is generated; a failure abandons the
	// upload, so no partial file is ever downloadable.
	var rows int64
	size, err := storage.PutFunc(ctx, s.exports.st, key, ContentType(e.Format), storage.PutOptions{}, func(file io.Writer) error {
		var err error
		switch e.Kind {
		case ExportRecipients:
			rows, err = s.writeRecipientsTo(ctx, e.CampaignID, f, e.Format, file)
		case ExportReport:
			if e.Format == FormatPDF {
				err = s.WriteReportPDF(ctx, e.CampaignID, file)
			} else {
				rows, err = s.WriteReportXLSX(ctx, e.CampaignID, file)
			}
		}
		return err
	})
	if err != nil {
		if ctx.Err() != nil {
			return err // shutting down: the job is retried
		}
		return fail(err)
	}
	c, _ := s.q.GetCampaign(ctx, e.CampaignID)
	name := exportFilename(c.Name, e.Kind, e.Format)
	exp := time.Now().Add(ExportRetention)
	return s.q.FinishCampaignExport(ctx, db.FinishCampaignExportParams{ID: e.ID, FileKey: &key, Filename: &name, SizeBytes: &size,
		RowsWritten: &rows, ExpiresAt: &exp})
}

// exportFilename is a safe download name.
func exportFilename(campaign, kind, format string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(campaign) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		base = "campaign"
	}
	if len(base) > 60 {
		base = base[:60]
	}
	suffix := "alicilar"
	if kind == ExportReport {
		suffix = "rapor"
	}
	return fmt.Sprintf("%s-%s.%s", base, suffix, format)
}

// Exports lists a campaign's recent exports.
func (s *Service) Exports(ctx context.Context, id uuid.UUID) ([]db.CampaignExport, error) {
	if _, err := s.q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("Campaign not found.")
	}
	return s.q.ListCampaignExports(ctx, id)
}

// OpenExport checks a finished export can be downloaded, records the
// download and returns its object key (in ExportObjects()).
func (s *Service) OpenExport(ctx context.Context, campaignID, exportID uuid.UUID) (string, db.CampaignExport, error) {
	e, err := s.q.GetCampaignExport(ctx, exportID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && e.CampaignID != campaignID) {
		return "", e, apperr.NotFound("Export not found.")
	}
	if err != nil {
		return "", e, err
	}
	if e.Status != "done" || e.FileKey == nil || s.exports == nil || (e.ExpiresAt != nil && e.ExpiresAt.Before(time.Now())) {
		return "", e, apperr.Conflict("EXPORT_NOT_READY", "The export is not ready or has expired.")
	}
	if err := storage.CheckTenantKey(storage.TenantKey(ctx), *e.FileKey); err != nil {
		return "", e, apperr.Conflict("EXPORT_NOT_READY", "The export file is gone.")
	}
	if _, err := s.exports.st.Stat(ctx, *e.FileKey); errors.Is(err, storage.ErrNotFound) {
		return "", e, apperr.Conflict("EXPORT_NOT_READY", "The export file is gone.")
	} else if err != nil {
		return "", e, err
	}
	if err := audit.Record(ctx, s.q, audit.Entry{Action: "campaign.export_download", EntityType: "campaign", EntityID: &campaignID,
		Changes: map[string]any{"export_id": exportID, "kind": e.Kind, "format": e.Format}}); err != nil {
		return "", e, err
	}
	return *e.FileKey, e, nil
}

// ExportObjects is the store export files are kept in.
func (s *Service) ExportObjects() storage.Store {
	if s.exports == nil {
		return nil
	}
	return s.exports.st
}

// expireExports deletes expired files and re-queues exports whose job was
// lost.
func (s *Service) expireExports(ctx context.Context) error {
	gone, err := s.q.ExpiredCampaignExports(ctx)
	if err != nil {
		return err
	}
	for _, g := range gone {
		if s.exports != nil && g.FileKey != nil {
			s.exports.remove(ctx, *g.FileKey)
		}
	}
	stale, err := s.q.StaleCampaignExports(ctx)
	if err != nil {
		return err
	}
	for _, id := range stale {
		msg := "The export job did not finish; request it again."
		if err := s.q.FailCampaignExport(ctx, db.FailCampaignExportParams{ID: id, Error: &msg}); err != nil {
			return err
		}
	}
	return nil
}
