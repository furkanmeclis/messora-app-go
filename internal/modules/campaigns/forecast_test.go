package campaigns

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
)

func TestPlanDurationWithoutLimit(t *testing.T) {
	sec, windows := planDuration(1000, 1000, 10, limitState{})
	if sec == nil || *sec != 100 || windows != 1 {
		t.Fatalf("got %v %d", sec, windows)
	}
	if sec, _ := planDuration(1000, 1000, 0, limitState{}); sec != nil {
		t.Fatal("no capacity: no duration")
	}
	if sec, _ := planDuration(0, 0, 0, limitState{}); sec == nil || *sec != 0 {
		t.Fatal("empty audience takes no time")
	}
}

func TestPlanDurationFitsTheLimit(t *testing.T) {
	// 250-tier, 100 used: 150 left; 120 count → one window.
	sec, windows := planDuration(200, 120, 20, limitState{Limit: 250, Used: 100})
	if windows != 1 || sec == nil || *sec != 10 {
		t.Fatalf("got %v %d", sec, windows)
	}
}

func TestPlanDurationNeedsSeveralWindows(t *testing.T) {
	// 2K tier, 1,500 used + 100 in flight: 400 left now. 5,000 count:
	// 400 today, then 2,000, 2,000 and 600 → 4 windows; the last batch of
	// 600 starts three days later.
	lim := limitState{Limit: 2000, Used: 1500, InFlight: 100}
	sec, windows := planDuration(6000, 5000, 50, lim)
	if windows != 4 {
		t.Fatalf("windows %d", windows)
	}
	want := int64(3*24*3600 + 600/50)
	if sec == nil || *sec != want {
		t.Fatalf("duration %v want %d", sec, want)
	}
	// Customers that do not count (open window) never need extra windows.
	if _, w := planDuration(6000, 300, 50, lim); w != 1 {
		t.Fatalf("windows %d", w)
	}
}

func TestHistoryRatesInsufficient(t *testing.T) {
	r := historyRates([]db.ForecastHistoryRow{{ID: uuid.New(), Recipients: 900, Sent: 880, Delivered: 800},
		{ID: uuid.New(), Recipients: 300, Sent: 290, Delivered: 280}})
	if r.Status != RatesInsufficientHistory || r.Campaigns != 2 || r.Recipients != 1200 || r.DeliveryRate != nil {
		t.Fatalf("got %+v", r)
	}
	if r.CampaignsNeeded != 3 || r.RecipientsNeeded != 500 {
		t.Fatalf("thresholds %+v", r)
	}
	// Enough campaigns but too few recipients.
	r = historyRates([]db.ForecastHistoryRow{{ID: uuid.New(), Recipients: 100}, {ID: uuid.New(), Recipients: 100}, {ID: uuid.New(), Recipients: 100}})
	if r.Status != RatesInsufficientHistory {
		t.Fatalf("300 recipients must not be enough: %+v", r)
	}
	if e := expected(1000, r); e.Sent != nil {
		t.Fatal("no expected counts without history")
	}
}

func TestHistoryRatesPooled(t *testing.T) {
	rows := []db.ForecastHistoryRow{
		{ID: uuid.New(), Recipients: 300, OBlocked: 0, Sent: 300, Delivered: 270, Read: 135, Replied: 27, Positive: 9, OptedOut: 3},
		{ID: uuid.New(), Recipients: 200, OBlocked: 0, Sent: 200, Delivered: 180, Read: 90, Replied: 18, Positive: 6, OptedOut: 2},
		{ID: uuid.New(), Recipients: 100, OBlocked: 0, Sent: 100, Delivered: 50, Read: 25, Replied: 5, Positive: 0, OptedOut: 0},
		{ID: uuid.New(), Recipients: 0}, // empty campaigns do not count
	}
	r := historyRates(rows)
	if r.Status != RatesOK || r.Campaigns != 3 || r.Recipients != 600 {
		t.Fatalf("got %+v", r)
	}
	// Pooled: delivered 500 / sent 600.
	if *r.DeliveryRate != 0.8333 || *r.ReadRate != 0.5 || *r.ReplyRate != 0.1 {
		t.Fatalf("rates %+v", r)
	}
	e := expected(1200, r)
	if *e.Sent != 1200 || *e.Delivered != 1000 || *e.Read != 500 || *e.Replied != 100 || *e.Positive != 30 {
		t.Fatalf("expected %+v / %d %d %d %d %d", e, *e.Sent, *e.Delivered, *e.Read, *e.Replied, *e.Positive)
	}
}

func rateCard() metacost.Card {
	d := func(s string) time.Time { t, _ := time.Parse(time.DateOnly, s); return t }
	return metacost.Card{Currency: "USD", Markets: map[string]string{"TR": "Turkey", "DE": "Germany"}, Rates: []metacost.Rate{
		{Currency: "USD", Market: "Turkey", Category: "marketing", Price: 10900, EffectiveFrom: d("2026-07-01")},
		{Currency: "USD", Market: "Turkey", Category: "utility", Price: 5300, EffectiveFrom: d("2026-07-01")},
		{Currency: "USD", Market: "Germany", Category: "marketing", Price: 131000, EffectiveFrom: d("2026-10-01")},
	}}
}

func TestEstimateCostMarketingAndMissingRates(t *testing.T) {
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	groups := []costGroup{{Country: "TR", N: 100}, {Country: "TR", WindowOpen: true, N: 10}, {Country: "DE", N: 5}, {Country: "BR", N: 7}}
	c := estimateCost(rateCard(), groups, "marketing", at)
	// Marketing is charged inside the window too; Brazil has no rate.
	if c.Priced != 115 || c.Free != 0 || c.NotInRateCard != 7 {
		t.Fatalf("got %+v", c)
	}
	// 110 × 0.0109 + 5 × 0.131 = 1.199 + 0.655
	if c.Amount != "1.854000" || c.Currency != "USD" {
		t.Fatalf("amount %s %s", c.Amount, c.Currency)
	}
	if c.ByCountry[0].Country != "TR" || *c.ByCountry[0].Amount != "1.199000" {
		t.Fatalf("by country %+v", c.ByCountry[0])
	}
	for _, bc := range c.ByCountry {
		if bc.Country == "BR" && (bc.Price != nil || bc.Amount != nil || bc.Charged != 7) {
			t.Fatalf("missing rate must not be guessed: %+v", bc)
		}
	}
	// Germany's rate only starts on 2026-10-01: in September it is missing.
	sep := estimateCost(rateCard(), []costGroup{{Country: "DE", N: 5}}, "marketing", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if sep.NotInRateCard != 5 || sep.Priced != 0 {
		t.Fatalf("september %+v", sep)
	}
}

func TestEstimateCostUtilityInsideWindow2026Rules(t *testing.T) {
	groups := []costGroup{{Country: "TR", N: 20}, {Country: "TR", WindowOpen: true, N: 30}}
	// Before 2026-10-01 utility inside an open window was free.
	before := estimateCost(rateCard(), groups, "utility", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	if before.Free != 30 || before.Priced != 20 || before.Amount != "0.106000" {
		t.Fatalf("before %+v", before)
	}
	// Since 2026-10-01 it is charged again.
	after := estimateCost(rateCard(), groups, "utility", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if after.Free != 0 || after.Priced != 50 || after.Amount != "0.265000" {
		t.Fatalf("after %+v", after)
	}
}
