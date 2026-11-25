package channels

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// Periodic and on-demand tasks of the account signals.
const (
	// TypeSignalSync refreshes every active account's currency and its
	// numbers' throughput, quality rating and messaging limit (payload
	// {"account_id": …} limits it to one account).
	TypeSignalSync = "channels:signal_sync"
	// TypeCostReconcile stores Meta's daily cost (pricing_analytics) of the
	// last ReconcileDays days.
	TypeCostReconcile = "channels:cost_reconcile"
)

// ReconcileDays is how many days back the daily cost is re-read; Meta's
// figures for recent days still change.
const ReconcileDays = 3

// SyncSignals refreshes one account (id) or every active one (nil). A
// failing account does not stop the others.
func (s *Service) SyncSignals(ctx context.Context, id *uuid.UUID) error {
	ids, err := s.accountIDs(ctx, id)
	if err != nil {
		return err
	}
	var errs []error
	for _, a := range ids {
		if err := s.syncAccountSignals(ctx, a); err != nil {
			errs = append(errs, fmt.Errorf("account %s: %w", a, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) accountIDs(ctx context.Context, id *uuid.UUID) ([]uuid.UUID, error) {
	if id != nil {
		return []uuid.UUID{*id}, nil
	}
	accs, err := s.q.ListActiveWhatsAppAccounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(accs))
	for i, a := range accs {
		out[i] = a.ID
	}
	return out, nil
}

func (s *Service) syncAccountSignals(ctx context.Context, id uuid.UUID) error {
	acc, token, err := s.accountToken(ctx, id)
	if err != nil {
		return err
	}
	list, err := s.wa.ListPhoneNumbers(ctx, acc.WabaID, token)
	if err != nil {
		s.flag(ctx, acc.ID, err)
		return err
	}
	if err := s.tx(ctx, func(q *db.Queries) error { return upsertNumbers(ctx, q, acc.ID, list) }); err != nil {
		return err
	}
	return s.syncCurrency(ctx, acc, token)
}

// syncCurrency stores the account's billing currency; rates and Meta's
// cost figures are in this currency.
func (s *Service) syncCurrency(ctx context.Context, acc db.WhatsappAccount, token string) error {
	cur, err := s.wa.GetAccountCurrency(ctx, acc.WabaID, token)
	if err != nil {
		return err
	}
	if cur == "" || (acc.Currency != nil && *acc.Currency == cur) {
		return nil
	}
	return s.q.SetWhatsAppAccountCurrency(ctx, db.SetWhatsAppAccountCurrencyParams{ID: acc.ID, Currency: &cur})
}

// ReconcileCosts stores Meta's daily cost and volume per number, country,
// pricing category and type for the last ReconcileDays days of every
// active account. Accounts on a partner's credit line get volumes without
// cost.
func (s *Service) ReconcileCosts(ctx context.Context, now time.Time) error {
	ids, err := s.accountIDs(ctx, nil)
	if err != nil {
		return err
	}
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(ReconcileDays - 1))
	var errs []error
	for _, id := range ids {
		if err := s.reconcileAccount(ctx, id, start, now); err != nil {
			errs = append(errs, fmt.Errorf("account %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) reconcileAccount(ctx context.Context, id uuid.UUID, start, end time.Time) error {
	acc, token, err := s.accountToken(ctx, id)
	if err != nil {
		return err
	}
	if acc.Currency == nil {
		if err := s.syncCurrency(ctx, acc, token); err != nil {
			s.log.WarnContext(ctx, "account currency unknown", "account_id", id, "error", err)
		} else if acc, err = s.q.GetWhatsAppAccount(ctx, id); err != nil {
			return err
		}
	}
	points, err := s.wa.PricingAnalytics(ctx, acc.WabaID, token, whatsapp.PricingQuery{Start: start, End: end, Granularity: "DAILY",
		Dimensions: []string{"PHONE", "COUNTRY", "PRICING_CATEGORY", "PRICING_TYPE"}, MetricTypes: []string{"COST", "VOLUME"}})
	if err != nil {
		s.flag(ctx, acc.ID, err)
		return err
	}
	return s.tx(ctx, func(q *db.Queries) error {
		for _, p := range points {
			day := time.Unix(p.Start, 0).UTC()
			params := db.UpsertMetaCostDailyParams{AccountID: acc.ID,
				Day:         pgtype.Date{Time: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
				PhoneNumber: p.PhoneNumber, Country: p.Country, PricingCategory: p.PricingCategory, PricingType: p.PricingType,
				Volume: p.Volume, Currency: acc.Currency}
			if p.Cost != nil {
				c := strconv.FormatFloat(*p.Cost, 'f', 6, 64)
				params.Cost = &c
			}
			if err := q.UpsertMetaCostDaily(ctx, params); err != nil {
				return err
			}
		}
		return nil
	})
}
