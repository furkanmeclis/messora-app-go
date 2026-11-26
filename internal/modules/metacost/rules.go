package metacost

import (
	"context"
	"strings"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

// Charging rules of Meta's per-message pricing, for estimates made before a
// message is sent (campaign forecasts). Actual costs come from the status
// webhook's pricing object (PriceMessage); these rules predict it.
//
// Sources: docs/PHASE-16C.md §1.1 and Meta's documentation (pricing;
// "Upcoming pricing updates for … service and utility messages"; the
// messages status webhook reference, pricing.type):
//   - Marketing, utility and authentication templates are charged per
//     message ("regular"). The only free case for them is the free entry
//     point window (click-to-WhatsApp ads, pricing.type free_entry_point),
//     which a campaign cannot know beforehand and is not assumed.
//   - Utility templates sent inside an open customer service window were
//     free (free_customer_service) from 2025-07-01 until 2026-09-30. "As of
//     October 1, 2026, Meta will charge on a per-message basis for utility
//     messages sent in response to users (within an open 24-hour customer
//     service window)": from 2026-10-01 they are charged again.
//   - Service (non-template) messages were free; "effective October 1,
//     2026 Meta will charge on a per-message basis for all service
//     messages", after the business phone number's free tier of monthly
//     service messages (pricing.sub_type free_tier; 1,000 per number per
//     month, docs/PHASE-16C.md §1.1). Service has no volume tiers; its rate
//     equals the market's utility/authentication rate (Meta publishes it as
//     its own rate card row: category "service").
//   - Rates change at most quarterly; the rate card valid on the send day
//     applies (Card.RateFor).

// UtilityInWindowChargedFrom is the day utility templates inside an open
// customer service window became charged again.
var UtilityInWindowChargedFrom = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// ServiceChargedFrom is the day service messages became charged.
var ServiceChargedFrom = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// ServiceFreeTier is the monthly number of free service messages per
// business phone number from ServiceChargedFrom on.
const ServiceFreeTier = 1000

// Charged reports whether Meta charges a message of category sent at at:
// windowOpen is whether the customer's service window with the number is
// open then, serviceSent how many service messages the number already sent
// in that calendar month (only used for service).
func Charged(category string, at time.Time, windowOpen bool, serviceSent int64) bool {
	switch NormalizeCategory(category) {
	case "marketing", "authentication", "authentication-international":
		return true
	case "utility":
		return !windowOpen || !at.Before(UtilityInWindowChargedFrom)
	case "service":
		return !at.Before(ServiceChargedFrom) && serviceSent >= ServiceFreeTier
	}
	return true // unknown categories: assume charged, never free by guess
}

// TemplateCategory maps a template's category (MARKETING, UTILITY,
// AUTHENTICATION) to the rate card's.
func TemplateCategory(c string) string { return NormalizeCategory(strings.ToLower(c)) }

// LoadCard reads the whole imported rate card (rates and the country →
// market mapping) for estimates; currency restricts it to the account's
// currency ("" = any).
func LoadCard(ctx context.Context, q *db.Queries, currency string) (Card, error) {
	rows, err := q.ListMetaRates(ctx)
	if err != nil {
		return Card{}, err
	}
	card := Card{Markets: map[string]string{}, Currency: currency}
	for _, r := range rows {
		price, err := ParseMicros(r.Price)
		if err != nil {
			return card, err
		}
		card.Rates = append(card.Rates, Rate{Currency: r.Currency, Market: r.Market, Category: r.Category, Price: price,
			TierLower: r.TierLower, TierUpper: r.TierUpper, EffectiveFrom: fromDate(r.EffectiveFrom), EffectiveTo: optDate(r.EffectiveTo),
			Source: r.Source})
		for _, c := range strings.Fields(r.Countries) {
			card.Markets[c] = r.Market
		}
	}
	return card, nil
}
