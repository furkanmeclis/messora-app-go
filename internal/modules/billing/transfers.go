package billing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// The payments inbox (docs/BILLING.md "Matching transfers"). A platform
// admin records each incoming transfer (or imports the bank statement);
// matching suggests the proforma:
//
//   - reference_amount: the description carries a valid reference and the
//     amount is what the proforma still owes;
//   - reference: the reference matches but the amount differs (a partial
//     payment, or an overpayment kept as account credit);
//   - amount: no reference, and exactly one open proforma of all companies
//     owes this amount; several candidates are listed for a manual choice.
//
// Nothing is applied until the admin confirms (Match).

// Transfer is a recorded transfer.
type Transfer struct {
	Row        db.BankTransfer
	TenantSlug *string
	TenantName *string
}

// TransferInput is a transfer to record.
type TransferInput struct {
	AmountKurus int64
	ReceivedOn  time.Time
	SenderName  string
	SenderIBAN  string
	Description string
	Reference   string
	Note        string
	Source      string
	Fingerprint *string
}

// suggest finds the proforma a transfer most likely pays.
func suggest(ctx context.Context, q *db.Queries, amount int64, ref *string) (*uuid.UUID, *string, error) {
	kind := func(s string) *string { return &s }
	if ref != nil {
		f, err := q.GetProformaByReference(ctx, *ref)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		if err != nil {
			return nil, nil, err
		}
		if (f.Status == "open" || f.Status == "partially_paid") && f.TotalKurus-f.PaidKurus == amount {
			return &f.ID, kind("reference_amount"), nil
		}
		return &f.ID, kind("reference"), nil
	}
	rows, err := q.OpenProformasByAmount(ctx, amount)
	if err != nil || len(rows) != 1 {
		return nil, nil, err
	}
	return &rows[0].ID, kind("amount"), nil
}

func (s *Service) recordTransfer(ctx context.Context, q *db.Queries, admin *principal.Principal, in TransferInput) (db.BankTransfer, bool, error) {
	if in.AmountKurus <= 0 {
		return db.BankTransfer{}, false, apperr.Validation("amount", "Only incoming transfers (an amount above 0) are recorded.")
	}
	var ref *string
	if strings.TrimSpace(in.Reference) != "" {
		r, ok := ParseReference(in.Reference)
		if !ok {
			return db.BankTransfer{}, false, apperr.Validation("reference", "This is not a valid payment reference (check the characters).")
		}
		ref = &r
	} else if found := FindReferences(in.Description); len(found) > 0 {
		ref = &found[0]
	}
	var iban *string
	if v := NormalizeIBAN(in.SenderIBAN); v != "" {
		if !ibanShape.MatchString(v) || len(v) > 34 {
			return db.BankTransfer{}, false, apperr.Validation("sender_iban", "This is not an IBAN.")
		}
		iban = &v
	}
	sp, kind, err := suggest(ctx, q, in.AmountKurus, ref)
	if err != nil {
		return db.BankTransfer{}, false, err
	}
	src := in.Source
	if src == "" {
		src = "manual"
	}
	row, err := q.InsertBankTransfer(ctx, db.InsertBankTransferParams{AmountKurus: in.AmountKurus, ReceivedOn: dateOnly(in.ReceivedOn),
		SenderName: truncate(strings.TrimSpace(in.SenderName), 200), SenderIban: iban, Description: truncate(strings.TrimSpace(in.Description), 1000),
		Reference: ref, Source: src, Fingerprint: in.Fingerprint, SuggestedProformaID: sp, Suggestion: kind, Note: truncate(strings.TrimSpace(in.Note), 500),
		CreatedBy: actorID(admin)})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.BankTransfer{}, false, nil // a statement line imported before
	}
	if err != nil {
		return db.BankTransfer{}, false, err
	}
	err = audit.Record(ctx, q, audit.Entry{Action: "platform.billing_transfer_record", EntityType: "bank_transfer", EntityID: &row.ID,
		Changes: map[string]any{"amount": FormatKurus(row.AmountKurus), "received_on": in.ReceivedOn.Format(time.DateOnly), "reference": ref,
			"source": src, "suggestion": kind}})
	return row, true, err
}

func dateOnly(t time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// RecordTransfer records an incoming transfer with its suggestion.
func (s *Service) RecordTransfer(ctx context.Context, admin *principal.Principal, in TransferInput) (db.BankTransfer, error) {
	var out db.BankTransfer
	err := s.platformTx(ctx, func(ctx context.Context, q *db.Queries, _ *effects) error {
		var err error
		out, _, err = s.recordTransfer(ctx, q, admin, in)
		return err
	})
	return out, err
}

// Match confirms a transfer: for a proforma (the amount it needs is
// applied, the rest kept as the company's account credit) or, without
// one, all of it as a company's account credit. The account credit then
// pays the company's other open proformas, oldest due first.
func (s *Service) Match(ctx context.Context, admin *principal.Principal, transferID uuid.UUID, proformaID, tenantID *uuid.UUID, note string) (db.BankTransfer, error) {
	var out db.BankTransfer
	err := s.platformTx(ctx, func(ctx context.Context, q *db.Queries, fx *effects) error {
		t, err := q.GetBankTransferForUpdate(ctx, transferID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Transfer not found.")
		}
		if err != nil {
			return err
		}
		if t.Status != "unmatched" {
			return apperr.Conflict("TRANSFER_NOT_UNMATCHED", "This transfer is matched or ignored already.")
		}
		var f *db.Proforma
		var company uuid.UUID
		switch {
		case proformaID != nil:
			p, err := q.GetPlatformProforma(ctx, *proformaID)
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("proforma_id", "Unknown proforma.")
			}
			if err != nil {
				return err
			}
			f, company = &p, p.TenantID
		case tenantID != nil:
			if _, err := q.GetTenant(ctx, *tenantID); errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("tenant_id", "Unknown company.")
			} else if err != nil {
				return err
			}
			company = *tenantID
		default:
			return apperr.Validation("proforma_id", "Choose the proforma (or the company) the transfer belongs to.")
		}
		if err := q.SetTenantLocal(ctx, company.String()); err != nil {
			return err
		}
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		method := "manual"
		if f != nil && t.SuggestedProformaID != nil && *t.SuggestedProformaID == f.ID && t.Suggestion != nil {
			method = *t.Suggestion
		}
		by := actorID(admin)
		out, err = q.MatchBankTransfer(ctx, db.MatchBankTransferParams{ID: t.ID, TenantID: &company, ProformaID: proformaID, MatchMethod: &method,
			MatchedBy: by, Note: truncate(strings.TrimSpace(note), 500)})
		if err != nil {
			return err
		}
		left := t.AmountKurus
		received := t.ReceivedOn.Time
		if f != nil {
			// Lock the proforma, then apply what it still owes.
			cur, err := q.GetProformaForUpdate(ctx, f.ID)
			if err != nil {
				return err
			}
			if left, err = s.pay(ctx, q, &sub, cur, left, "transfer", &t.ID, received, by, fx); err != nil {
				return err
			}
		}
		if left > 0 {
			reason, noteText := "unallocated", "Transfer of "+t.ReceivedOn.Time.Format(time.DateOnly)
			if f != nil {
				reason, noteText = "overpayment", "Overpayment of "+f.Number
			}
			if err := s.addCredit(ctx, q, &sub, left, reason, &t.ID, proformaID, noteText, by); err != nil {
				return err
			}
			if err := s.applyCredit(ctx, q, &sub, fx); err != nil {
				return err
			}
		}
		ch := map[string]any{"transfer": t.ID, "amount": FormatKurus(t.AmountKurus), "method": method, "kept_as_credit": FormatKurus(left)}
		if f != nil {
			ch["proforma"] = f.Number
		}
		if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.payment_received", EntityType: "bank_transfer", EntityID: &t.ID, Changes: ch}); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.billing_transfer_match", EntityType: "bank_transfer", EntityID: &t.ID, Tenant: &company, Changes: ch})
	})
	return out, err
}

// Ignore marks a transfer as not a customer payment.
func (s *Service) Ignore(ctx context.Context, admin *principal.Principal, id uuid.UUID, reason string) (db.BankTransfer, error) {
	var out db.BankTransfer
	err := s.platformTx(ctx, func(ctx context.Context, q *db.Queries, _ *effects) error {
		var err error
		out, err = q.IgnoreBankTransfer(ctx, db.IgnoreBankTransferParams{ID: id, Note: truncate(strings.TrimSpace(reason), 500), MatchedBy: actorID(admin)})
		if errors.Is(err, pgx.ErrNoRows) {
			if _, gerr := q.GetBankTransfer(ctx, id); errors.Is(gerr, pgx.ErrNoRows) {
				return apperr.NotFound("Transfer not found.")
			}
			return apperr.Conflict("TRANSFER_NOT_UNMATCHED", "This transfer is matched or ignored already.")
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.billing_transfer_ignore", EntityType: "bank_transfer", EntityID: &id,
			Changes: map[string]any{"reason": reason}})
	})
	return out, err
}

// ── Bank statement CSV ─────────────────────────────────────────────────────

// LineError is a statement line that could not be imported.
type LineError struct {
	Line    int
	Message string
}

// ImportResult is what an import did.
type ImportResult struct {
	Created    []db.BankTransfer
	Duplicates int
	Errors     []LineError
}

// csv column names (lower-cased, Turkish names accepted).
var csvColumns = map[string]string{
	"date": "date", "tarih": "date", "islem tarihi": "date", "işlem tarihi": "date", "value date": "date",
	"amount": "amount", "tutar": "amount", "alacak": "amount", "credit": "amount",
	"description": "description", "aciklama": "description", "açıklama": "description",
	"sender": "sender", "gonderen": "sender", "gönderen": "sender", "sender_name": "sender",
	"sender_iban": "sender_iban", "iban": "sender_iban", "gonderen iban": "sender_iban", "gönderen iban": "sender_iban",
}

// parseCSVAmount reads 1234.56, 1,234.56, 1.234,56 or 1234,56.
func parseCSVAmount(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ReplaceAll(s, "TL", "")), "TRY"))
	s = strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), " ", "")
	dot, comma := strings.LastIndexByte(s, '.'), strings.LastIndexByte(s, ',')
	switch {
	case dot >= 0 && comma >= 0 && comma > dot: // 1.234,56
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	case dot >= 0 && comma >= 0: // 1,234.56
		s = strings.ReplaceAll(s, ",", "")
	case comma >= 0 && len(s)-comma-1 <= 2: // 1234,56
		s = strings.Replace(s, ",", ".", 1)
	case comma >= 0: // 1,234
		s = strings.ReplaceAll(s, ",", "")
	case dot >= 0 && len(s)-dot-1 == 3 && strings.Count(s, ".") >= 1 && !strings.HasPrefix(s, "0."): // 1.234
		s = strings.ReplaceAll(s, ".", "")
	}
	return ParseKurus(s)
}

func parseCSVDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.DateOnly, "02.01.2006", "02/01/2006", "2006-01-02 15:04:05", "02.01.2006 15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("date %q: use YYYY-MM-DD or DD.MM.YYYY", s)
}

// ImportCSV records the incoming lines of a bank statement (docs/BILLING.md
// "Statement CSV"). Each line is imported once: its fingerprint covers
// date, amount, sender, description and how many identical lines came
// before it in the file.
func (s *Service) ImportCSV(ctx context.Context, admin *principal.Principal, body io.Reader) (ImportResult, error) {
	raw, err := io.ReadAll(io.LimitReader(body, 2<<20))
	if err != nil {
		return ImportResult{}, err
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(raw) {
		return ImportResult{}, apperr.Validation("", "The file must be UTF-8 encoded.")
	}
	first, _, _ := strings.Cut(string(raw), "\n")
	r := csv.NewReader(bytes.NewReader(raw))
	if strings.Count(first, ";") > strings.Count(first, ",") {
		r.Comma = ';'
	}
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return ImportResult{}, apperr.Validation("", "The file has no header row.")
	}
	cols := map[string]int{}
	for i, h := range header {
		if c, ok := csvColumns[strings.ToLower(strings.TrimSpace(h))]; ok {
			cols[c] = i
		}
	}
	for _, need := range []string{"date", "amount", "description"} {
		if _, ok := cols[need]; !ok {
			return ImportResult{}, apperr.Validation("", "The header needs the columns date, amount and description (and optionally sender, sender_iban).")
		}
	}
	var res ImportResult
	seen := map[string]int{}
	err = s.platformTx(ctx, func(ctx context.Context, q *db.Queries, _ *effects) error {
		line := 1
		for {
			rec, err := r.Read()
			if errors.Is(err, io.EOF) {
				return nil
			}
			line++
			if err != nil {
				res.Errors = append(res.Errors, LineError{Line: line, Message: err.Error()})
				continue
			}
			get := func(c string) string {
				if i, ok := cols[c]; ok && i < len(rec) {
					return strings.TrimSpace(rec[i])
				}
				return ""
			}
			if strings.Join(rec, "") == "" {
				continue
			}
			day, err := parseCSVDate(get("date"))
			if err != nil {
				res.Errors = append(res.Errors, LineError{Line: line, Message: err.Error()})
				continue
			}
			amount, err := parseCSVAmount(get("amount"))
			if err != nil {
				res.Errors = append(res.Errors, LineError{Line: line, Message: "amount: " + err.Error()})
				continue
			}
			if amount <= 0 {
				continue // outgoing or empty lines are not payments
			}
			key := strings.Join([]string{day.Format(time.DateOnly), FormatKurus(amount), strings.ToLower(get("sender")), normalize(get("description"))}, "|")
			seen[key]++
			sum := sha256.Sum256(fmt.Appendf(nil, "%s#%d", key, seen[key]))
			fp := hex.EncodeToString(sum[:])
			row, created, err := s.recordTransfer(ctx, q, admin, TransferInput{AmountKurus: amount, ReceivedOn: day, SenderName: get("sender"),
				SenderIBAN: get("sender_iban"), Description: get("description"), Source: "csv", Fingerprint: &fp})
			if e, ok := apperr.As(err); ok && e.Status == 422 {
				res.Errors = append(res.Errors, LineError{Line: line, Message: e.Message})
				continue
			}
			if err != nil {
				return err
			}
			if !created {
				res.Duplicates++
				continue
			}
			res.Created = append(res.Created, row)
		}
	})
	return res, err
}

// Transfers lists the inbox.
func (s *Service) Transfers(ctx context.Context, status *string) ([]db.ListBankTransfersRow, error) {
	return s.q.ListBankTransfers(tenant.Platform(ctx), db.ListBankTransfersParams{Status: status, Lim: 500})
}
