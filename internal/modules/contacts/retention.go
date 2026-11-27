package contacts

import (
	"context"
	"errors"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
)

// TypeImportRetention is the periodic task that removes old imports
// (ExpireImports), once per tenant.
const TypeImportRetention = "contacts:import_retention"

// ImportRetention is how long an import's uploaded file and error report
// (and the import itself) are kept after it finished.
const ImportRetention = 30 * 24 * time.Hour

// importRetentionBatch bounds one run; the next run continues.
const importRetentionBatch = 500

// ExpireImports deletes the files and rows of the context tenant's imports
// that finished (or were uploaded and never started) more than retention
// ago. An import whose objects cannot be deleted keeps its row, so the next
// run tries again.
func (s *Service) ExpireImports(ctx context.Context, retention time.Duration) (int, error) {
	rows, err := s.q.ExpiredContactImports(ctx, db.ExpiredContactImportsParams{Before: time.Now().Add(-retention), MaxRows: importRetentionBatch})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		keys := []string{r.StorageKey}
		if r.ErrorReportKey != nil {
			keys = append(keys, *r.ErrorReportKey)
		}
		if err := s.imp.removeAll(ctx, keys); err != nil {
			s.log.WarnContext(ctx, "could not delete an old import's files", "import_id", r.ID, "err", err)
			continue
		}
		if err := s.q.DeleteContactImport(ctx, r.ID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// removeAll deletes keys of the context's tenant. Missing objects count as
// deleted; a key of another tenant is never deleted.
func (s *ImportStore) removeAll(ctx context.Context, keys []string) error {
	for _, k := range keys {
		if k == "" || storage.CheckTenantKey(storage.TenantKey(ctx), k) != nil {
			continue
		}
		if err := s.st.Delete(ctx, k); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
	}
	return nil
}
