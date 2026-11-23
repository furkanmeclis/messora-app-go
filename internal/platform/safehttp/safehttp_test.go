package safehttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeResolver answers from a fixed table.
type fakeResolver map[string][]string

func (f fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	ips, ok := f[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	out := make([]netip.Addr, len(ips))
	for i, s := range ips {
		out[i] = netip.MustParseAddr(s)
	}
	return out, nil
}

func TestCheckURL(t *testing.T) {
	strict := New(Options{})
	devHTTP := New(Options{AllowHTTP: true})
	cases := []struct {
		url     string
		c       *Client
		blocked bool
	}{
		{"https://example.com/hook", strict, false},
		{"https://example.com:8443/hook?x=1", strict, false},
		{"https://8.8.8.8/", strict, false},
		{"https://[2606:4700:4700::1111]/", strict, false},
		{"https://[64:ff9b::808:808]/", strict, false}, // NAT64 of 8.8.8.8
		{"http://example.com/", strict, true},
		{"http://example.com/", devHTTP, false},
		{"ftp://example.com/", devHTTP, true},
		{"file:///etc/passwd", strict, true},
		{"https:///nohost", strict, true},
		{"https://example.com:0/", strict, true},
		{"https://example.com:99999/", strict, true},
		{"not a url", strict, true},
		{"https://127.0.0.1/", strict, true},
		{"https://127.255.0.1/", strict, true},
		{"https://10.1.2.3/", strict, true},
		{"https://172.16.0.1/", strict, true},
		{"https://192.168.1.1/", strict, true},
		{"https://100.64.0.1/", strict, true},            // CGNAT
		{"https://169.254.169.254/latest", strict, true}, // cloud metadata
		{"https://0.0.0.0/", strict, true},
		{"https://224.0.0.1/", strict, true},
		{"https://255.255.255.255/", strict, true},
		{"https://192.0.2.10/", strict, true},
		{"https://[::1]/", strict, true},
		{"https://[::]/", strict, true},
		{"https://[fd00::1]/", strict, true},
		{"https://[fe80::1%25en0]/", strict, true},
		{"https://[ff02::1]/", strict, true},
		{"https://[::ffff:127.0.0.1]/", strict, true},
		{"https://[::ffff:169.254.169.254]/", strict, true},
		{"https://[64:ff9b::7f00:1]/", strict, true}, // NAT64 of 127.0.0.1
		{"https://[2002:7f00:1::]/", strict, true},   // 6to4
		{"https://2130706433/", strict, true},        // decimal 127.0.0.1
		{"https://0x7f000001/", strict, true},        // hex
		{"https://0177.0.0.1/", strict, true},        // octal
		{"https://0x7f.0.0.1/", strict, true},
		{"https://127.1/", strict, true},
		{"https://localhost/", strict, true},
		{"https://LOCALHOST./", strict, true},
		{"https://api.localhost/", strict, true},
		{"https://cafe.example/", strict, false}, // hex-looking labels are still names
		{"https://123.example/", strict, false},
	}
	for _, tc := range cases {
		_, err := tc.c.CheckURL(tc.url)
		if tc.blocked != (err != nil) {
			t.Errorf("%s: blocked=%v, err=%v", tc.url, tc.blocked, err)
		}
		if err != nil && !IsBlocked(err) {
			t.Errorf("%s: error is not a BlockedError: %v", tc.url, err)
		}
	}
}

func TestDisallowedRanges(t *testing.T) {
	for addr, want := range map[string]bool{
		"1.1.1.1": false, "93.184.216.34": false, "2a00:1450:4001::1": false,
		"127.0.0.1": true, "10.0.0.1": true, "172.31.255.255": true, "192.168.0.1": true, "100.127.255.255": true,
		"169.254.0.1": true, "198.18.0.1": true, "203.0.113.5": true, "240.0.0.1": true, "::ffff:10.0.0.1": true,
		"fc00::1": true, "fe80::1": true, "fec0::1": true, "2001:db8::1": true, "ff05::1": true, "::": true, "::1": true,
	} {
		if got := disallowed(netip.MustParseAddr(addr)) != ""; got != want {
			t.Errorf("%s: blocked=%v, want %v", addr, got, want)
		}
	}
	c := New(Options{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16")}})
	if c.allowed(netip.MustParseAddr("10.20.1.1")) != "" || c.allowed(netip.MustParseAddr("10.21.1.1")) == "" {
		t.Fatal("allowlist must permit exactly its prefixes")
	}
}

func TestCheckResolvesNames(t *testing.T) {
	c := New(Options{Resolver: fakeResolver{
		"public.test":   {"93.184.216.34"},
		"mixed.test":    {"93.184.216.34", "10.0.0.7"},
		"metadata.test": {"169.254.169.254"},
		"mapped.test":   {"::ffff:127.0.0.1"},
		"v6.test":       {"2606:4700::1111"},
	}})
	ctx := context.Background()
	for host, blocked := range map[string]bool{
		"public.test": false, "v6.test": false, "mixed.test": true, "metadata.test": true, "mapped.test": true,
	} {
		err := c.Check(ctx, "https://"+host+"/hook")
		if blocked != IsBlocked(err) || (!blocked && err != nil) {
			t.Errorf("%s: blocked=%v, err=%v", host, blocked, err)
		}
	}
	if err := c.Check(ctx, "https://nx.test/"); err == nil || IsBlocked(err) {
		t.Fatalf("unknown host must fail resolution, got %v", err)
	}
}

// loopbackServer counts requests that actually arrived.
func loopbackServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *atomic.Int32, string) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return srv, &hits, u.Port()
}

func get(t *testing.T, c *Client, raw string) (*Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c.Do(req)
}

// A name that resolves to a private address (DNS rebinding) never gets a
// connection, even though the URL itself looks harmless.
func TestRebindingIsRefusedAtDialTime(t *testing.T) {
	_, hits, port := loopbackServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	c := New(Options{AllowHTTP: true, Resolver: fakeResolver{"rebind.test": {"127.0.0.1"}}})
	if _, err := c.CheckURL("http://rebind.test:" + port + "/"); err != nil {
		t.Fatalf("static check must pass for a name: %v", err)
	}
	_, err := get(t, c, "http://rebind.test:"+port+"/")
	if !IsBlocked(err) {
		t.Fatalf("want a blocked error, got %v", err)
	}
	var b *BlockedError
	if !errors.As(err, &b) || b.Addr != netip.MustParseAddr("127.0.0.1") {
		t.Fatalf("error should name the refused address: %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the private server was reached")
	}
	// The socket-level hook refuses on its own as well.
	if err := c.control("tcp4", "127.0.0.1:"+port, nil); !IsBlocked(err) {
		t.Fatalf("control hook: %v", err)
	}
	if err := c.control("tcp6", "[::ffff:10.0.0.1]:443", nil); !IsBlocked(err) {
		t.Fatalf("control hook, mapped: %v", err)
	}
	if err := c.control("tcp4", "93.184.216.34:443", nil); err != nil {
		t.Fatalf("control hook, public: %v", err)
	}
}

func TestRedirects(t *testing.T) {
	_, hits, port := loopbackServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-metadata":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
		case "/to-name":
			http.Redirect(w, r, "http://internal.test:"+r.Host[strings.LastIndex(r.Host, ":")+1:]+"/final", http.StatusFound)
		case "/to-self":
			http.Redirect(w, r, "/final", http.StatusFound)
		default:
			_, _ = io.WriteString(w, "final")
		}
	})
	allow := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	res := fakeResolver{"receiver.test": {"127.0.0.1"}, "internal.test": {"10.0.0.5"}}
	base := "http://receiver.test:" + port

	noFollow := New(Options{AllowHTTP: true, AllowedPrefixes: allow, Resolver: res})
	r, err := get(t, noFollow, base+"/to-self")
	if err != nil || r.StatusCode != http.StatusFound {
		t.Fatalf("redirects are not followed by default: %v %+v", err, r)
	}

	follow := New(Options{AllowHTTP: true, AllowedPrefixes: allow, Resolver: res, MaxRedirects: 3})
	r, err = get(t, follow, base+"/to-self")
	if err != nil || r.StatusCode != 200 || string(r.Body) != "final" {
		t.Fatalf("allowed redirect: %v %+v", err, r)
	}
	before := hits.Load()
	if _, err := get(t, follow, base+"/to-metadata"); !IsBlocked(err) {
		t.Fatalf("redirect to the metadata service: %v", err)
	}
	if _, err := get(t, follow, base+"/to-name"); !IsBlocked(err) {
		t.Fatalf("redirect to a name resolving privately: %v", err)
	}
	if hits.Load()-before != 2 {
		t.Fatalf("only the first hops may reach the server, got %d", hits.Load()-before)
	}
}

func TestBoundedBodyAndTimeout(t *testing.T) {
	_, _, port := loopbackServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(500 * time.Millisecond)
		}
		_, _ = io.WriteString(w, strings.Repeat("x", 5000))
	})
	c := New(Options{AllowPrivate: true, AllowHTTP: true, MaxResponseBytes: 100, Timeout: 200 * time.Millisecond, UserAgent: "test-agent"})
	r, err := get(t, c, "http://127.0.0.1:"+port+"/")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Body) != 100 || !r.Truncated || r.Duration <= 0 {
		t.Fatalf("body %d truncated=%v duration=%v", len(r.Body), r.Truncated, r.Duration)
	}
	if _, err := get(t, c, "http://127.0.0.1:"+port+"/slow"); err == nil {
		t.Fatal("slow server must time out")
	}
}

func TestErrorsDoNotLeakCredentials(t *testing.T) {
	c := New(Options{Resolver: fakeResolver{}})
	_, err := get(t, c, "https://user:s3cr3t-value@nx.test/hook?token=abc") // gitleaks:allow (test fixture)
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), "s3cr3t") || strings.Contains(err.Error(), "token=abc") {
		t.Fatalf("credentials leaked: %v", err)
	}
	u, _ := url.Parse("https://user:pw@example.com/p?token=x#frag") // gitleaks:allow (test fixture)
	if got := Redact(u); got != "https://example.com/p" {
		t.Fatalf("redact: %s", got)
	}
}
