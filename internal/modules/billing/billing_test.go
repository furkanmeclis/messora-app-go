package billing

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

func TestKurusParseAndFormat(t *testing.T) {
	for in, want := range map[string]int64{"1290": 129000, "1290.5": 129050, "1290.50": 129050, "0.01": 1, "-12.30": -1230, "3588,00": 358800} {
		got, err := ParseKurus(in)
		if err != nil || got != want {
			t.Errorf("ParseKurus(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "1.234", "abc", "1e3x", "1.2.3"} {
		if _, err := ParseKurus(bad); err == nil {
			t.Errorf("ParseKurus(%q) accepted", bad)
		}
	}
	if FormatKurus(358800) != "3588.00" || FormatKurus(-5) != "-0.05" || FormatKurus(0) != "0.00" {
		t.Fatal("FormatKurus")
	}
	if FormatMicros(1_842_600_000) != "1842.600000" || FormatMicros(-660) != "-0.000660" {
		t.Fatal("FormatMicros")
	}
	if tl(358800) != "3.588,00 TL" || tl(123456789) != "1.234.567,89 TL" || tl(5) != "0,05 TL" {
		t.Fatalf("tl: %s %s", tl(358800), tl(123456789))
	}
}

// VAT is computed once on the net total and rounded half up to a kuruş:
// 0.5 kuruş becomes 1 (never banker's rounding).
func TestVATRoundsHalfUp(t *testing.T) {
	cases := []struct {
		net, vat int64
		bp       int64
	}{
		{299000, 59800, 2000},  // 2,990 TL → 598 TL
		{129000, 25800, 2000},  // 1,290 TL
		{1, 0, 2000},           // 0.2 kuruş → 0
		{3, 1, 2000},           // 0.6 kuruş → 1
		{25, 5, 2000},          // exactly 5
		{2, 0, 2000},           // 0.4 → 0
		{125, 25, 2000},        // 25
		{12, 2, 2000},          // 2.4 → 2
		{13, 3, 2000},          // 2.6 → 3
		{105, 11, 1000},        // 10.5 → 11 (half up; banker's would give 10)
		{125, 13, 1000},        // 12.5 → 13 (banker's would give 12)
		{59900, 5990, 1000},    // 10 %
		{100, 1, 50},           // 0.5 % of 1 TL = 0.5 kuruş → 1
		{299000 * 10, 598000, 2000},
	}
	for _, c := range cases {
		if got := VAT(c.net, c.bp); got != c.vat {
			t.Errorf("VAT(%d, %d) = %d, want %d", c.net, c.bp, got, c.vat)
		}
	}
	tot := WithVAT(299000, 2000)
	if tot.Total != 358800 || tot.Net+tot.VAT != tot.Total {
		t.Fatalf("totals: %+v", tot)
	}
	if FormatRateBP(2000) != "20" || FormatRateBP(1850) != "18.5" || FormatRateBP(1825) != "18.25" {
		t.Fatal("FormatRateBP")
	}
	if bp, err := ParseRateBP("20"); err != nil || bp != 2000 {
		t.Fatal("ParseRateBP")
	}
}

// Upgrades are prorated by whole days: the price difference × days left ÷
// the period's days, half up; today counts as a full day.
func TestProrate(t *testing.T) {
	start := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	end := AddCycle(start, CycleMonthly) // 31 days
	if !end.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("end %v", end)
	}
	diff := int64(350000) // growth → pro: 6,490 − 2,990 TL
	cases := []struct {
		now  time.Time
		want int64
	}{
		{start, diff},                                    // the whole period
		{start.Add(-time.Hour), diff},                    // before the period: the whole period
		{start.AddDate(0, 0, 15), 180645},                // 16 days left: 350000 × 16 / 31 = 180645.16
		{start.AddDate(0, 0, 15).Add(time.Hour), 180645}, // part of a day counts as the day
		{end.Add(-time.Minute), 11290},                   // the last day: 350000 / 31 = 11290.32
		{end, 0},                                         // over
	}
	for _, c := range cases {
		if got := Prorate(diff, c.now, start, end); got != c.want {
			t.Errorf("Prorate at %v = %d, want %d", c.now, got, c.want)
		}
	}
	if Prorate(-5, start, start, end) != 0 {
		t.Fatal("a cheaper plan is never prorated")
	}
	// Halves round up: 3 × 1 / 2 = 1.5 → 2.
	if Prorate(3, start.AddDate(0, 0, 1), start, start.AddDate(0, 0, 2)) != 2 {
		t.Fatal("half up")
	}
	// Yearly: 365 days.
	ys := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	ye := AddCycle(ys, CycleYearly)
	if got := Prorate(365, ys.AddDate(0, 0, 100), ys, ye); got != 265 {
		t.Fatalf("yearly: %d", got)
	}
}

func TestAddCycleKeepsMonthEnds(t *testing.T) {
	jan31 := time.Date(2027, 1, 31, 10, 0, 0, 0, time.UTC)
	if got := AddCycle(jan31, CycleMonthly); !got.Equal(time.Date(2027, 2, 28, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("31 Jan + 1 month = %v", got)
	}
	leap := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	if got := AddCycle(leap, CycleYearly); !got.Equal(time.Date(2029, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("29 Feb + 1 year = %v", got)
	}
	if got := addMonths(time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC), 3); !got.Equal(time.Date(2027, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("comp 3 months = %v", got)
	}
}

// Message charges: marketing at Meta × 1.20, other categories Meta +
// $0.0006, converted at the day's rate and rounded half up to a micro-lira.
func TestMessageChargeAndFX(t *testing.T) {
	fx, _ := ParseRate("42.18")
	mk, _ := ParseRate("1.20")
	other, _ := ParseRate("0.0006")
	// Marketing $0.0109 → 0.0109 × 1.2 × 42.18 = 0.5517144 TL.
	if got := MessageCharge(10_900, "marketing", fx, mk, other); got != 551_714 {
		t.Fatalf("marketing: %d", got)
	}
	// Utility $0.0009 → (0.0009 + 0.0006) × 42.18 = 0.06327 TL.
	if got := MessageCharge(900, "utility", fx, mk, other); got != 63_270 {
		t.Fatalf("utility: %d", got)
	}
	if got := MessageCharge(900, "authentication", fx, mk, other); got != 63_270 {
		t.Fatalf("authentication: %d", got)
	}
	if MessageCharge(0, "marketing", fx, mk, other) != 0 {
		t.Fatal("free messages cost nothing")
	}
	// Half up at the micro-lira: 1 micro-USD × 0.5 = 0.5 → 1.
	half, _ := ParseRate("0.5")
	one, _ := ParseRate("1")
	if got := MessageCharge(1, "marketing", half, one, other); got != 1 {
		t.Fatalf("half up: %d", got)
	}
	if _, err := ParseRate("-1"); err == nil {
		t.Fatal("negative rate accepted")
	}
	if r, _ := ParseRate("42,1800"); r.String() != "42.18" {
		t.Fatalf("rate string %q", r.String())
	}
	if MicrosToKurus(1_842_605_000) != 184_261 || MicrosToKurus(4_999) != 0 || MicrosToKurus(5_000) != 1 {
		t.Fatal("MicrosToKurus")
	}
}

func TestReferenceChecksum(t *testing.T) {
	ref, err := Reference("4F8K", 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(ref) != 12 || !strings.HasPrefix(ref, "MSR4F8K") || !valid(ref) {
		t.Fatalf("reference %q", ref)
	}
	disp := DisplayReference(ref)
	if disp != ref[:3]+"-"+ref[3:7]+"-"+ref[7:11]+"-"+ref[11:] {
		t.Fatalf("display %q", disp)
	}
	// Typed with dashes, spaces, lower case and look-alikes.
	if got, ok := ParseReference(strings.ToLower(disp)); !ok || got != ref {
		t.Fatalf("parse %q: %q %v", disp, got, ok)
	}
	if got, ok := ParseReference(strings.ReplaceAll(ref[3:], "0", "O")); !ok || got != ref {
		t.Fatalf("without prefix and with O for 0: %q %v", got, ok)
	}
	// Every single wrong character is caught.
	for i := 3; i < 12; i++ {
		for _, c := range []byte(crockford) {
			if c == ref[i] {
				continue
			}
			bad := ref[:i] + string(c) + ref[i+1:]
			if valid(bad) {
				t.Fatalf("single error at %d not caught: %s", i, bad)
			}
		}
	}
	// Swaps of neighbours: count what is caught (Luhn mod 32 misses only
	// a handful of pairs).
	missed, total := 0, 0
	for seq := int64(1); seq <= 2000; seq++ {
		r, _ := Reference("ZX9Q", seq)
		for i := 3; i < 11; i++ {
			if r[i] == r[i+1] {
				continue
			}
			total++
			b := []byte(r)
			b[i], b[i+1] = b[i+1], b[i]
			if valid(string(b)) {
				missed++
			}
		}
	}
	if float64(missed)/float64(total) > 0.02 {
		t.Fatalf("transpositions missed: %d of %d", missed, total)
	}
	if _, err := Reference("4F8K", 0); err == nil {
		t.Fatal("counter 0 accepted")
	}
	if _, err := Reference("4F8K", MaxProformaSeq+1); err == nil {
		t.Fatal("counter overflow accepted")
	}
	if _, err := Reference("4FIK", 1); err == nil {
		t.Fatal("I is not a Crockford digit")
	}
}

func TestReferencesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, code := range []string{"AAAA", "AAAB", "ZZZZ"} {
		for seq := int64(1); seq <= 3000; seq++ {
			r, err := Reference(code, seq)
			if err != nil || seen[r] {
				t.Fatalf("%s %d: %q %v", code, seq, r, err)
			}
			seen[r] = true
		}
	}
	for range 200 {
		c, err := NewCode()
		if err != nil || len(c) != 4 || strings.Trim(c, crockford) != "" {
			t.Fatalf("code %q %v", c, err)
		}
	}
}

func TestFindReferencesInDescriptions(t *testing.T) {
	a, _ := Reference("4F8K", 7)
	b, _ := Reference("9TQ1", 12)
	text := "EFT / " + strings.ToLower(DisplayReference(a)) + " aralik abonelik, ayrica " + b + " ve tekrar " + a
	got := FindReferences(text)
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("found %v", got)
	}
	bad := a[:11] + string(crockford[(strings.IndexByte(crockford, a[11])+1)%32])
	if len(FindReferences("ref "+bad)) != 0 {
		t.Fatal("a reference with a wrong check character was found")
	}
	if len(FindReferences("no reference here MSR")) != 0 {
		t.Fatal("found something in nothing")
	}
}

func TestIBANAndTaxNumbers(t *testing.T) {
	// TR33 0006 1005 1978 6457 8413 26 is the textbook Turkish example.
	if !ValidIBAN("TR33 0006 1005 1978 6457 8413 26") || ValidIBAN("TR33 0006 1005 1978 6457 8413 27") || ValidIBAN("TR12") {
		t.Fatal("IBAN check")
	}
	if FormatIBAN("tr330006100519786457841326") != "TR33 0006 1005 1978 6457 8413 26" {
		t.Fatal("FormatIBAN")
	}
	if !validTCKN("10000000146") || validTCKN("10000000147") || validTCKN("00000000146") || validTCKN("1000000014") {
		t.Fatal("TCKN check")
	}
}

func TestCSVAmounts(t *testing.T) {
	for in, want := range map[string]int64{"1234.56": 123456, "1.234,56": 123456, "1,234.56": 123456, "1234,56": 123456,
		"3.588,00 TL": 358800, "1.234": 123400, "590": 59000, "0.50": 50, "-100,00": -10000} {
		got, err := parseCSVAmount(in)
		if err != nil || got != want {
			t.Errorf("parseCSVAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if d, err := parseCSVDate("28.11.2026"); err != nil || d.Format(time.DateOnly) != "2026-11-28" {
		t.Fatal("DD.MM.YYYY")
	}
	if _, err := parseCSVDate("11/28/2026"); err == nil {
		t.Fatal("US dates are refused")
	}
}

func TestPricingValidation(t *testing.T) {
	p, err := ParsePricing(DefaultPricing())
	if err != nil {
		t.Fatal(err)
	}
	if p.VATRateBP != 2000 || p.MinTopUp != 100000 || len(p.Packs) != 3 || p.Packs[0].Price != 59000 {
		t.Fatalf("defaults: %+v", p)
	}
	bad := DefaultPricing()
	bad.MessageTopups = append(bad.MessageTopups, "500.00")
	if _, err := ParsePricing(bad); err == nil {
		t.Fatal("a top-up below the minimum was accepted")
	}
	bad = DefaultPricing()
	bad.RenewalLeadDays = 3
	if _, err := ParsePricing(bad); err == nil {
		t.Fatal("a lead time shorter than the first reminder was accepted")
	}
}

func TestPlanPriceYearlyIsTenMonths(t *testing.T) {
	var p db.Plan
	_ = p.PriceMonthly.Scan("2990")
	if m, ok := PlanPrice(p, CycleMonthly, nil); !ok || m != 299000 {
		t.Fatalf("monthly %d", m)
	}
	if y, ok := PlanPrice(p, CycleYearly, nil); !ok || y != 2990000 {
		t.Fatalf("yearly without a list price: %d", y)
	}
	_ = p.PriceYearly.Scan("29000")
	if y, _ := PlanPrice(p, CycleYearly, nil); y != 2900000 {
		t.Fatalf("yearly list price: %d", y)
	}
	custom := int64(1)
	if c, _ := PlanPrice(p, CycleYearly, &custom); c != 1 {
		t.Fatal("custom price")
	}
	if monthlyValue(2990000, CycleYearly) != 249167 {
		t.Fatalf("monthly value of a year: %d", monthlyValue(2990000, CycleYearly))
	}
}

func TestProformaPDF(t *testing.T) {
	f := db.Proforma{Number: "PF-2026-000001", Reference: "MSR4F8K00017", Status: "open", NetKurus: 299000, VatRateBp: 2000, VatKurus: 59800,
		TotalKurus: 358800, CreatedAt: time.Now(), DueAt: time.Now().AddDate(0, 0, 7),
		Lines:  []byte(`[{"description_tr":"Messora Büyüme planı · aylık","description_en":"Messora Growth plan · monthly","quantity":1,"unit_kurus":299000,"net_kurus":299000}]`),
		Buyer:  []byte(`{"name":"Acme Gıda Ltd. Şti.","tax_office":"Kadıköy","tax_number":"1234567890","address":"Moda Cd. 1, İstanbul","email":"muhasebe@acme.test"}`),
		Seller: []byte(`{"name":"Messora Teknoloji A.Ş.","tax_office":"Beşiktaş","tax_number":"9876543210","address":"İstanbul","email":"fatura@messora.tech","bank_accounts":[{"bank_name":"Örnek Bank","account_holder":"Messora Teknoloji A.Ş.","iban":"TR330006100519786457841326"}]}`)}
	var buf bytes.Buffer
	if err := WriteProformaPDF(&buf, f); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) || buf.Len() < 5000 {
		t.Fatalf("pdf of %d bytes", buf.Len())
	}
	// PROFORMA_PDF_OUT=<file> keeps the sample for a visual check.
	if out := os.Getenv("PROFORMA_PDF_OUT"); out != "" {
		if err := os.WriteFile(out, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
