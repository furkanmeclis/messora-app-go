package contacts

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// xlsxToCSV converts the first worksheet of an .xlsx file into CSV,
// streaming rows so large workbooks do not have to fit in memory. Formulas
// are not evaluated: the cached value Excel saved is used.
func xlsxToCSV(src string, dst io.Writer, tmpDir string) error {
	f, err := excelize.OpenFile(src, excelize.Options{
		TmpDir: tmpDir, // the container's root filesystem is read-only
		// Guard against zip bombs: the sheet XML may not unpack beyond this.
		UnzipSizeLimit:    512 << 20,
		UnzipXMLSizeLimit: 64 << 20,
	})
	if err != nil {
		return apperr.Validation("file", "The file is not a readable Excel workbook (.xlsx).")
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return apperr.Validation("file", "The workbook has no worksheets.")
	}
	rows, err := f.Rows(sheets[0])
	if err != nil {
		return apperr.Validation("file", "The first worksheet could not be read.")
	}
	defer func() { _ = rows.Close() }()
	w := csv.NewWriter(dst)
	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			return apperr.Validation("file", "The first worksheet could not be read.")
		}
		if len(cols) > maxColumns {
			return apperr.Validation("file", fmt.Sprintf("The file has more than %d columns.", maxColumns))
		}
		if err := w.Write(cols); err != nil {
			return err
		}
	}
	if err := rows.Error(); err != nil {
		return apperr.Validation("file", "The first worksheet could not be read.")
	}
	w.Flush()
	return w.Error()
}

// convertXLSX replaces the uploaded workbook at key with its CSV form.
func (s *ImportStore) convertXLSX(key string) error {
	src, err := s.path(key)
	if err != nil {
		return err
	}
	tmp := src + ".csv.tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640) //nolint:gosec // derived from a generated key
	if err != nil {
		return err
	}
	cerr := xlsxToCSV(src, out, s.tmpDir())
	if err := out.Close(); cerr == nil {
		cerr = err
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	return os.Rename(tmp, src)
}

// xlsxWriter streams an export as a workbook.
type xlsxWriter struct {
	f  *excelize.File
	sw *excelize.StreamWriter
	r  int
}

func newXLSXWriter(tmpDir string) (*xlsxWriter, error) {
	f := excelize.NewFile(excelize.Options{TmpDir: tmpDir})
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, err
	}
	return &xlsxWriter{f: f, sw: sw}, nil
}

func (x *xlsxWriter) Write(row []string) error {
	x.r++
	cells := make([]any, len(row))
	for i, v := range row {
		cells[i] = v // always text: phone numbers must not become numbers
	}
	cell, _ := excelize.CoordinatesToCellName(1, x.r)
	return x.sw.SetRow(cell, cells)
}

// Flush is a no-op: rows are buffered by the stream writer until Finish.
func (x *xlsxWriter) Flush() error { return nil }

func (x *xlsxWriter) Finish(w io.Writer) error {
	if err := x.sw.Flush(); err != nil {
		return err
	}
	_, err := x.f.WriteTo(w)
	_ = x.f.Close()
	return err
}

// tmpDir is a writable scratch directory on the data volume.
func (s *ImportStore) tmpDir() string {
	d := filepath.Join(s.root, "imports", "tmp")
	_ = os.MkdirAll(d, 0o750)
	return d
}
