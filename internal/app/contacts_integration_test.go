//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
)

func whatsapptestInbound(from string) []byte {
	return whatsapptest.InboundText(testPNID, from, "wamid.optout-"+from, "help", "", time.Now())
}

// raw sends a non-JSON body (file upload) or reads a non-JSON response.
func (h *harness) raw(method, path, token, contentType string, body io.Reader) (int, []byte, http.Header) {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, body)
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b, res.Header
}

func TestContactsTagsGroupsAndConsent(t *testing.T) {
	h := newHarness(t)
	token, _ := h.setupOwner()

	r := h.do(call{method: "POST", path: "/api/v1/groups", token: token, body: map[string]any{"name": "Istanbul store"}})
	h.expect(r, 201, "")
	group := r.data()["id"].(string)

	r = h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{
		"phone": "+90 (555) 111-22-33", "name": "Ayşe Yılmaz", "email": "ayse@example.com", "var1": "Gold",
		"attributes": map[string]string{"city": "Istanbul"}, "tags": []string{"VIP", "vip", "newsletter"}, "group_ids": []string{group}}})
	h.expect(r, 201, "")
	c := r.data()
	id := c["id"].(string)
	if c["phone"] != "+905551112233" || len(c["tags"].([]any)) != 2 || len(c["group_ids"].([]any)) != 1 || c["source"] != "manual" {
		t.Fatalf("created contact: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{"phone": "905551112233"}}), 409, "CONTACT_EXISTS")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{"phone": "12345"}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{"phone": "+905550000000", "email": "not-an-email"}}), 422, "VALIDATION_ERROR")

	// Partial update: clear var1, replace tags, leave e-mail untouched.
	r = h.do(call{method: "PATCH", path: "/api/v1/contacts/" + id, token: token, body: map[string]any{"var1": "", "tags": []string{"Customer"}}})
	h.expect(r, 200, "")
	if r.data()["var1"] != nil || r.data()["email"] != "ayse@example.com" || r.data()["tags"].([]any)[0].(map[string]any)["name"] != "Customer" {
		t.Fatalf("update: %s", r.raw)
	}

	// Tags keep their first spelling and count contacts.
	r = h.do(call{method: "GET", path: "/api/v1/tags", token: token})
	h.expect(r, 200, "")
	counts := map[string]float64{}
	for _, x := range r.list() {
		m := x.(map[string]any)
		counts[m["name"].(string)] = m["contact_count"].(float64)
	}
	if counts["VIP"] != 0 || counts["Customer"] != 1 {
		t.Fatalf("tags: %v", counts)
	}

	// Notes.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/" + id + "/notes", token: token, body: map[string]any{"body": "Prefers mornings"}})
	h.expect(r, 201, "")
	if r.data()["author_name"] != "Ayşe Owner" {
		t.Fatalf("note: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/contacts/" + id + "/notes", token: token}), 200, "")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/contacts/" + id + "/notes/" + r.data()["id"].(string), token: token}), 204, "")

	// Consent is recorded and visible.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/" + id + "/consent", token: token, body: map[string]any{"kind": "opt_out", "note": "asked by phone"}})
	h.expect(r, 200, "")
	if r.data()["opted_out"] != true {
		t.Fatalf("opt-out: %s", r.raw)
	}
	if h.count("SELECT count(*) FROM consent_events WHERE contact_id = $1 AND kind = 'opt_out'", id) != 1 {
		t.Fatal("consent event missing")
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?opted_out=true", token: token})
	if len(r.list()) != 1 {
		t.Fatalf("opted-out filter: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + id + "/consent", token: token, body: map[string]any{"kind": "opt_in"}}), 200, "")

	// Groups: membership by API, counts.
	r = h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{"phone": "+14155550100", "name": "Bob"}})
	bob := r.data()["id"].(string)
	r = h.do(call{method: "POST", path: "/api/v1/groups/" + group + "/members", token: token, body: map[string]any{"action": "add", "contact_ids": []string{bob, id, uuid.NewString()}}})
	h.expect(r, 200, "")
	if r.data()["count"] != float64(1) {
		t.Fatalf("members added: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?group_id=" + group, token: token})
	if len(r.list()) != 2 {
		t.Fatalf("group filter: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/contacts?q=ayşe", token: token}), 200, "")
	if len(h.do(call{method: "GET", path: "/api/v1/contacts?q=%2B1415", token: token}).list()) != 1 {
		t.Fatal("phone search")
	}

	// Pagination.
	r = h.do(call{method: "GET", path: "/api/v1/contacts?limit=1", token: token})
	next, _ := r.body["meta"].(map[string]any)["next_cursor"].(string)
	if len(r.list()) != 1 || next == "" {
		t.Fatalf("page 1: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?limit=1&cursor=" + next, token: token})
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["id"] == next {
		t.Fatalf("page 2: %s", r.raw)
	}

	h.expect(h.do(call{method: "DELETE", path: "/api/v1/contacts/" + bob, token: token}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/contacts/" + bob, token: token}), 404, "NOT_FOUND")
}

func TestSegmentsPreviewAndExport(t *testing.T) {
	h := newHarness(t)
	token, _ := h.setupOwner()
	mk := func(phone, name, var1 string, tags ...string) {
		body := map[string]any{"phone": phone, "name": name, "var1": var1}
		if len(tags) > 0 {
			body["tags"] = tags
		}
		h.expect(h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: body}), 201, "")
	}
	mk("+905550000001", "Ali", "Istanbul", "vip")
	mk("+905550000002", "=HYPERLINK(\"http://evil\")", "Ankara", "vip")
	mk("+905550000003", "Can", "Istanbul")
	mk("+14155550199", "Dana", "Istanbul", "vip")

	def := map[string]any{"op": "and", "rules": []any{
		map[string]any{"field": "tag", "operator": "has", "value": "VIP"},
		map[string]any{"field": "var1", "operator": "eq", "value": "istanbul"},
		map[string]any{"not": true, "field": "phone", "operator": "starts_with", "value": "+1"},
	}}
	r := h.do(call{method: "POST", path: "/api/v1/segments/preview", token: token, body: map[string]any{"definition": def}})
	h.expect(r, 200, "")
	if r.data()["count"] != float64(1) || len(r.data()["sample"].([]any)) != 1 {
		t.Fatalf("preview: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/segments/preview", token: token, body: map[string]any{
		"definition": map[string]any{"field": "password_hash", "operator": "eq", "value": "x"}}}), 422, "VALIDATION_ERROR")

	r = h.do(call{method: "POST", path: "/api/v1/segments", token: token, body: map[string]any{"name": "Istanbul VIPs", "definition": def}})
	h.expect(r, 201, "")
	seg := r.data()["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/segments", token: token, body: map[string]any{"name": "istanbul vips", "definition": def}}), 409, "SEGMENT_EXISTS")
	r = h.do(call{method: "GET", path: "/api/v1/segments/" + seg, token: token})
	if r.data()["contact_count"] != float64(1) {
		t.Fatalf("segment count: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?segment_id=" + seg, token: token})
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["name"] != "Ali" {
		t.Fatalf("list by segment: %s", r.raw)
	}
	// Segments follow data changes: they are evaluated, not snapshotted.
	mk("+905550000005", "Eda", "Istanbul", "VIP")
	if h.do(call{method: "GET", path: "/api/v1/segments/" + seg, token: token}).data()["contact_count"] != float64(2) {
		t.Fatal("segment not dynamic")
	}

	status, body, hdr := h.raw("GET", "/api/v1/contact-exports?tag_id=", token, "", nil)
	if status != 422 {
		t.Fatalf("bad filter must fail before streaming: %d %s", status, body)
	}
	status, body, hdr = h.raw("GET", "/api/v1/contact-exports", token, "", nil)
	if status != 200 || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") {
		t.Fatalf("export: %d %s", status, body)
	}
	rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil || len(rows) != 6 || rows[0][0] != "phone" {
		t.Fatalf("export rows: %v %v", rows, err)
	}
	for _, row := range rows {
		if strings.HasPrefix(row[1], "=") {
			t.Fatalf("formula not neutralised: %q", row[1])
		}
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'contact.export'") != 1 {
		t.Fatal("export not audited")
	}
}

func uploadCSV(h *harness, token, name, content string) map[string]any {
	h.t.Helper()
	status, body, _ := h.raw("POST", "/api/v1/contact-imports?filename="+name, token, "text/csv", strings.NewReader(content))
	if status != 201 {
		h.t.Fatalf("upload: %d %s", status, body)
	}
	r := resp{status: status, raw: body}
	_ = jsonUnmarshal(body, &r.body)
	return r.data()
}

func TestContactImportMappingDuplicatesAndErrors(t *testing.T) {
	h := newHarness(t)
	token, _ := h.setupOwner()
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{"phone": "+905550000001", "name": "Existing", "var2": "keep"}}), 201, "")

	// Semicolon-separated with a BOM, as exported by spreadsheet programs.
	content := "\ufeffTelefon;Ad;Şehir;Etiketler;Not\n" +
		"+90 555 000 00 01;Updated Name;Izmir;vip|gold;x\n" +
		"905550000002;Ece;Ankara;;y\n" +
		"not-a-number;Bad;X;;z\n" +
		";Missing;X;;z\n" +
		"\n" +
		"00905550000003;Fırat;Bursa;vip;w\n" +
		"+905550000002;Dup in file;Adana;;v\n"
	imp := uploadCSV(h, token, "kisiler.csv", content)
	if imp["total_rows"] != float64(6) || len(imp["headers"].([]any)) != 5 || imp["status"] != "uploaded" {
		t.Fatalf("upload result: %v", imp)
	}
	id := imp["id"].(string)

	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id + "/start", token: token, body: map[string]any{"mapping": map[string]string{"name": "Ad"}}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id + "/start", token: token, body: map[string]any{"mapping": map[string]string{"phone": "Nope"}}}), 422, "VALIDATION_ERROR")
	r := h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id + "/start", token: token, body: map[string]any{
		"mapping":          map[string]string{"phone": "Telefon", "name": "Ad", "var1": "Şehir", "tags": "Etiketler", "attr:note": "Not"},
		"duplicate_policy": "update", "add_tags": []string{"imported"}}})
	h.expect(r, 202, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id + "/start", token: token, body: map[string]any{"mapping": map[string]string{"phone": "Telefon"}}}), 409, "IMPORT_STARTED")
	if h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:contacts:import'") != 1 {
		t.Fatal("import task not scheduled")
	}

	if err := h.svc.contacts.Run(context.Background(), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contact-imports/" + id, token: token})
	d := r.data()
	if d["status"] != "completed" || d["processed_rows"] != float64(6) || d["created_count"] != float64(2) ||
		d["updated_count"] != float64(2) || d["failed_count"] != float64(2) || d["has_error_report"] != true {
		t.Fatalf("import result: %s", r.raw)
	}
	if h.count("SELECT count(*) FROM contacts WHERE phone_e164 = '+905550000001' AND name = 'Updated Name' AND var1 = 'Izmir' AND var2 = 'keep' AND attributes->>'note' = 'x'") != 1 {
		t.Fatal("update policy did not merge fields")
	}
	if h.count(`SELECT count(*) FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id JOIN contacts c ON c.id = ct.contact_id
		WHERE c.phone_e164 = '+905550000001' AND t.name IN ('vip', 'gold', 'imported')`) != 3 {
		t.Fatal("tags from file and add_tags not applied")
	}
	if h.count("SELECT count(*) FROM contacts WHERE phone_e164 = '+905550000003' AND source = 'import'") != 1 {
		t.Fatal("00-prefixed number not normalised")
	}
	status, body, _ := h.raw("GET", "/api/v1/contact-imports/"+id+"/errors", token, "", nil)
	if status != 200 || !strings.Contains(string(body), "not in international format") || !strings.Contains(string(body), "phone number is missing") {
		t.Fatalf("error report: %d %s", status, body)
	}

	// Skip policy leaves existing contacts untouched.
	imp = uploadCSV(h, token, "again.csv", "phone,name\n+905550000002,Overwritten?\n+905550000009,New\n")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + imp["id"].(string) + "/start", token: token, body: map[string]any{"mapping": map[string]string{"phone": "phone", "name": "name"}}}), 202, "")
	if err := h.svc.contacts.Run(context.Background(), uuid.MustParse(imp["id"].(string))); err != nil {
		t.Fatal(err)
	}
	d = h.do(call{method: "GET", path: "/api/v1/contact-imports/" + imp["id"].(string), token: token}).data()
	if d["created_count"] != float64(1) || d["skipped_count"] != float64(1) || h.count("SELECT count(*) FROM contacts WHERE name = 'Overwritten?'") != 0 {
		t.Fatalf("skip policy: %v", d)
	}

	// Rejected uploads.
	for name, c := range map[string]string{"x.xlsx": "phone\n1\n", "empty.csv": "phone\n", "dup.csv": "phone,Phone\n1,2\n"} {
		status, body, _ := h.raw("POST", "/api/v1/contact-imports?filename="+name, token, "text/csv", strings.NewReader(c))
		if status != 422 {
			t.Errorf("%s accepted: %d %s", name, status, body)
		}
	}
	// A cancelled queued import never runs.
	imp = uploadCSV(h, token, "c.csv", "phone\n+905551230000\n")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + imp["id"].(string) + "/start", token: token, body: map[string]any{"mapping": map[string]string{"phone": "phone"}}}), 202, "")
	r = h.do(call{method: "POST", path: "/api/v1/contact-imports/" + imp["id"].(string) + "/cancel", token: token})
	h.expect(r, 200, "")
	_ = h.svc.contacts.Run(context.Background(), uuid.MustParse(imp["id"].(string)))
	if r.data()["status"] != "cancelled" || h.count("SELECT count(*) FROM contacts WHERE phone_e164 = '+905551230000'") != 0 {
		t.Fatalf("cancel: %s", r.raw)
	}
}

// Phase 4 done criterion: a 100k-row import stays within a memory bound and
// can be cancelled while running.
func TestLargeImportBoundedMemoryAndCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("large import")
	}
	h := newHarness(t)
	token, _ := h.setupOwner()
	const n = 100_000
	var b strings.Builder
	b.WriteString("phone,name,var1,tags\n")
	for i := range n {
		fmt.Fprintf(&b, "+9055%08d,Contact %d,City %d,t%d\n", i, i, i%50, i%5)
	}
	file := b.String()

	// Full run with memory sampling.
	imp := uploadCSV(h, token, "big.csv", file)
	if imp["total_rows"] != float64(n) {
		t.Fatalf("rows: %v", imp["total_rows"])
	}
	id := uuid.MustParse(imp["id"].(string))
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id.String() + "/start", token: token,
		body: map[string]any{"mapping": map[string]string{"phone": "phone", "name": "name", "var1": "var1", "tags": "tags"}}}), 202, "")
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	stop := make(chan struct{})
	go func() {
		var m runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				runtime.ReadMemStats(&m)
				if m.HeapInuse > peak.Load() {
					peak.Store(m.HeapInuse)
				}
			}
		}
	}()
	start := time.Now()
	if err := h.svc.contacts.Run(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	close(stop)
	growth := int64(peak.Load()) - int64(base.HeapInuse)
	t.Logf("100k rows in %s, peak heap growth %d MiB", time.Since(start).Round(time.Millisecond), growth>>20)
	if growth > 64<<20 {
		t.Fatalf("import used %d MiB of heap; it must stream", growth>>20)
	}
	if got := h.count("SELECT count(*) FROM contacts"); got != n {
		t.Fatalf("contacts: %d", got)
	}
	if got := h.count("SELECT count(*) FROM contact_tags"); got != n {
		t.Fatalf("tags: %d", got)
	}

	// Cancel while running: rows already imported stay, the rest is skipped.
	imp = uploadCSV(h, token, "big2.csv", strings.ReplaceAll(file, "+9055", "+9056"))
	id = uuid.MustParse(imp["id"].(string))
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id.String() + "/start", token: token,
		body: map[string]any{"mapping": map[string]string{"phone": "phone"}}}), 202, "")
	done := make(chan error, 1)
	go func() { done <- h.svc.contacts.Run(context.Background(), id) }()
	deadline := time.Now().Add(30 * time.Second)
	for h.count("SELECT processed_rows FROM contact_imports WHERE id = $1", id) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id.String() + "/cancel", token: token}), 200, "")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	d := h.do(call{method: "GET", path: "/api/v1/contact-imports/" + id.String(), token: token}).data()
	processed := int(d["processed_rows"].(float64))
	if d["status"] != "cancelled" || processed == 0 || processed >= n {
		t.Fatalf("cancel during run: %v", d)
	}
	if got := h.count("SELECT count(*) FROM contacts WHERE phone_e164 LIKE '+9056%'"); got != processed {
		t.Fatalf("imported %d, counted %d", got, processed)
	}
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func TestOptedOutContactsGetNoTemplates(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	r := h.do(call{method: "POST", path: "/api/v1/contacts", token: token, body: map[string]any{"phone": "+" + testCustomer}})
	h.expect(r, 201, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + r.data()["id"].(string) + "/consent", token: token, body: map[string]any{"kind": "opt_out"}}), 200, "")
	tpl := map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}}
	h.expect(h.send(token, tpl), 422, "CONTACT_OPTED_OUT")
	// The customer writes in: a reply inside the window is still allowed.
	h.webhook(whatsapptestInbound(testCustomer))
	h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "Sure, here you go"}}), 202, "")
	h.expect(h.send(token, tpl), 422, "CONTACT_OPTED_OUT")
}

func TestXLSXImportAndExport(t *testing.T) {
	h := newHarness(t)
	token, _ := h.setupOwner()

	wb := excelize.NewFile()
	rows := [][]any{{"Phone", "Name", "City"}, {"+905550001111", "Gül", "Izmir"}, {905550002222, "Numeric phone", "Bursa"}, {"bad", "x", "y"}}
	for i, r := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := wb.SetSheetRow("Sheet1", cell, &r); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if _, err := wb.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	status, body, _ := h.raw("POST", "/api/v1/contact-imports?filename=list.xlsx", token,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", &buf)
	if status != 201 {
		t.Fatalf("xlsx upload: %d %s", status, body)
	}
	var up struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(body, &up)
	if up.Data["total_rows"] != float64(3) || up.Data["headers"].([]any)[0] != "Phone" {
		t.Fatalf("xlsx scan: %s", body)
	}
	id := up.Data["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/contact-imports/" + id + "/start", token: token,
		body: map[string]any{"mapping": map[string]string{"phone": "Phone", "name": "Name", "var1": "City"}}}), 202, "")
	if err := h.svc.contacts.Run(context.Background(), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	d := h.do(call{method: "GET", path: "/api/v1/contact-imports/" + id, token: token}).data()
	if d["created_count"] != float64(2) || d["failed_count"] != float64(1) {
		t.Fatalf("xlsx import: %v", d)
	}

	// Corrupt workbooks are rejected cleanly.
	status, _, _ = h.raw("POST", "/api/v1/contact-imports?filename=broken.xlsx", token, "", strings.NewReader("PK\x03\x04 not really"))
	if status != 422 {
		t.Fatalf("corrupt workbook: %d", status)
	}

	status, body, hdr := h.raw("GET", "/api/v1/contact-exports?format=xlsx", token, "", nil)
	if status != 200 || !strings.Contains(hdr.Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("xlsx export: %d %s", status, hdr.Get("Content-Type"))
	}
	out, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := out.GetRows("Sheet1")
	if len(got) != 3 || got[0][0] != "phone" || got[1][0] == "" {
		t.Fatalf("exported rows: %v", got)
	}
}
