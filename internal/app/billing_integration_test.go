//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/billing"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Phase 17 WP9: subscriptions and billing (docs/BILLING.md). The tests
// share harnesses through subtests and move billing's clock instead of
// waiting.

const testIBAN = "TR330006100519786457841326"

// clock is billing's fake clock.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

// useBillingClock gives billing a clock the test moves.
func (h *harness) useBillingClock(start time.Time) *clock {
	c := &clock{now: start}
	h.svc.billing.SetClock(c.Now)
	return c
}

// setupSeller enters Messora's seller identity and bank account.
func (h *harness) setupSeller() {
	h.t.Helper()
	r := h.adminDo(call{method: "PUT", path: "/billing/settings", body: map[string]any{"seller": map[string]any{
		"company_name": "Messora Teknoloji A.Ş.", "tax_office": "Beşiktaş", "tax_number": "9876543210", "address": "Levent, İstanbul",
		"email": "fatura@messora.test", "phone": "", "website": "https://messora.tech", "invoice_note": "",
		"bank_accounts": []any{map[string]any{"bank_name": "Örnek Bank", "account_holder": "Messora Teknoloji A.Ş.", "iban": "TR33 0006 1005 1978 6457 8413 26"}}}}})
	h.expect(r, 200, "")
}

// billingProfile enters the workspace's invoice details.
func (h *harness) billingProfile(token string) {
	h.t.Helper()
	h.expect(h.do(call{method: "PUT", path: "/api/v1/billing/profile", token: token, body: map[string]any{"kind": "company",
		"title": "Acme Gıda Ltd. Şti. (" + h.host + ")", "tax_office": "Kadıköy", "tax_number": "1234567890", "address": "Moda Cd. 1, Kadıköy",
		"city": "İstanbul", "email": "muhasebe@" + h.host}}), 200, "")
}

// billingTick runs the billing job of the harness's tenant.
func (h *harness) billingTick() billing.TickResult {
	h.t.Helper()
	res, err := h.svc.billing.Tick(h.ctx())
	if err != nil {
		h.t.Fatal(err)
	}
	return res
}

// setSubscription sets the tenant's status, trial and plan the way the
// registry stores them.
func (h *harness) setSubscription(status string, trialEnds *time.Time, plan string) {
	h.t.Helper()
	if _, err := h.owner.Exec(context.Background(), `UPDATE tenants SET subscription_status = $2, trial_ends_at = $3, plan_key = $4 WHERE id = $1`,
		h.tenantID, status, trialEnds, plan); err != nil {
		h.t.Fatal(err)
	}
	h.svc.planDB.InvalidateAll()
	h.svc.billing.InvalidateGate()
}

func (h *harness) subStatus() string {
	h.t.Helper()
	var s string
	if err := h.owner.QueryRow(context.Background(), `SELECT subscription_status FROM tenants WHERE id = $1`, h.tenantID).Scan(&s); err != nil {
		h.t.Fatal(err)
	}
	return s
}

// mailsTo counts captured e-mails whose subject contains part.
func (h *harness) mailsWith(part string) int {
	h.mail.mu.Lock()
	defer h.mail.mu.Unlock()
	n := 0
	for _, m := range h.mail.sent {
		if strings.Contains(m.Subject, part) {
			n++
		}
	}
	return n
}

// transfer records an incoming transfer in the payments inbox.
func (h *harness) transfer(amount, description string) map[string]any {
	h.t.Helper()
	r := h.adminDo(call{method: "POST", path: "/billing/transfers", body: map[string]any{"amount": amount,
		"received_on": time.Now().Format(time.DateOnly), "sender_name": "ACME GIDA", "description": description}})
	h.expect(r, 201, "")
	return r.data()
}

func (h *harness) match(transferID, proformaID string) map[string]any {
	h.t.Helper()
	r := h.adminDo(call{method: "POST", path: "/billing/transfers/" + transferID + "/match", body: map[string]any{"proforma_id": proformaID}})
	h.expect(r, 200, "")
	return r.data()
}

func (h *harness) proforma(token, id string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/billing/proformas/" + id, token: token})
	h.expect(r, 200, "")
	return r.data()
}

func (h *harness) billingOverview(token string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/billing", token: token})
	h.expect(r, 200, "")
	return r.data()
}

func sub(o map[string]any) map[string]any { return o["subscription"].(map[string]any) }

func TestBillingSubscriptions(t *testing.T) {
	h := newHarness(t)
	h.setupSeller()
	start := time.Now().UTC().Truncate(time.Second)
	clk := h.useBillingClock(start)

	hb := h.tenant("billco")
	ownerB := hb.setupOwnerAs("owner@billco.test")
	trialEnd := start.Add(24 * time.Hour)
	hb.setSubscription("trial", &trialEnd, "growth")

	var trialProforma map[string]any
	t.Run("invoice details are required before the first proforma", func(t *testing.T) {
		r := hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "growth", "billing_cycle": "monthly"}})
		hb.expect(r, 409, billing.CodeProfileRequired)
		// A company needs a valid VKN and tax office, a person a valid TCKN.
		hb.expect(hb.do(call{method: "PUT", path: "/api/v1/billing/profile", token: ownerB, body: map[string]any{"kind": "individual",
			"title": "Ayşe Yılmaz", "tax_number": "12345678901", "address": "Moda Cd. 1", "email": "ayse@billco.test"}}), 422, "VALIDATION_ERROR")
		hb.expect(hb.do(call{method: "PUT", path: "/api/v1/billing/profile", token: ownerB, body: map[string]any{"kind": "company",
			"title": "Acme", "tax_number": "1234567890", "address": "Moda Cd. 1", "email": "ayse@billco.test"}}), 422, "VALIDATION_ERROR")
		hb.billingProfile(ownerB)
		o := hb.billingOverview(ownerB)
		if o["profile"] == nil || sub(o)["status"] != "trial" || sub(o)["read_only"] != false || len(sub(o)["code"].(string)) != 4 {
			t.Fatalf("overview: %v", o)
		}
		// Prices with VAT apart: growth 2,990 TL monthly, yearly 10 months.
		var growth map[string]any
		for _, p := range o["offer"].(map[string]any)["plans"].([]any) {
			if p.(map[string]any)["key"] == "growth" {
				growth = p.(map[string]any)
			}
		}
		if growth["monthly"] != "2990.00" || growth["yearly"] != "29900.00" || growth["current"] != true {
			t.Fatalf("offer: %v", growth)
		}
	})

	t.Run("a trial chooses a plan: a proforma with the reference and IBAN", func(t *testing.T) {
		r := hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "growth", "billing_cycle": "monthly"}})
		hb.expect(r, 200, "")
		if r.data()["effect"] != "proforma" {
			t.Fatalf("change: %s", r.raw)
		}
		trialProforma = r.data()["proforma"].(map[string]any)
		f := trialProforma
		if f["kind"] != "subscription" || f["net"] != "2990.00" || f["vat"] != "598.00" || f["total"] != "3588.00" || f["vat_rate_percent"] != "20" ||
			f["status"] != "open" || f["due"] != "3588.00" {
			t.Fatalf("proforma: %v", f)
		}
		ref := f["reference"].(string)
		if _, ok := billing.ParseReference(ref); !ok || !strings.HasPrefix(ref, "MSR"+sub(hb.billingOverview(ownerB))["code"].(string)) {
			t.Fatalf("reference %q", ref)
		}
		banks := f["seller"].(map[string]any)["bank_accounts"].([]any)
		if len(banks) != 1 || banks[0].(map[string]any)["iban"] != testIBAN || f["buyer"].(map[string]any)["tax_number"] != "1234567890" {
			t.Fatalf("seller/buyer: %v %v", f["seller"], f["buyer"])
		}
		if hb.mailsWith(f["number"].(string)) == 0 {
			t.Fatal("no invoice_issued e-mail")
		}
		// The PDF downloads, branded, from the workspace.
		code, body, hdr := hb.uploadAt(hb.host, ownerB, "GET", "/api/v1/billing/proformas/"+f["id"].(string)+"/pdf", "", nil)
		if code != 200 || !bytes.HasPrefix(body, []byte("%PDF-")) || hdr.Get("Content-Type") != "application/pdf" {
			t.Fatalf("pdf: %d %s", code, hdr.Get("Content-Type"))
		}
		// Choosing again replaces the unpaid proforma (one open plan proforma).
		r = hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "growth", "billing_cycle": "yearly"}})
		hb.expect(r, 200, "")
		if hb.proforma(ownerB, f["id"].(string))["status"] != "void" || r.data()["proforma"].(map[string]any)["total"] != "35880.00" {
			t.Fatalf("replace: %s", r.raw)
		}
		r = hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "growth", "billing_cycle": "monthly"}})
		hb.expect(r, 200, "")
		trialProforma = r.data()["proforma"].(map[string]any)
		// Custom-priced plans are not self-serve.
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "enterprise", "billing_cycle": "monthly"}}),
			409, "PLAN_NOT_SELF_SERVE")
	})

	t.Run("the trial ends unpaid: read-only, once", func(t *testing.T) {
		if res := hb.billingTick(); len(res.Transitions) != 0 {
			t.Fatalf("during the trial: %+v", res)
		}
		clk.set(trialEnd.Add(time.Minute))
		suspended := func() int { return hb.mailsWith("salt okunur") + hb.mailsWith("read-only") }
		before := suspended()
		if res := hb.billingTick(); len(res.Transitions) != 1 || res.Transitions[0] != "trial→read_only" {
			t.Fatalf("trial end: %+v", res)
		}
		// The owner and the billing contact, once.
		after := suspended()
		if res := hb.billingTick(); len(res.Transitions) != 0 {
			t.Fatalf("second run: %+v", res)
		}
		if hb.subStatus() != "read_only" || after != before+2 || suspended() != after {
			t.Fatalf("status %s, suspended mails %d → %d → %d", hb.subStatus(), before, after, suspended())
		}
		me := hb.do(call{method: "GET", path: "/api/v1/me", token: ownerB}).data()["tenant"].(map[string]any)["plan"].(map[string]any)
		if me["read_only"] != true || me["subscription_status"] != "read_only" {
			t.Fatalf("/me: %v", me)
		}
		// Reading and billing work; creating is refused with 402.
		hb.expect(hb.do(call{method: "GET", path: "/api/v1/contacts", token: ownerB}), 200, "")
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/contacts", token: ownerB, body: map[string]any{"phone": "+905550001111"}}), 402, billing.CodeReadOnly)
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/tags", token: ownerB, body: map[string]any{"name": "x"}}), 402, billing.CodeReadOnly)
		hb.expect(hb.do(call{method: "PATCH", path: "/api/v1/me", token: ownerB, body: map[string]any{"name": "Owner Billco"}}), 200, "")
		hb.billingProfile(ownerB)
	})

	t.Run("payment by reference brings it back to active at once", func(t *testing.T) {
		f := trialProforma
		tr := h.transfer("3588.00", "EFT "+strings.ToLower(f["reference_display"].(string))+" abonelik")
		if tr["suggestion"] != "reference_amount" || tr["suggested_proforma"].(map[string]any)["id"] != f["id"] || tr["status"] != "unmatched" {
			t.Fatalf("suggestion: %v", tr)
		}
		// Nothing changes before the admin confirms.
		if hb.subStatus() != "read_only" {
			t.Fatal("matched without confirmation")
		}
		m := h.match(tr["id"].(string), f["id"].(string))
		if m["status"] != "matched" || m["match_method"] != "reference_amount" || m["tenant"].(map[string]any)["id"] != hb.tenantID.String() {
			t.Fatalf("match: %v", m)
		}
		h.expect(h.adminDo(call{method: "POST", path: "/billing/transfers/" + tr["id"].(string) + "/match", body: map[string]any{"proforma_id": f["id"]}}),
			409, "TRANSFER_NOT_UNMATCHED")
		o := hb.billingOverview(ownerB)
		s := sub(o)
		if s["status"] != "active" || s["read_only"] != false || s["plan_key"] != "growth" || s["period_end"] == nil {
			t.Fatalf("after payment: %v", s)
		}
		end, _ := time.Parse(time.RFC3339, s["period_end"].(string))
		if want := billing.AddCycle(clk.Now(), "monthly"); end.Sub(want).Abs() > time.Second {
			t.Fatalf("period end %v, want %v", end, want)
		}
		if hb.proforma(ownerB, f["id"].(string))["status"] != "paid" {
			t.Fatal("proforma not paid")
		}
		r := hb.do(call{method: "GET", path: "/api/v1/billing/invoices", token: ownerB})
		hb.expect(r, 200, "")
		if len(r.list()) != 1 || r.list()[0].(map[string]any)["status"] != "pending" || r.list()[0].(map[string]any)["issuer"] != "manual" {
			t.Fatalf("invoice record: %s", r.raw)
		}
		if hb.mailsWith("Ödemeniz alındı")+hb.mailsWith("Payment received")+hb.mailsWith("ödeme") == 0 {
			t.Fatalf("no payment_received e-mail")
		}
		hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+905550001111"})
		// The official invoice: number, then the PDF; the workspace downloads it.
		inv := r.list()[0].(map[string]any)["id"].(string)
		hb.expect(hb.do(call{method: "GET", path: "/api/v1/billing/invoices/" + inv + "/pdf", token: ownerB}), 409, "INVOICE_PDF_PENDING")
		rec := h.adminDo(call{method: "PATCH", path: "/billing/invoices/" + inv, body: map[string]any{"official_number": "MSR2026000000001", "issued_on": "2026-11-28"}})
		h.expect(rec, 200, "")
		if rec.data()["status"] != "issued" || rec.data()["official_number"] != "MSR2026000000001" {
			t.Fatalf("record: %s", rec.raw)
		}
		code, _, _ := h.uploadAt(adminHost, h.platformSession(), "PUT", "/api/v1/platform/billing/invoices/"+inv+"/pdf", "application/pdf", []byte("not a pdf"))
		if code != 422 {
			t.Fatalf("non-PDF upload: %d", code)
		}
		pdf := []byte("%PDF-1.4\n% official invoice\n")
		if code, body, _ := h.uploadAt(adminHost, h.platformSession(), "PUT", "/api/v1/platform/billing/invoices/"+inv+"/pdf", "application/pdf", pdf); code != 200 {
			t.Fatalf("upload: %d %s", code, body)
		}
		code, body, _ := hb.uploadAt(hb.host, ownerB, "GET", "/api/v1/billing/invoices/"+inv+"/pdf", "", nil)
		if code != 200 || !bytes.Equal(body, pdf) {
			t.Fatalf("download: %d %q", code, body)
		}
	})

	var renewal map[string]any
	t.Run("renewal: proforma before the end, reminders 7/3/0 once each, grace, read-only", func(t *testing.T) {
		s := sub(hb.billingOverview(ownerB))
		end, _ := time.Parse(time.RFC3339, s["period_end"].(string))
		issued := func() int { return hb.count("SELECT count(*) FROM billing_events WHERE kind = 'renewal'") }
		clk.set(end.AddDate(0, 0, -11))
		if res := hb.billingTick(); len(res.Issued) != 0 || issued() != 0 {
			t.Fatalf("too early: %+v", res)
		}
		clk.set(end.AddDate(0, 0, -10).Add(time.Minute))
		res := hb.billingTick()
		if len(res.Issued) != 1 || len(res.Reminders) != 0 {
			t.Fatalf("renewal: %+v", res)
		}
		if res := hb.billingTick(); len(res.Issued) != 0 || issued() != 1 {
			t.Fatalf("renewal issued twice: %+v", res)
		}
		for _, f := range hb.billingOverview(ownerB)["open_proformas"].([]any) {
			if f.(map[string]any)["renewal"] == true {
				renewal = f.(map[string]any)
			}
		}
		if renewal == nil || renewal["total"] != "3588.00" || renewal["due_at"] == nil {
			t.Fatalf("renewal: %v", renewal)
		}
		ps, _ := time.Parse(time.RFC3339, renewal["period_start"].(string))
		if !ps.Equal(end) {
			t.Fatalf("renewal period %v, want from %v", ps, end)
		}
		steps := []struct {
			at   time.Time
			want string
		}{{end.AddDate(0, 0, -6), billing.Reminder7}, {end.AddDate(0, 0, -5), ""}, {end.AddDate(0, 0, -2), billing.Reminder3},
			{end.Add(-12 * time.Hour), billing.Reminder0}, {end.Add(-time.Hour), ""}}
		for _, st := range steps {
			clk.set(st.at)
			// A retried job sends nothing again.
			for i := range 2 {
				res := hb.billingTick()
				got := strings.Join(res.Reminders, ",")
				if (i == 0 && got != st.want) || (i == 1 && got != "") {
					t.Fatalf("at %v run %d: reminders %q, want %q", st.at, i, got, st.want)
				}
			}
		}
		for _, k := range []string{billing.Reminder7, billing.Reminder3, billing.Reminder0} {
			if n := hb.count("SELECT count(*) FROM billing_events WHERE kind = $1", k); n != 1 {
				t.Fatalf("%s sent %d times", k, n)
			}
		}
		clk.set(end.Add(time.Hour))
		if res := hb.billingTick(); strings.Join(res.Transitions, ",") != "active→past_due" {
			t.Fatalf("due: %+v", res)
		}
		s = sub(hb.billingOverview(ownerB))
		grace, _ := time.Parse(time.RFC3339, s["grace_ends_at"].(string))
		if s["status"] != "past_due" || s["read_only"] != false || !grace.Equal(end.AddDate(0, 0, 7)) {
			t.Fatalf("past due: %v", s)
		}
		// During the grace the workspace still works.
		hb.created(ownerB, "/api/v1/tags", map[string]any{"name": "grace"})
		clk.set(end.AddDate(0, 0, 7).Add(time.Minute))
		if res := hb.billingTick(); strings.Join(res.Transitions, ",") != "past_due→read_only" {
			t.Fatalf("grace end: %+v", res)
		}
		if res := hb.billingTick(); len(res.Transitions) != 0 {
			t.Fatalf("again: %+v", res)
		}
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/tags", token: ownerB, body: map[string]any{"name": "late"}}), 402, billing.CodeReadOnly)
	})

	t.Run("a partial payment holds; the rest pays it; a lapsed period restarts", func(t *testing.T) {
		tr := h.transfer("1000.00", "odeme "+renewal["reference"].(string))
		if tr["suggestion"] != "reference" {
			t.Fatalf("partial suggestion: %v", tr)
		}
		h.match(tr["id"].(string), renewal["id"].(string))
		f := hb.proforma(ownerB, renewal["id"].(string))
		if f["status"] != "partially_paid" || f["paid"] != "1000.00" || f["due"] != "2588.00" || hb.subStatus() != "read_only" {
			t.Fatalf("partial: %v %s", f, hb.subStatus())
		}
		// The rest, without a reference: one open proforma owes exactly this.
		tr = h.transfer("2588.00", "ACME GIDA HAVALE")
		if tr["suggestion"] != "amount" || tr["suggested_proforma"].(map[string]any)["id"] != renewal["id"] {
			t.Fatalf("amount suggestion: %v", tr)
		}
		h.match(tr["id"].(string), renewal["id"].(string))
		s := sub(hb.billingOverview(ownerB))
		start, _ := time.Parse(time.RFC3339, s["period_start"].(string))
		if s["status"] != "active" || start.Sub(clk.Now()).Abs() > time.Second {
			t.Fatalf("restart: %v (now %v)", s, clk.Now())
		}
	})

	t.Run("upgrade is prorated, a downgrade waits for the renewal", func(t *testing.T) {
		s := sub(hb.billingOverview(ownerB))
		ps, _ := time.Parse(time.RFC3339, s["period_start"].(string))
		pe, _ := time.Parse(time.RFC3339, s["period_end"].(string))
		clk.set(ps.AddDate(0, 0, 10))
		r := hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "pro", "billing_cycle": "monthly"}})
		hb.expect(r, 200, "")
		f := r.data()["proforma"].(map[string]any)
		want := billing.Prorate(649000-299000, clk.Now(), ps, pe)
		if r.data()["effect"] != "proforma" || f["kind"] != "upgrade" || f["net"] != billing.FormatKurus(want) ||
			f["total"] != billing.FormatKurus(want+billing.VAT(want, 2000)) {
			t.Fatalf("upgrade: %s (want net %d)", r.raw, want)
		}
		// Nothing changes until it is paid.
		if hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()["key"] != "growth" {
			t.Fatal("plan changed before payment")
		}
		tr := h.transfer(f["total"].(string), f["reference"].(string))
		h.match(tr["id"].(string), f["id"].(string))
		if k := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()["key"]; k != "pro" {
			t.Fatalf("plan after the upgrade: %v", k)
		}
		if after := sub(hb.billingOverview(ownerB)); after["period_end"] != s["period_end"] {
			t.Fatalf("an upgrade keeps the period: %v", after)
		}
		// A lower plan is scheduled for the renewal.
		r = hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "starter", "billing_cycle": "monthly"}})
		hb.expect(r, 200, "")
		if r.data()["effect"] != "scheduled" || r.data()["proforma"] != nil || sub(r.data())["pending_plan_key"] != "starter" {
			t.Fatalf("downgrade: %s", r.raw)
		}
		if hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()["key"] != "pro" {
			t.Fatal("downgrade applied before the period end")
		}
		clk.set(pe.AddDate(0, 0, -9))
		if res := hb.billingTick(); len(res.Issued) != 1 {
			t.Fatalf("renewal: %+v", res)
		}
		var next map[string]any
		for _, x := range hb.billingOverview(ownerB)["open_proformas"].([]any) {
			next = x.(map[string]any)
		}
		if next["plan_key"] != "starter" || next["total"] != "1548.00" {
			t.Fatalf("renewal with the lower plan: %v", next)
		}
		// Changing the cycle inside the renewal window re-issues the renewal.
		r = hb.do(call{method: "POST", path: "/api/v1/billing/plan-change", token: ownerB, body: map[string]any{"plan_key": "starter", "billing_cycle": "yearly"}})
		hb.expect(r, 200, "")
		if hb.proforma(ownerB, next["id"].(string))["status"] != "void" {
			t.Fatal("old renewal not replaced")
		}
		for _, x := range hb.billingOverview(ownerB)["open_proformas"].([]any) {
			next = x.(map[string]any)
		}
		if next["billing_cycle"] != "yearly" || next["total"] != "15480.00" || next["renewal"] != true {
			t.Fatalf("yearly renewal: %v", next)
		}
		tr = h.transfer("15480.00", next["reference"].(string))
		h.match(tr["id"].(string), next["id"].(string))
		clk.set(pe.Add(time.Hour))
		s = sub(hb.billingOverview(ownerB))
		renewedFrom, _ := time.Parse(time.RFC3339, s["period_start"].(string))
		if s["plan_key"] != "starter" || s["billing_cycle"] != "yearly" || s["pending_plan_key"] != nil || !renewedFrom.Equal(pe) {
			t.Fatalf("renewed: %v (want from %v)", s, pe)
		}
	})

	t.Run("AI credit packs grant purchased credits", func(t *testing.T) {
		before := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()["ai_credits"].(map[string]any)["granted"].(float64)
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/billing/proformas", token: ownerB, body: map[string]any{"kind": "ai_credits", "pack_key": "nope"}}),
			422, "VALIDATION_ERROR")
		r := hb.do(call{method: "POST", path: "/api/v1/billing/proformas", token: ownerB, body: map[string]any{"kind": "ai_credits", "pack_key": "c1000"}})
		hb.expect(r, 201, "")
		f := r.data()
		if f["total"] != "708.00" || f["ai_credits"] != float64(1000) {
			t.Fatalf("pack: %v", f)
		}
		tr := h.transfer("708.00", f["reference"].(string))
		h.match(tr["id"].(string), f["id"].(string))
		after := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()["ai_credits"].(map[string]any)["granted"].(float64)
		if after-before != 1000 {
			t.Fatalf("credits %v → %v", before, after)
		}
		if hb.count("SELECT count(*) FROM ai_credit_lots WHERE source = 'purchase' AND reference = $1 AND expires_at IS NOT NULL", f["reference"]) != 1 {
			t.Fatal("no purchase lot")
		}
	})

	t.Run("matching: ambiguous amounts stay manual; overpayments become account credit", func(t *testing.T) {
		hc := h.tenant("billtwo")
		ownerC := hc.setupOwnerAs("owner@billtwo.test")
		hc.setSubscription("active", nil, "growth")
		hc.billingProfile(ownerC)
		fb := hb.do(call{method: "POST", path: "/api/v1/billing/proformas", token: ownerB, body: map[string]any{"kind": "ai_credits", "pack_key": "c1000"}}).data()
		fc := hc.do(call{method: "POST", path: "/api/v1/billing/proformas", token: ownerC, body: map[string]any{"kind": "ai_credits", "pack_key": "c1000"}}).data()
		tr := h.transfer("708.00", "havale")
		if tr["suggestion"] != nil || len(tr["candidates"].([]any)) != 2 {
			t.Fatalf("ambiguous: %v", tr)
		}
		// The admin chooses; the other stays open.
		h.match(tr["id"].(string), fc["id"].(string))
		if hc.proforma(ownerC, fc["id"].(string))["status"] != "paid" || hb.proforma(ownerB, fb["id"].(string))["status"] != "open" {
			t.Fatal("manual match")
		}
		// 1,000 TL for a 708 TL proforma: 292 TL account credit, applied to
		// the next proforma at once.
		tr = h.transfer("1000.00", fb["reference"].(string))
		if tr["suggestion"] != "reference" {
			t.Fatalf("overpayment suggestion: %v", tr)
		}
		h.match(tr["id"].(string), fb["id"].(string))
		if s := sub(hb.billingOverview(ownerB)); s["account_credit"] != "292.00" {
			t.Fatalf("account credit: %v", s["account_credit"])
		}
		f := hb.do(call{method: "POST", path: "/api/v1/billing/proformas", token: ownerB, body: map[string]any{"kind": "ai_credits", "pack_key": "c1000"}}).data()
		if f["status"] != "partially_paid" || f["paid"] != "292.00" || f["due"] != "416.00" || sub(hb.billingOverview(ownerB))["account_credit"] != "0.00" {
			t.Fatalf("credit applied: %v", f)
		}
		// A transfer for a company only goes to its account credit.
		tr = h.transfer("50.00", "bilinmeyen")
		h.expect(h.adminDo(call{method: "POST", path: "/billing/transfers/" + tr["id"].(string) + "/match", body: map[string]any{}}), 422, "VALIDATION_ERROR")
		h.expect(h.adminDo(call{method: "POST", path: "/billing/transfers/" + tr["id"].(string) + "/match", body: map[string]any{"tenant_id": hc.tenantID}}), 200, "")
		if s := sub(hc.billingOverview(ownerC)); s["account_credit"] != "50.00" {
			t.Fatalf("company credit: %v", s)
		}
		// Not a customer payment.
		tr = h.transfer("12.00", "faiz")
		h.expect(h.adminDo(call{method: "POST", path: "/billing/transfers/" + tr["id"].(string) + "/ignore", body: map[string]any{"reason": "bank interest"}}), 200, "")
		// A wrong check character is not a reference.
		h.expect(h.adminDo(call{method: "POST", path: "/billing/transfers", body: map[string]any{"amount": "1.00", "received_on": "2026-11-28",
			"reference": f["reference"].(string)[:11] + "U"}}), 422, "VALIDATION_ERROR")
	})

	t.Run("bank statement CSV: imported once", func(t *testing.T) {
		f := hb.do(call{method: "GET", path: "/api/v1/billing/proformas", token: ownerB}).list()[0].(map[string]any)
		csv := "Tarih;Tutar;Açıklama;Gönderen\n28.11.2026;416,00;" + f["reference_display"].(string) + " AI paketi;ACME GIDA\n" +
			"28.11.2026;-50,00;masraf;BANKA\n28.11.2026;12,00;faiz;BANKA\n28.11.2026;12,00;faiz;BANKA\n"
		import1 := func() map[string]any {
			code, body, _ := h.uploadAt(adminHost, h.platformSession(), "POST", "/api/v1/platform/billing/transfers/import", "text/csv", []byte(csv))
			if code != 200 {
				t.Fatalf("import: %d %s", code, body)
			}
			return decodeData(t, body)
		}
		r := import1()
		if r["created"] != float64(3) || r["duplicates"] != float64(0) || len(r["errors"].([]any)) != 0 {
			t.Fatalf("import: %v", r)
		}
		if tr := r["transfers"].([]any)[0].(map[string]any); tr["suggestion"] != "reference_amount" || tr["source"] != "csv" {
			t.Fatalf("csv suggestion: %v", tr)
		}
		if r = import1(); r["created"] != float64(0) || r["duplicates"] != float64(3) {
			t.Fatalf("re-import: %v", r)
		}
	})

	t.Run("isolation: proformas, invoices and payments stay in their workspace", func(t *testing.T) {
		hc := h.tenant("billthree")
		ownerC := hc.setupOwnerAs("owner@billthree.test")
		fb := hb.do(call{method: "GET", path: "/api/v1/billing/proformas", token: ownerB}).list()[0].(map[string]any)
		inv := hb.do(call{method: "GET", path: "/api/v1/billing/invoices", token: ownerB}).list()[0].(map[string]any)
		hc.expect(hc.do(call{method: "GET", path: "/api/v1/billing/proformas/" + fb["id"].(string), token: ownerC}), 404, "")
		hc.expect(hc.do(call{method: "POST", path: "/api/v1/billing/proformas/" + fb["id"].(string) + "/cancel", token: ownerC}), 404, "")
		if code, _, _ := hc.uploadAt(hc.host, ownerC, "GET", "/api/v1/billing/proformas/"+fb["id"].(string)+"/pdf", "", nil); code != 404 {
			t.Fatalf("other's pdf: %d", code)
		}
		if code, _, _ := hc.uploadAt(hc.host, ownerC, "GET", "/api/v1/billing/invoices/"+inv["id"].(string)+"/pdf", "", nil); code != 404 {
			t.Fatalf("other's invoice: %d", code)
		}
		if l := hc.do(call{method: "GET", path: "/api/v1/billing/proformas", token: ownerC}).list(); len(l) != 0 {
			t.Fatalf("lists another's proformas: %v", l)
		}
		if l := hc.do(call{method: "GET", path: "/api/v1/billing/invoices", token: ownerC}).list(); len(l) != 0 {
			t.Fatal("lists another's invoices")
		}
		// The application role cannot write billing rows or see transfers.
		for _, stmt := range []string{"UPDATE proformas SET status = 'paid'", "INSERT INTO billing_payments (proforma_id, source, amount_kurus, received_on) VALUES (gen_random_uuid(), 'transfer', 1, now())",
			"UPDATE subscriptions SET account_credit_kurus = 1000000", "SELECT count(*) FROM bank_transfers"} {
			if _, err := hc.pool.Exec(hc.ctx(), stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Errorf("%s: %v", stmt, err)
			}
		}
		// Workspaces never reach the platform's billing API.
		for _, p := range []string{"/api/v1/platform/billing/transfers", "/api/v1/platform/billing/overview", "/api/v1/platform/tenants/" + hb.tenantID.String() + "/billing"} {
			if r := hc.do(call{method: "GET", path: p, token: ownerC}); r.status != 404 {
				t.Errorf("%s on a workspace host: %d", p, r.status)
			}
			if code, _ := hc.rawAt(adminHost, "GET", p, ownerC, true); code != 401 {
				t.Errorf("%s with a workspace session: %d", p, code)
			}
		}
	})

	t.Run("platform panel: overview, lists, actions, settings", func(t *testing.T) {
		clk.set(time.Now()) // collections are counted by calendar month
		o := h.adminDo(call{method: "GET", path: "/billing/overview"})
		h.expect(o, 200, "")
		if o.data()["mrr"] == "0.00" || o.data()["collected_this_month"] == "0.00" || o.data()["pending_invoices"].(float64) < 1 {
			t.Fatalf("overview: %s", o.raw)
		}
		l := h.adminDo(call{method: "GET", path: "/billing/subscriptions?status=active"})
		h.expect(l, 200, "")
		found := false
		for _, x := range l.list() {
			if x.(map[string]any)["tenant"].(map[string]any)["id"] == hb.tenantID.String() {
				found = true
			}
		}
		if !found {
			t.Fatalf("subscriptions: %s", l.raw)
		}
		h.expect(h.adminDo(call{method: "GET", path: "/billing/proformas?status=open"}), 200, "")
		h.expect(h.adminDo(call{method: "GET", path: "/billing/invoices?status=pending"}), 200, "")
		h.expect(h.adminDo(call{method: "GET", path: "/billing/transfers?status=unmatched"}), 200, "")
		id := hb.tenantID.String()
		// Every action needs a reason and lands in both audit logs.
		h.expect(h.adminDo(call{method: "POST", path: "/tenants/" + id + "/billing/actions", body: map[string]any{"action": "comp_months", "months": 1, "reason": "x"}}),
			422, "VALIDATION_ERROR")
		before := sub(hb.billingOverview(ownerB))["period_end"].(string)
		r := h.adminDo(call{method: "POST", path: "/tenants/" + id + "/billing/actions", body: map[string]any{"action": "comp_months", "months": 2, "reason": "launch gift"}})
		h.expect(r, 200, "")
		b, _ := time.Parse(time.RFC3339, before)
		a, _ := time.Parse(time.RFC3339, sub(r.data())["period_end"].(string))
		if !a.Equal(billing.AddCycle(billing.AddCycle(b, "monthly"), "monthly")) {
			t.Fatalf("comp: %v → %v", b, a)
		}
		h.expect(h.adminDo(call{method: "POST", path: "/tenants/" + id + "/billing/actions", body: map[string]any{"action": "set_status", "status": "read_only", "reason": "chargeback"}}), 200, "")
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/tags", token: ownerB, body: map[string]any{"name": "blocked"}}), 402, billing.CodeReadOnly)
		h.expect(h.adminDo(call{method: "POST", path: "/tenants/" + id + "/billing/actions", body: map[string]any{"action": "set_status", "status": "active", "reason": "resolved"}}), 200, "")
		hb.created(ownerB, "/api/v1/tags", map[string]any{"name": "unblocked"})
		if hb.count("SELECT count(*) FROM audit_logs WHERE action = 'billing.admin_set_status'") != 2 {
			t.Fatal("tenant audit")
		}
		var n int
		_ = h.owner.QueryRow(context.Background(), "SELECT count(*) FROM platform_audit_logs WHERE action = 'tenant.billing_comp_months' AND tenant_id = $1", hb.tenantID).Scan(&n)
		if n != 1 {
			t.Fatal("platform audit")
		}
		// Invalid IBANs are refused.
		h.expect(h.adminDo(call{method: "PUT", path: "/billing/settings", body: map[string]any{"seller": map[string]any{"company_name": "X", "tax_office": "",
			"tax_number": "", "address": "", "email": "", "phone": "", "website": "", "invoice_note": "",
			"bank_accounts": []any{map[string]any{"bank_name": "B", "account_holder": "X", "iban": "TR330006100519786457841327"}}}}}), 422, "VALIDATION_ERROR")
		st := h.adminDo(call{method: "GET", path: "/billing/settings"})
		h.expect(st, 200, "")
		p := st.data()["pricing"].(map[string]any)
		p["fx_usd_try"] = "43.10"
		r = h.adminDo(call{method: "PUT", path: "/billing/settings", body: map[string]any{"pricing": p}})
		h.expect(r, 200, "")
		if r.data()["pricing"].(map[string]any)["fx_usd_try"] != "43.10" || r.data()["pricing"].(map[string]any)["fx_updated_at"] == nil {
			t.Fatalf("pricing: %s", r.raw)
		}
	})
}

// decodeData reads a response envelope's data.
func decodeData(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

// uploadAt sends a raw body to host.
func (h *harness) uploadAt(host, token, method, path, contentType string, body []byte) (int, []byte, http.Header) {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, bytes.NewReader(body))
	req.Host = host
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set(tenant.HostHeader, host)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw, res.Header
}

// setMethod switches the harness account between its own Meta app and
// Messora's Tech Provider (the message balance applies to the latter).
func (h *harness) setMethod(method string) {
	h.t.Helper()
	if _, err := h.owner.Exec(context.Background(), `UPDATE whatsapp_accounts SET connection_method = $2 WHERE tenant_id = $1`, h.tenantID, method); err != nil {
		h.t.Fatal(err)
	}
}

// webhookAs delivers a webhook (signed with the account's own app secret)
// and processes it while the account uses method.
func (h *harness) webhookAs(method string, body []byte) {
	h.t.Helper()
	if _, err := whatsapptest.Deliver(h.ctx(), h.srv.URL+"/webhooks/whatsapp", testAppSecret, body); err != nil {
		h.t.Fatal(err)
	}
	h.setMethod(method)
	h.processWebhooks()
	h.setMethod("own_app")
}

func (h *harness) wallet() int64 {
	h.t.Helper()
	var b int64
	if err := h.pool.QueryRow(h.ctx(), "SELECT coalesce((SELECT balance_micros FROM message_wallets), 0)").Scan(&b); err != nil {
		h.t.Fatal(err)
	}
	return b
}

func (h *harness) adjustBalance(amount string) {
	h.t.Helper()
	h.expect(h.adminDo(call{method: "POST", path: "/tenants/" + h.tenantID.String() + "/billing/actions",
		body: map[string]any{"action": "adjust_message_balance", "amount": amount, "reason": "test top-up"}}), 200, "")
}

func TestBillingReadOnlyAndMessageBalance(t *testing.T) {
	h := newHarness(t)
	h.setupSeller()
	token, phoneID := h.connect()
	h.expect(h.uploadRates(token, testRateCard), 200, "")
	billable := true

	t.Run("own Meta app: Meta bills the company, nothing is debited", func(t *testing.T) {
		id, wamid := h.sendTemplate(token, "+"+testCustomer, "order_update")
		h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "sent", testCustomer, id, time.Now(), 0,
			whatsapp.Pricing{Type: "regular", Category: "utility", PricingModel: "PMP", Billable: &billable}))
		if h.count("SELECT count(*) FROM message_wallet_entries") != 0 {
			t.Fatal("an own-app message was charged")
		}
		if o := h.billingOverview(token); o["message_balance"].(map[string]any)["used"] != false {
			t.Fatalf("balance used: %v", o["message_balance"])
		}
	})

	t.Run("Tech Provider: templates need balance; each priced message is debited once", func(t *testing.T) {
		h.setMethod("tech_provider")
		r := h.send(token, map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}})
		h.expect(r, 402, billing.CodeBalanceExhausted)
		if me := h.do(call{method: "GET", path: "/api/v1/me", token: token}).data()["tenant"].(map[string]any)["plan"].(map[string]any); me["message_balance_exhausted"] != true {
			t.Fatalf("/me: %v", me)
		}
		h.adjustBalance("100.00")
		h.setMethod("own_app")
		id, wamid := h.sendTemplate(token, "+"+testCustomer, "order_update")
		// Utility, tier 2 this month ($0.003): ($0.003 + $0.0006) × 42 = 0.1512 TL.
		st := whatsapptest.StatusUpdate(testPNID, wamid, "sent", testCustomer, id, time.Now(), 0,
			whatsapp.Pricing{Type: "regular", Category: "utility", PricingModel: "PMP", Billable: &billable})
		h.webhookAs("tech_provider", st)
		h.webhookAs("tech_provider", st) // Meta repeats webhooks
		want := int64(100_000_000 - 151_200)
		if got := h.wallet(); got != want {
			t.Fatalf("wallet %d, want %d", got, want)
		}
		if h.count("SELECT count(*) FROM message_wallet_entries WHERE kind = 'charge' AND message_id = $1", id) != 1 {
			t.Fatal("charged twice")
		}
		// A free message costs nothing.
		id2, w2 := h.sendTemplate(token, "+"+testCustomer, "order_update")
		h.webhookAs("tech_provider", whatsapptest.StatusUpdate(testPNID, w2, "sent", testCustomer, id2, time.Now(), 0,
			whatsapp.Pricing{Type: "free_customer_service", Category: "service", PricingModel: "PMP"}))
		if h.wallet() != want {
			t.Fatal("a free message was charged")
		}
		// Below the alert threshold (200 TL): one credit_low e-mail.
		var n int
		_ = h.pool.QueryRow(h.ctx(), "SELECT count(*) FROM outbox_events WHERE topic = 'task:billing:notify'").Scan(&n)
		if n != 1 {
			t.Fatalf("low balance notices queued: %d", n)
		}
		before := h.mailsWith("azaldı") + h.mailsWith("running low")
		if err := h.svc.billing.HandleNotify(h.ctx(), []byte(`{"kind":"message_low"}`)); err != nil {
			t.Fatal(err)
		}
		if h.mailsWith("azaldı")+h.mailsWith("running low") != before+1 {
			t.Fatal("no credit_low e-mail")
		}
		d := h.do(call{method: "GET", path: "/api/v1/billing/message-balance", token: token})
		h.expect(d, 200, "")
		e := d.data()["entries"].([]any)[0].(map[string]any)
		if e["kind"] != "charge" || e["amount"] != "-0.151200" || e["meta_usd"] != "0.003000" || !strings.HasPrefix(fmt.Sprint(e["fx_rate"]), "42") {
			t.Fatalf("entry: %v", e)
		}
	})

	t.Run("concurrent debits lose nothing", func(t *testing.T) {
		h.setMethod("tech_provider")
		defer h.setMethod("own_app")
		pn := uuid.MustParse(phoneID)
		start := h.wallet()
		now := time.Now()
		p := metacost.Pricing{Type: "regular", Category: "marketing", Model: "PMP"}
		ids := make([]uuid.UUID, 40)
		for i := range ids {
			ids[i] = uuid.New()
		}
		var wg sync.WaitGroup
		errs := make(chan error, 2*len(ids))
		for _, id := range append(ids, ids...) { // each message twice, concurrently
			wg.Add(1)
			go func() {
				defer wg.Done()
				tx, err := h.pool.Begin(h.ctx())
				if err != nil {
					errs <- err
					return
				}
				defer func() { _ = tx.Rollback(h.ctx()) }()
				m := db.Message{ID: id, PhoneNumberID: pn, SentAt: &now}
				if err := h.svc.billing.ChargeMessage(h.ctx(), tx, db.New(tx), m, p, "+"+testCustomer); err != nil {
					errs <- err
					return
				}
				errs <- tx.Commit(h.ctx())
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		// Marketing $0.015 × 1.20 × 42 = 0.756 TL each.
		if got := start - h.wallet(); got != int64(len(ids))*756_000 {
			t.Fatalf("debited %d, want %d", got, int64(len(ids))*756_000)
		}
		var last int64
		_ = h.pool.QueryRow(h.ctx(), "SELECT balance_after FROM message_wallet_entries WHERE kind = 'charge' ORDER BY balance_after LIMIT 1").Scan(&last)
		if last != h.wallet() {
			t.Fatalf("lowest balance_after %d, wallet %d", last, h.wallet())
		}
	})

	t.Run("campaigns: refused without balance, paused when it runs out", func(t *testing.T) {
		tag := h.seedContacts(5, "+9055590")
		c := h.createCampaign(token, tag, 2, nil)
		empty := func() {
			if b := h.wallet(); b != 0 {
				h.adjustBalance(billing.FormatKurus(-billing.MicrosToKurus(b)))
			}
		}
		h.setMethod("tech_provider")
		empty()
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + c + "/start", token: token}), 402, billing.CodeBalanceExhausted)
		h.adjustBalance("50.00")
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + c + "/start", token: token}), 200, "")
		// The fake Meta serves the account's own app; the balance gate is
		// what this test is about.
		h.setMethod("own_app")
		for range 10 {
			if h.status(c) == "running" {
				break
			}
			h.pump(h.ctx(), 2)
		}
		if h.status(c) != "running" {
			t.Fatalf("campaign %s", h.status(c))
		}
		h.setMethod("tech_provider")
		defer h.setMethod("own_app")
		empty()
		var gen int32
		_ = h.pool.QueryRow(h.ctx(), "SELECT dispatch_gen FROM campaigns WHERE id = $1", c).Scan(&gen)
		if err := h.svc.campaigns.Dispatch(h.ctx(), uuid.MustParse(c), gen); err != nil {
			t.Fatal(err)
		}
		if reason, _ := h.lastPause(c); h.status(c) != "paused" || reason != "message_balance" {
			t.Fatalf("pause: %s %s", h.status(c), reason)
		}
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + c + "/resume", token: token}), 402, billing.CodeBalanceExhausted)
		// An own-app number is never checked.
		h.setMethod("own_app")
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + c + "/resume", token: token}), 200, "")
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + c + "/cancel", token: token}), 200, "")
	})

	t.Run("read-only: sends, campaigns and automations stop; inbound is stored", func(t *testing.T) {
		auto := h.automation(token, map[string]any{"name": "Tag everyone", "active": true, "trigger_type": "message_received",
			"steps": []any{map[string]any{"type": "add_tag", "tag": "seen"}}})
		h.expect(h.adminDo(call{method: "POST", path: "/tenants/" + h.tenantID.String() + "/billing/actions",
			body: map[string]any{"action": "set_status", "status": "read_only", "reason": "unpaid"}}), 200, "")
		h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "hi"}}), 402, billing.CodeReadOnly)
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: map[string]any{"name": "x", "template_name": "order_update",
			"template_language": "en", "audience": map[string]any{"all": true}}}), 402, billing.CodeReadOnly)
		// Exports and previews still work.
		h.expect(h.do(call{method: "POST", path: "/api/v1/segments/preview", token: token, body: map[string]any{"definition": map[string]any{
			"field": "phone", "operator": "starts_with", "value": "+90"}}}), 200, "")
		// Inbound messages are stored; no automation runs.
		h.inbound(testCustomer, "wamid.readonly-1", "merhaba", time.Now())
		h.drain()
		if h.count("SELECT count(*) FROM messages WHERE wamid = 'wamid.readonly-1'") != 1 {
			t.Fatal("inbound not stored")
		}
		if h.runs(auto) != 0 {
			t.Fatal("an automation ran while read-only")
		}
		// Engines refuse too (an AI reply or an automation step sending).
		if _, err := h.svc.messaging.Queue(h.ctx(), messagingText(testCustomer)); err == nil || !strings.Contains(err.Error(), billing.CodeReadOnly) {
			t.Fatalf("engine send: %v", err)
		}
		h.expect(h.adminDo(call{method: "POST", path: "/tenants/" + h.tenantID.String() + "/billing/actions",
			body: map[string]any{"action": "set_status", "status": "active", "reason": "paid"}}), 200, "")
		h.inbound(testCustomer, "wamid.readonly-2", "tekrar", time.Now())
		h.drain()
		if h.runs(auto) != 1 {
			t.Fatalf("automation after reactivation: %d runs", h.runs(auto))
		}
	})
}

func messagingText(to string) messaging.SendInput {
	return messaging.SendInput{To: "+" + to, Type: "text", Text: &whatsapp.Text{Body: "hello"}, Source: "automation"}
}
