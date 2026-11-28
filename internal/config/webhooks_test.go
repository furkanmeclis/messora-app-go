package config

import "testing"

func TestParseWebhookCIDRs(t *testing.T) {
	got, err := ParseWebhookCIDRs(" 10.20.0.0/16, 172.16.0.0/12 ,fd12:3456::/48,")
	if err != nil || len(got) != 3 || got[0].String() != "10.20.0.0/16" {
		t.Fatalf("%v %v", got, err)
	}
	if got, err := ParseWebhookCIDRs(""); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	for _, bad := range []string{"10.0.0.1", "0.0.0.0/0", "169.254.0.0/16", "100.64.0.0/10", "127.0.0.0/8", "::/0", "fd00::/8", "nonsense"} {
		if _, err := ParseWebhookCIDRs(bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
