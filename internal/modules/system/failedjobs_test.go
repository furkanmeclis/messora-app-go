package system

import (
	"maps"
	"strings"
	"testing"
)

func TestPayloadRefsKeepOnlyIdentifiers(t *testing.T) {
	got := payloadRefs([]byte(`{"message_id":"m1","campaign_id":"c1","gen":3,"run_id":7,"body":"secret text",
		"token":"abc","contact":{"phone":"+90"},"import_id":"","id":"x"}`))
	want := map[string]string{"message_id": "m1", "campaign_id": "c1", "run_id": "7", "id": "x"}
	if !maps.Equal(got, want) {
		t.Fatalf("refs = %v, want %v", got, want)
	}
	if len(payloadRefs([]byte("not json"))) != 0 || len(payloadRefs([]byte(`["a"]`))) != 0 {
		t.Fatal("non-object payloads expose nothing")
	}
}

func TestRedactError(t *testing.T) {
	for _, tc := range []struct{ in, leak string }{
		{`Post "https://graph.facebook.com/v26.0/123/messages?access_token=EAAB123": timeout`, "EAAB123"},
		{`auth failed: Authorization: Bearer sk_live_abc.def`, "sk_live_abc.def"},
		{`dial redis://user:hunter2@redis:6379/0: refused`, "hunter2"},
		{`{"client_secret":"s3cr3t","x":1}`, "s3cr3t"},
		{`password=pw123 rejected`, "pw123"},
	} {
		out := redactError(tc.in)
		if strings.Contains(out, tc.leak) {
			t.Errorf("%q leaked in %q", tc.leak, out)
		}
		if !strings.Contains(out, "[redacted]") {
			t.Errorf("no redaction marker in %q", out)
		}
	}
	if got := redactError("plain failure"); got != "plain failure" {
		t.Fatalf("plain error changed: %q", got)
	}
	if got := redactError(strings.Repeat("ş", 800)); len(got) > 1010 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long error not truncated: %d bytes", len(got))
	}
}
