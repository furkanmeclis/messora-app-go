package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
)

// The message balance (docs/BILLING.md "Message balance"). Companies that
// send through Messora's Tech Provider connection send through Messora's
// Meta account, so Messora pays Meta and passes the fee on: every message
// Meta charges (pricing type "regular" in its status webhook) is debited
// once at Meta's USD rate card price × the platform's FX rate, plus the
// margin (marketing × marketing_markup, other categories + other_markup_usd
// per message). Free messages (the customer service window, free entry
// points) cost nothing; a failed message has no pricing and is never
// charged. Companies on their own Meta app are billed by Meta directly:
// nothing is debited and nothing is checked.

// TypeNotify is the task that sends a billing notice after the
// transaction that triggered it (the low message balance e-mail).
const TypeNotify = "billing:notify"

// NotifyPayload is TypeNotify's payload.
type NotifyPayload struct {
	Kind string `json:"kind"`
}

// NoticeMessageLow is the TypeNotify kind of the low message balance e-mail.
const NoticeMessageLow = "message_low"

// TypeTick is the per-tenant billing job.
const TypeTick = "billing:tick"

// usesTechProvider reports whether the message's number belongs to an
// account connected through Messora's Tech Provider.
func usesTechProvider(ctx context.Context, q *db.Queries, phoneNumberID uuid.UUID) (bool, error) {
	m, err := q.AccountConnectionMethod(ctx, phoneNumberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return m == "tech_provider", err
}

// ChargeMessage debits the message balance for a message Meta priced, in
// the transaction that stores the pricing (the messaging webhook): once
// per message, race-safe (the charge row is unique per message, the
// wallet row is updated in place).
func (s *Service) ChargeMessage(ctx context.Context, tx pgx.Tx, q *db.Queries, m db.Message, p metacost.Pricing, recipient string) error {
	if p.Type != "regular" || p.Category == "" {
		return nil
	}
	tp, err := usesTechProvider(ctx, q, m.PhoneNumberID)
	if err != nil || !tp {
		return err
	}
	category := metacost.NormalizeCategory(p.Category)
	usd, ok, err := metacost.PriceIn(ctx, q, m, category, recipient, "USD")
	if err != nil {
		return err
	}
	if !ok {
		s.o.Logger.WarnContext(ctx, "message balance: no USD rate on the rate card; not charged", "message_id", m.ID, "category", category)
		return nil
	}
	_, pricing, err := s.settings(ctx)
	if err != nil {
		return err
	}
	amount := MessageCharge(int64(usd), category, pricing.FX, pricing.MarketingMarkup, pricing.OtherMarkupUSD)
	if amount <= 0 {
		return nil
	}
	if err := q.EnsureMessageWallet(ctx); err != nil {
		return err
	}
	usdMicros := int64(usd)
	entry, err := q.InsertMessageCharge(ctx, db.InsertMessageChargeParams{AmountMicros: -amount, MessageID: &m.ID, Category: &category,
		MetaUsdMicros: &usdMicros, FxRate: pricing.FX.String()})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // charged before (a repeated webhook)
	}
	if err != nil {
		return err
	}
	w, err := q.AddMessageWalletBalance(ctx, -amount)
	if err != nil {
		return err
	}
	if err := q.SetWalletEntryBalance(ctx, db.SetWalletEntryBalanceParams{ID: entry, BalanceAfter: w.BalanceMicros}); err != nil {
		return err
	}
	if w.BalanceMicros < pricing.LowThreshold*MicrosPerKurus && w.LowNotifiedAt == nil {
		n, err := q.MarkWalletLowNotified(ctx)
		if err != nil {
			return err
		}
		if n == 1 {
			_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeNotify), NotifyPayload{Kind: NoticeMessageLow}, 0)
			return err
		}
	}
	return nil
}

// topUp adds micros to the wallet (a paid top-up or an admin adjustment);
// a balance back above the alert threshold re-arms the low balance e-mail.
func (s *Service) topUp(ctx context.Context, q *db.Queries, micros int64, proforma *uuid.UUID, note string, by *uuid.UUID) error {
	if micros == 0 {
		return nil
	}
	if err := q.EnsureMessageWallet(ctx); err != nil {
		return err
	}
	w, err := q.AddMessageWalletBalance(ctx, micros)
	if err != nil {
		return err
	}
	kind := "top_up"
	if proforma == nil {
		kind = "adjustment"
	}
	if _, err := q.InsertMessageWalletEntry(ctx, db.InsertMessageWalletEntryParams{Kind: kind, AmountMicros: micros, BalanceAfter: w.BalanceMicros,
		ProformaID: proforma, Note: note, CreatedBy: by}); err != nil {
		return err
	}
	_, pricing, err := s.settings(ctx)
	if err != nil {
		return err
	}
	if w.BalanceMicros >= pricing.LowThreshold*MicrosPerKurus && w.LowNotifiedAt != nil {
		return q.ClearWalletLowNotified(ctx)
	}
	return nil
}

// balanceMicros is the context tenant's wallet balance (0 without one).
func balanceMicros(ctx context.Context, q *db.Queries) (int64, error) {
	w, err := q.GetMessageWallet(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return w.BalanceMicros, err
}

// CheckMessageBalance answers 402 MESSAGE_BALANCE_EXHAUSTED when a template
// would be sent through a Tech Provider number while the balance is used
// up (q: the caller's transaction or the pool's queries). Own-app numbers
// are never checked.
func (s *Service) CheckMessageBalance(ctx context.Context, q *db.Queries, phoneNumberID uuid.UUID) error {
	if q == nil {
		q = s.q
	}
	tp, err := usesTechProvider(ctx, q, phoneNumberID)
	if err != nil || !tp {
		return err
	}
	b, err := balanceMicros(ctx, q)
	if err != nil {
		return err
	}
	if b <= 0 {
		return ErrBalanceExhausted()
	}
	return nil
}

// MessageBalance is the context tenant's message balance for the panel.
func (s *Service) MessageBalance(ctx context.Context) (api.MessageBalance, error) {
	return s.messageBalance(ctx, s.q)
}

func (s *Service) messageBalance(ctx context.Context, q *db.Queries) (api.MessageBalance, error) {
	_, pricing, err := s.settings(ctx)
	if err != nil {
		return api.MessageBalance{}, err
	}
	used, err := q.UsesTechProvider(ctx)
	if err != nil {
		return api.MessageBalance{}, err
	}
	b, err := balanceMicros(ctx, q)
	if err != nil {
		return api.MessageBalance{}, err
	}
	low := pricing.LowThreshold * MicrosPerKurus
	return api.MessageBalance{Used: used, Balance: FormatKurus(MicrosToKurus(b)), BalanceExact: FormatMicros(b), Low: b < low,
		Exhausted: b <= 0, LowThreshold: FormatKurus(pricing.LowThreshold), FxUsdTry: pricing.FX.String(),
		MarketingMarkup: pricing.MarketingMarkup.String(), OtherMarkupUsd: pricing.OtherMarkupUSD.String()}, nil
}

// MessageBalanceExhausted reports whether a Tech Provider workspace's
// balance is used up (the panel's banner).
func (s *Service) MessageBalanceExhausted(ctx context.Context) bool {
	used, err := s.q.UsesTechProvider(ctx)
	if err != nil || !used {
		return false
	}
	b, err := balanceMicros(ctx, s.q)
	return err == nil && b <= 0
}

func entryToAPI(e db.MessageWalletEntry) api.MessageBalanceEntry {
	out := api.MessageBalanceEntry{Id: e.ID, Kind: api.MessageBalanceEntryKind(e.Kind), Amount: FormatMicros(e.AmountMicros),
		BalanceAfter: FormatMicros(e.BalanceAfter), Category: e.Category, Note: e.Note, CreatedAt: e.CreatedAt}
	if e.MetaUsdMicros != nil {
		v := metacost.Micros(*e.MetaUsdMicros).String()
		out.MetaUsd = &v
	}
	if e.FxRate.Valid {
		if v, err := e.FxRate.Value(); err == nil && v != nil {
			s := fmt.Sprint(v)
			out.FxRate = &s
		}
	}
	return out
}

// MessageBalanceDetail is the balance with its latest entries.
func (s *Service) MessageBalanceDetail(ctx context.Context, limit int32) (api.MessageBalanceDetail, error) {
	b, err := s.MessageBalance(ctx)
	if err != nil {
		return api.MessageBalanceDetail{}, err
	}
	rows, err := s.q.ListMessageWalletEntries(ctx, limit)
	if err != nil {
		return api.MessageBalanceDetail{}, err
	}
	out := api.MessageBalanceDetail{Balance: b, Entries: make([]api.MessageBalanceEntry, len(rows))}
	for i, r := range rows {
		out.Entries[i] = entryToAPI(r)
	}
	return out, nil
}

// HandleNotify sends a TypeNotify notice for the context tenant.
func (s *Service) HandleNotify(ctx context.Context, payload []byte) error {
	var p NotifyPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.Kind == NoticeMessageLow {
		s.mailMessageLow(ctx)
	}
	return nil
}
