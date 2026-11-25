//go:build integration

package app

import (
	"context"
	"testing"
)

// TestAudienceInsights: a group's insights count its contacts and every
// campaign that reached any of them — also one aimed at a tag — measured
// on the group's contacts only.
func TestAudienceInsights(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	ctx := context.Background()
	var group, tag, direct, other string
	if err := h.pool.QueryRow(ctx, `
		WITH g AS (INSERT INTO contact_groups (name) VALUES ('VIP') RETURNING id)
		SELECT id::text FROM g`).Scan(&group); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(ctx, `INSERT INTO tags (name) VALUES ('newsletter') RETURNING id::text`).Scan(&tag); err != nil {
		t.Fatal(err)
	}
	// Four group members (one opted out after the campaign), one outsider.
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO contacts (phone_e164, name, source, last_inbound_at) VALUES
		  ('+905550000101', 'Ada', 'manual', now() - interval '2 days'),
		  ('+905550000102', 'Bora', 'manual', NULL),
		  ('+905550000103', 'Cem', 'manual', now() - interval '20 days'),
		  ('+905550000104', 'Deniz', 'manual', NULL),
		  ('+905550000199', 'Dışarıdaki', 'manual', NULL)`, []any{}},
		{`INSERT INTO contact_group_members (group_id, contact_id)
		  SELECT $1, id FROM contacts WHERE phone_e164 IN ('+905550000101','+905550000102','+905550000103','+905550000104')`, []any{group}},
		{`INSERT INTO contact_tags (contact_id, tag_id)
		  SELECT id, $1 FROM contacts WHERE phone_e164 IN ('+905550000101','+905550000199')`, []any{tag}},
	} {
		if _, err := h.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("%v: %s", err, q.sql)
		}
	}
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language, audience, total_recipients, started_at, finished_at)
		SELECT 'Doğrudan', 'completed', id, 'order_update', 'en', jsonb_build_object('group_id', $1::text), 4, now() - interval '5 days', now() - interval '5 days'
		FROM phone_numbers LIMIT 1 RETURNING id::text`, group).Scan(&direct); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language, audience, total_recipients, started_at)
		SELECT 'Bülten', 'completed', id, 'order_update', 'en', jsonb_build_object('tag_id', $1::text), 2, now() - interval '1 day'
		FROM phone_numbers LIMIT 1 RETURNING id::text`, tag).Scan(&other); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164, state)
		  SELECT $1, id, phone_e164, CASE WHEN phone_e164 = '+905550000104' THEN 'skipped' ELSE 'queued' END
		  FROM contacts WHERE phone_e164 LIKE '+9055500001%' AND phone_e164 <> '+905550000199'`, []any{direct}},
		{`INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164, state)
		  SELECT $1, id, phone_e164, 'queued' FROM contacts WHERE phone_e164 IN ('+905550000101','+905550000199')`, []any{other}},
		{`INSERT INTO messages (phone_number_id, contact_id, direction, type, status, source, campaign_id, sent_at, delivered_at, read_at, failed_at, created_at)
		  SELECT (SELECT id FROM phone_numbers LIMIT 1), c.id, 'outbound', 'template', x.status, 'campaign', $1,
		         CASE WHEN x.status = 'failed' THEN NULL ELSE now() - interval '5 days' END, x.dl, x.rd, x.fl, now() - interval '5 days'
		  FROM contacts c JOIN (VALUES
		    ('+905550000101', 'read',      now() - interval '5 days', now() - interval '5 days', NULL::timestamptz),
		    ('+905550000102', 'delivered', now() - interval '5 days', NULL, NULL),
		    ('+905550000103', 'failed',    NULL, NULL, now() - interval '5 days')
		  ) AS x(phone, status, dl, rd, fl) ON x.phone = c.phone_e164`, []any{direct}},
		{`INSERT INTO messages (phone_number_id, contact_id, direction, wamid, type, body, status, source, occurred_at, created_at)
		  SELECT (SELECT id FROM phone_numbers LIMIT 1), id, 'inbound', 'wamid.ins-1', 'text', 'Teşekkürler', 'received', 'webhook',
		         now() - interval '4 days', now() - interval '4 days' FROM contacts WHERE phone_e164 = '+905550000101'`, []any{}},
		{`INSERT INTO messages (phone_number_id, contact_id, direction, type, status, source, campaign_id, sent_at, delivered_at, created_at)
		  SELECT (SELECT id FROM phone_numbers LIMIT 1), id, 'outbound', 'template', 'delivered', 'campaign', $1,
		         now() - interval '1 day', now() - interval '1 day', now() - interval '1 day'
		  FROM contacts WHERE phone_e164 IN ('+905550000101', '+905550000199')`, []any{other}},
		{`UPDATE contacts SET opted_out_at = now() - interval '3 days' WHERE phone_e164 = '+905550000102'`, []any{}},
	} {
		if _, err := h.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("%v: %s", err, q.sql)
		}
	}

	r := h.do(call{method: "GET", path: "/api/v1/groups/" + group + "/insights?tz=Europe/Istanbul", token: owner})
	h.expect(r, 200, "")
	d := r.data()
	sum := d["summary"].(map[string]any)
	if sum["contacts"] != float64(4) || sum["opted_out"] != float64(1) || sum["reachable"] != float64(3) ||
		sum["active_7d"] != float64(1) || sum["active_30d"] != float64(2) || sum["new_30d"] != float64(4) {
		t.Fatalf("summary: %v", sum)
	}
	msg := d["messaging"].(map[string]any)
	if msg["outbound"] != float64(4) || msg["read"] != float64(1) || msg["failed"] != float64(1) ||
		msg["inbound"] != float64(1) || msg["contacts_replied"] != float64(1) || msg["from_campaigns"] != float64(4) {
		t.Fatalf("messaging: %v", msg)
	}
	camps := d["campaigns"].([]any)
	if len(camps) != 2 {
		t.Fatalf("campaigns: %v", camps)
	}
	byName := map[string]map[string]any{}
	for _, c := range camps {
		m := c.(map[string]any)
		byName[m["name"].(string)] = m
	}
	dc, nl := byName["Doğrudan"], byName["Bülten"]
	if dc["targeted"] != true || dc["reached"] != float64(4) || dc["skipped"] != float64(1) || dc["sent"] != float64(2) ||
		dc["delivered"] != float64(2) || dc["read"] != float64(1) || dc["failed"] != float64(1) || dc["replied"] != float64(1) ||
		dc["opted_out_after"] != float64(1) {
		t.Fatalf("direct campaign: %v", dc)
	}
	// Aimed at a tag, measured on the one group member it reached.
	if nl["targeted"] != false || nl["reached"] != float64(1) || nl["sent"] != float64(1) || nl["total_recipients"] != float64(2) {
		t.Fatalf("tag campaign: %v", nl)
	}
	if days := d["daily"].([]any); len(days) != 30 {
		t.Fatalf("daily: %d days", len(days))
	}
	if eng := d["engaged"].([]any); len(eng) != 1 || eng[0].(map[string]any)["name"] != "Ada" {
		t.Fatalf("engaged: %v", eng)
	}

	// The tag sees the newsletter as targeted, the direct campaign through Ada.
	r = h.do(call{method: "GET", path: "/api/v1/tags/" + tag + "/insights", token: owner})
	h.expect(r, 200, "")
	if n := len(r.data()["campaigns"].([]any)); n != 2 {
		t.Fatalf("tag campaigns: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/groups/00000000-0000-7000-8000-000000000000/insights", token: owner}), 404, "NOT_FOUND")
}
