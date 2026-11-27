//go:build integration

package app

import (
	"fmt"
	"testing"
)

func TestContactPagesAndBulkActions(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	names := []string{"Zeynep", "ahmet", "Mehmet", "Ayşe", "Burak", "Can"}
	ids := map[string]string{}
	for i, n := range names {
		r := h.do(call{method: "POST", path: "/api/v1/contacts", token: owner, body: map[string]any{
			"phone": fmt.Sprintf("90555000%04d", i), "name": n}})
		h.expect(r, 201, "")
		ids[n] = r.data()["id"].(string)
	}

	// Numbered pages, sorting and the total.
	r := h.do(call{method: "GET", path: "/api/v1/contacts?sort=name_asc&page=1&limit=4", token: owner})
	h.expect(r, 200, "")
	if tot := r.body["meta"].(map[string]any)["total"]; tot != float64(6) {
		t.Fatalf("total: %v", tot)
	}
	if l := r.list(); len(l) != 4 || l[0].(map[string]any)["name"] != "ahmet" || l[1].(map[string]any)["name"] != "Ayşe" {
		t.Fatalf("page 1 by name: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?sort=name_asc&page=2&limit=4", token: owner})
	if l := r.list(); len(l) != 2 || l[1].(map[string]any)["name"] != "Zeynep" {
		t.Fatalf("page 2 by name: %s", r.raw)
	}

	// Tag selected ids; the tag is created on the fly.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{
		"action": "add_tag", "tag_name": "VIP", "ids": []string{ids["Zeynep"], ids["Mehmet"], ids["Can"]}}})
	h.expect(r, 200, "")
	if r.data()["matched"] != float64(3) || r.data()["affected"] != float64(3) {
		t.Fatalf("add_tag: %s", r.raw)
	}
	// Again: nothing changes.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{
		"action": "add_tag", "tag_name": "vip", "ids": []string{ids["Zeynep"]}}})
	if r.data()["affected"] != float64(0) {
		t.Fatalf("re-tag: %s", r.raw)
	}
	var tagID string
	if err := h.pool.QueryRow(h.ctx(), "SELECT id::text FROM tags WHERE lower(name)='vip'").Scan(&tagID); err != nil {
		t.Fatal(err)
	}

	// By filter, minus one: everyone tagged VIP except Can opts out.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{
		"action": "opt_out", "filter": map[string]any{"tag_id": tagID}, "exclude_ids": []string{ids["Can"]}}})
	h.expect(r, 200, "")
	if r.data()["matched"] != float64(2) || r.data()["affected"] != float64(2) {
		t.Fatalf("opt_out: %s", r.raw)
	}
	var events int
	_ = h.pool.QueryRow(h.ctx(), "SELECT count(*) FROM consent_events WHERE kind='opt_out' AND note='bulk action'").Scan(&events)
	if events != 2 {
		t.Fatalf("consent events: %d", events)
	}

	// Groups.
	r = h.do(call{method: "POST", path: "/api/v1/groups", token: owner, body: map[string]any{"name": "Bulk group"}})
	h.expect(r, 201, "")
	group := r.data()["id"].(string)
	r = h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{
		"action": "add_to_group", "group_id": group, "filter": map[string]any{"opted_out": false}}})
	if r.data()["affected"] != float64(4) {
		t.Fatalf("add_to_group: %s", r.raw)
	}

	// Validation and permissions.
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{"action": "remove_tag", "ids": []string{ids["Can"]}}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{"action": "delete"}}), 422, "VALIDATION_ERROR")
	agent, _ := h.invite(owner, "agent@example.com", "agent")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: agent, body: map[string]any{"action": "delete", "ids": []string{ids["Can"]}}}), 403, "")

	// Delete what matches a search.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/bulk", token: owner, body: map[string]any{
		"action": "delete", "filter": map[string]any{"q": "e"}}})
	h.expect(r, 200, "")
	if r.data()["affected"] != float64(4) {
		t.Fatalf("delete: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?page=1&sort=name_asc", token: owner})
	if l := r.list(); len(l) != 2 || l[0].(map[string]any)["name"] != "Burak" || l[1].(map[string]any)["name"] != "Can" {
		t.Fatalf("left after delete: %s", r.raw)
	}
	var audits int
	_ = h.pool.QueryRow(h.ctx(), "SELECT count(*) FROM audit_logs WHERE action LIKE 'contact.bulk_%'").Scan(&audits)
	if audits < 5 {
		t.Fatalf("audit entries: %d", audits)
	}
}
