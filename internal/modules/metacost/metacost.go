package metacost

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/phonecountry"
)

// Cost sources stored in messages.cost_source.
const (
	SourceRateCard = "rate_card" // priced from the imported rate card
	SourceFree     = "free"      // Meta reported a free pricing type
)

// Pricing is the pricing object of a status webhook.
type Pricing struct {
	Type     string // regular | free_customer_service | free_entry_point | …
	Category string
	Model    string // PMP
	Billable *bool  // deprecated by Meta, kept while it is sent
}

// chargeable reports whether Meta charges for the message.
func (p Pricing) chargeable() bool { return p.Type == "regular" }

// PriceMessage stores the pricing Meta reported for an outbound message
// and its cost: 0 for a free pricing type, the rate card's price in the
// account's currency for a regular one (no cost when no rate matches).
// recipient is the recipient's E.164 number.
func PriceMessage(ctx context.Context, q *db.Queries, m db.Message, p Pricing, recipient string) error {
	if p.Type == "" && p.Category == "" {
		return nil
	}
	country := phonecountry.Of(recipient)
	params := db.SetMessagePricingParams{ID: m.ID, PricingType: nonEmpty(p.Type), PricingCategory: nonEmpty(p.Category),
		PricingModel: nonEmpty(p.Model), Billable: p.Billable, RecipientCountry: nonEmpty(country)}
	switch {
	case p.Type != "" && !p.chargeable():
		zero, src := Micros(0).String(), SourceFree
		params.CostAmount, params.CostSource = &zero, &src
		if cur, err := accountCurrency(ctx, q, m.PhoneNumberID); err != nil {
			return err
		} else if cur != "" {
			params.CostCurrency = &cur
		}
	case p.chargeable() && p.Category != "":
		at := time.Now()
		if m.SentAt != nil {
			at = *m.SentAt
		}
		rate, ok, err := lookup(ctx, q, m, country, p.Category, at)
		if err != nil {
			return err
		}
		if ok {
			amount, src := rate.Price.String(), SourceRateCard
			params.CostAmount, params.CostCurrency, params.CostSource = &amount, &rate.Currency, &src
		}
	}
	return q.SetMessagePricing(ctx, params)
}

func accountCurrency(ctx context.Context, q *db.Queries, phoneNumberID uuid.UUID) (string, error) {
	pn, err := q.GetPhoneNumber(ctx, phoneNumberID)
	if err != nil {
		return "", err
	}
	acc, err := q.GetWhatsAppAccount(ctx, pn.AccountID)
	if err != nil {
		return "", err
	}
	if acc.Currency == nil {
		return "", nil
	}
	return *acc.Currency, nil
}

// lookup finds the rate for a message: the account's currency, the
// recipient's market and the month's volume so far (rate tiers).
func lookup(ctx context.Context, q *db.Queries, m db.Message, country, category string, at time.Time) (Rate, bool, error) {
	category = NormalizeCategory(category)
	rows, err := q.MetaRateCandidates(ctx, db.MetaRateCandidatesParams{Category: category, Country: country})
	if err != nil || len(rows) == 0 {
		return Rate{}, false, err
	}
	pn, err := q.GetPhoneNumber(ctx, m.PhoneNumberID)
	if err != nil {
		return Rate{}, false, err
	}
	acc, err := q.GetWhatsAppAccount(ctx, pn.AccountID)
	if err != nil {
		return Rate{}, false, err
	}
	card := Card{Markets: map[string]string{}}
	if acc.Currency != nil {
		card.Currency = *acc.Currency
	}
	market, err := q.GetMetaRateMarket(ctx, country)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		market = OtherMarket
	case err != nil:
		return Rate{}, false, err
	default:
		card.Markets[country] = market
	}
	for _, r := range rows {
		price, err := ParseMicros(r.Price)
		if err != nil {
			return Rate{}, false, err
		}
		card.Rates = append(card.Rates, Rate{Currency: r.Currency, Market: r.Market, Category: r.Category, Price: price,
			TierLower: r.TierLower, TierUpper: r.TierUpper, EffectiveFrom: fromDate(r.EffectiveFrom), EffectiveTo: optDate(r.EffectiveTo)})
	}
	// Tiers count the month's earlier billable messages to the same market.
	countries := []string{country}
	if market != OtherMarket {
		if countries, err = q.ListMarketCountries(ctx, market); err != nil {
			return Rate{}, false, err
		}
	}
	at = at.UTC()
	month := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	volume, err := q.CountBillableInMonth(ctx, db.CountBillableInMonthParams{AccountID: pn.AccountID, MessageID: m.ID,
		Category: &category, Countries: countries, MonthStart: &month, At: &at})
	if err != nil {
		return Rate{}, false, err
	}
	r, ok := card.RateFor(country, category, at, volume)
	return r, ok, nil
}

// Import stores a parsed rate card: countries are (re)assigned to their
// markets, rates are added or replace the same currency/market/category/
// tier/date, and every rate's end date is recomputed from the next one.
func Import(ctx context.Context, q *db.Queries, rates []Rate, markets map[string]string, actor *uuid.UUID) error {
	for country, market := range markets {
		if err := q.UpsertMetaRateMarket(ctx, db.UpsertMetaRateMarketParams{Country: country, Market: market}); err != nil {
			return err
		}
	}
	for _, r := range rates {
		if err := q.UpsertMetaRate(ctx, db.UpsertMetaRateParams{Currency: r.Currency, Market: r.Market, Category: r.Category,
			Price: r.Price.String(), TierLower: r.TierLower, TierUpper: r.TierUpper,
			EffectiveFrom: pgtype.Date{Time: r.EffectiveFrom, Valid: true}, Source: r.Source, ImportedBy: actor}); err != nil {
			return err
		}
	}
	return q.CloseSupersededMetaRates(ctx)
}

func fromDate(d pgtype.Date) time.Time {
	return time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, time.UTC)
}

func optDate(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := fromDate(d)
	return &t
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
