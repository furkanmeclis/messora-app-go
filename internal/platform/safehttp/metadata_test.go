package safehttp

import (
	"context"
	"net/http"
	"net/netip"
	"sync/atomic"
	"testing"
)

// Every spelling of the cloud metadata services and of the server's own
// network is refused, in IPv4 and IPv6 forms (docs/SECURITY.md, outgoing
// webhooks).
func TestMetadataAndPrivateSpellings(t *testing.T) {
	c := New(Options{})
	for _, raw := range []string{
		"https://169.254.169.254/latest/meta-data/",       // AWS, GCP, Azure, DigitalOcean
		"https://169.254.170.2/v2/credentials",            // ECS task metadata
		"https://100.100.100.200/latest/meta-data/",       // Alibaba (CGNAT range)
		"https://[fd00:ec2::254]/latest/meta-data/",       // AWS IMDS over IPv6 (ULA)
		"https://[::ffff:169.254.169.254]/",               // IPv4-mapped
		"https://[::ffff:a9fe:a9fe]/",                     // IPv4-mapped, hex
		"https://[0:0:0:0:0:ffff:169.254.169.254]/",       // IPv4-mapped, long form
		"https://[64:ff9b::a9fe:a9fe]/",                   // NAT64 of 169.254.169.254
		"https://[64:ff9b::7f00:1]/",                      // NAT64 of 127.0.0.1
		"https://[::169.254.169.254]/",                    // IPv4-compatible (deprecated)
		"https://[2002:a9fe:a9fe::1]/",                    // 6to4 embedding 169.254.169.254
		"https://[2001:0:4136:e378:8000:63bf:3fff:fdd2]/", // Teredo (embeds an IPv4 address)
		"https://[fe80::a9fe:a9fe]/",                      // link-local
		"https://[fe80::1%25eth0]/",                       // scoped
		"https://2852039166/",                             // 169.254.169.254 as one decimal number
		"https://0xa9fea9fe/",                             // … as hex
		"https://0251.0376.0251.0376/",                    // … as octal
		"https://169.254.43518/",                          // … with a short form
		"https://metadata.google.internal.localhost/",     // .localhost names
		"https://localhost/",
		"https://[::]/",
		"https://0.0.0.0/",
	} {
		if _, err := c.CheckURL(raw); !IsBlocked(err) {
			// Names (and the short forms resolvers accept) are refused at
			// resolution or dial time instead; those must still block.
			if err2 := c.Check(context.Background(), raw); !IsBlocked(err2) {
				t.Errorf("%s: not refused (%v / %v)", raw, err, err2)
			}
		}
	}
	// Public addresses pass, in both families.
	for _, raw := range []string{"https://93.184.216.34/", "https://[2606:4700:4700::1111]/"} {
		if _, err := c.CheckURL(raw); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

// flipResolver answers a public address first and a private one afterwards,
// the classic DNS rebinding attack against "check, then connect".
type flipResolver struct{ n atomic.Int32 }

func (f *flipResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	if f.n.Add(1) == 1 {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
}

// The name is resolved again when the connection is made and the client
// dials the address it checked, so an answer that changes after the
// endpoint was saved never reaches the metadata service.
func TestRebindingAfterValidation(t *testing.T) {
	res := &flipResolver{}
	c := New(Options{Resolver: res})
	if err := c.Check(context.Background(), "https://rebind.test/hook"); err != nil {
		t.Fatalf("first answer is public: %v", err)
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://rebind.test/hook", nil)
	if _, err := c.Do(req); !IsBlocked(err) {
		t.Fatalf("second answer is the metadata service: %v", err)
	}
	if res.n.Load() < 2 {
		t.Fatal("the delivery did not resolve the name again")
	}
}
