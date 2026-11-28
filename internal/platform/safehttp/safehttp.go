// Package safehttp is an HTTP client for URLs that users configure (outgoing
// webhooks, the automation HTTP action). It refuses to reach the server's own
// network: loopback, private (RFC 1918, IPv6 ULA), link-local (including the
// 169.254.169.254 cloud metadata service), CGNAT, multicast, reserved and
// documentation ranges, and IPv4-mapped or NAT64 forms of them.
//
// The destination address is checked where it matters, when the socket is
// dialled: names are resolved by the client, every resolved address is
// checked, the connection goes to a checked address, and the dialer's Control
// hook checks the final socket address once more. A DNS answer that changes
// between validation and use (DNS rebinding) therefore cannot smuggle in a
// private address. Environment proxies are ignored, redirects are not
// followed unless asked for (and then each hop is checked again), and
// response bodies are read up to a limit.
package safehttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Defaults for Options.
const (
	DefaultTimeout          = 10 * time.Second
	DefaultMaxResponseBytes = 64 << 10
)

// Resolver looks up the addresses of a host name. *net.Resolver satisfies it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Options configures a Client. The zero value is the safe production setup:
// https only, public addresses only, no redirects, 10 s timeout, 64 KiB of
// response body.
type Options struct {
	// Timeout bounds the whole request including reading the body.
	Timeout time.Duration
	// MaxResponseBytes is how much of a response body is read; the rest is
	// discarded and Response.Truncated is set.
	MaxResponseBytes int64
	// MaxRedirects is how many redirects are followed (each hop is checked
	// like the first request). 0 returns the 3xx response as is.
	MaxRedirects int
	// AllowHTTP permits plain http:// URLs. For development only: payloads
	// and signatures travel unencrypted.
	AllowHTTP bool
	// AllowedPrefixes are networks that may be reached although they are
	// private, e.g. an operator's explicit allowlist of internal receivers.
	AllowedPrefixes []netip.Prefix
	// AllowPrivate turns the address check off. It exists for tests whose
	// receivers listen on 127.0.0.1 and is honoured only inside a test
	// binary: New panics when it is set anywhere else, so no configuration
	// value can switch the protection off.
	AllowPrivate bool
	// Resolver replaces the system resolver (tests).
	Resolver Resolver
	// UserAgent is sent unless the request sets its own.
	UserAgent string
}

// Client sends requests to user-supplied URLs.
type Client struct {
	o    Options
	http *http.Client
}

// New returns a client.
func New(o Options) *Client {
	if o.AllowPrivate && !testing.Testing() {
		panic("safehttp: AllowPrivate is only available in tests")
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.MaxResponseBytes <= 0 {
		o.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if o.Resolver == nil {
		o.Resolver = net.DefaultResolver
	}
	c := &Client{o: o}
	dialer := &net.Dialer{Timeout: o.Timeout, KeepAlive: 30 * time.Second, Control: c.control}
	tr := &http.Transport{
		Proxy: nil, // never route through HTTP(S)_PROXY: the proxy would reach the address for us
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.dial(ctx, dialer, network, addr)
		},
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           50,
		MaxIdleConnsPerHost:    4,
		IdleConnTimeout:        60 * time.Second,
		TLSHandshakeTimeout:    o.Timeout,
		ResponseHeaderTimeout:  o.Timeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
	c.http = &http.Client{
		Transport: tr,
		Timeout:   o.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > o.MaxRedirects {
				return http.ErrUseLastResponse
			}
			_, err := c.CheckURL(req.URL.String())
			return err
		},
	}
	return c
}

// HTTPClient returns the guarded client for libraries that need an
// *http.Client (e.g. provider SDKs). Every connection is checked when it is
// dialled; the response size limit of Do does not apply.
func (c *Client) HTTPClient() *http.Client { return c.http }

// BlockedError reports a destination the client refused to contact.
type BlockedError struct {
	Host   string
	Addr   netip.Addr // invalid when the URL itself was refused
	Reason string
}

func (e *BlockedError) Error() string {
	if e.Addr.IsValid() {
		return fmt.Sprintf("safehttp: %s (%s) is not allowed: %s", e.Host, e.Addr, e.Reason)
	}
	return fmt.Sprintf("safehttp: %s is not allowed: %s", e.Host, e.Reason)
}

// IsBlocked reports whether err (or an error it wraps) is a *BlockedError.
func IsBlocked(err error) bool {
	var b *BlockedError
	return errors.As(err, &b)
}

// CheckURL validates a URL without network access: an absolute https URL
// (http too with AllowHTTP), a host, no ambiguous numeric host forms such as
// "2130706433", "0x7f.1" or "0177.0.0.1", no "localhost", and, for IP
// literals, an allowed address. Names are checked when they are resolved.
func (c *Client) CheckURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, &BlockedError{Reason: "invalid URL"}
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && c.o.AllowHTTP:
	default:
		return nil, &BlockedError{Host: u.Hostname(), Reason: "only https URLs are allowed"}
	}
	host := u.Hostname()
	if host == "" || u.Opaque != "" {
		return nil, &BlockedError{Host: host, Reason: "URL has no host"}
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil, &BlockedError{Host: host, Reason: "invalid port"}
		}
	}
	if a, err := netip.ParseAddr(host); err == nil {
		if reason := c.allowed(a); reason != "" {
			return nil, &BlockedError{Host: host, Addr: a.Unmap(), Reason: reason}
		}
		return u, nil
	}
	if c.o.AllowPrivate {
		return u, nil
	}
	if looksNumeric(host) {
		return nil, &BlockedError{Host: host, Reason: "ambiguous numeric host; write the address in dotted-decimal form"}
	}
	lh := strings.ToLower(strings.TrimSuffix(host, "."))
	if lh == "localhost" || strings.HasSuffix(lh, ".localhost") {
		return nil, &BlockedError{Host: host, Reason: "loopback host"}
	}
	return u, nil
}

// looksNumeric reports hosts made only of digits, dots and hex notation that
// some resolvers (getaddrinfo, browsers) read as IPv4 addresses.
func looksNumeric(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" {
		return false
	}
	for _, part := range strings.Split(h, ".") {
		if part == "" {
			return false
		}
		p := strings.TrimPrefix(part, "0x")
		if p == "" {
			continue // "0x" alone
		}
		for _, r := range p {
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
			if !isHex {
				return false
			}
			if p == part && (r < '0' || r > '9') {
				return false // hex letters without the 0x prefix make a name ("cafe")
			}
		}
	}
	return true
}

// Blocked networks beyond what netip's predicates cover.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),   // CGNAT
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved, broadcast
	netip.MustParsePrefix("100::/64"),        // discard
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
	netip.MustParsePrefix("2001::/32"),       // Teredo (embeds IPv4, reached through relays)
	netip.MustParsePrefix("2002::/16"),       // 6to4 (embeds IPv4)
	netip.MustParsePrefix("fec0::/10"),       // site-local (deprecated)
	netip.MustParsePrefix("::/96"),           // IPv4-compatible (deprecated)
}

var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// disallowed returns why a is not a public unicast address, or "".
func disallowed(a netip.Addr) string {
	a = a.Unmap()
	if a.Zone() != "" {
		return "scoped address"
	}
	if a.Is6() && nat64.Contains(a) {
		b := a.As16()
		return disallowed(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	}
	switch {
	case a.IsUnspecified():
		return "unspecified address"
	case a.IsLoopback():
		return "loopback address"
	case a.IsPrivate():
		return "private address"
	case a.IsLinkLocalUnicast(), a.IsLinkLocalMulticast():
		return "link-local address"
	case a.IsMulticast(), a.IsInterfaceLocalMulticast():
		return "multicast address"
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return "reserved address"
		}
	}
	if !a.IsGlobalUnicast() {
		return "not a public unicast address"
	}
	return ""
}

// allowed applies the options to a resolved address.
func (c *Client) allowed(a netip.Addr) string {
	if c.o.AllowPrivate {
		return ""
	}
	a = a.Unmap()
	for _, p := range c.o.AllowedPrefixes {
		if p.Contains(a) {
			return ""
		}
	}
	return disallowed(a)
}

// dial resolves addr itself and connects only to permitted addresses.
func (c *Client) dial(ctx context.Context, d *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := c.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// resolve returns the addresses of host, refusing the lookup if any of them
// is not allowed (a name with one private address is treated as hostile).
func (c *Client) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if a, err := netip.ParseAddr(host); err == nil {
		if reason := c.allowed(a); reason != "" {
			return nil, &BlockedError{Host: host, Addr: a, Reason: reason}
		}
		return []netip.Addr{a.Unmap()}, nil
	}
	if !c.o.AllowPrivate && looksNumeric(host) {
		return nil, &BlockedError{Host: host, Reason: "ambiguous numeric host"}
	}
	ips, err := c.o.Resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("safehttp: resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("safehttp: resolve %s: no addresses", host)
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if reason := c.allowed(ip); reason != "" {
			return nil, &BlockedError{Host: host, Addr: ip.Unmap(), Reason: reason}
		}
		out = append(out, ip.Unmap())
	}
	return out, nil
}

// control re-checks the socket address right before connect(2).
func (c *Client) control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return &BlockedError{Host: address, Reason: "unparseable socket address"}
	}
	if reason := c.allowed(ap.Addr()); reason != "" {
		return &BlockedError{Host: address, Addr: ap.Addr().Unmap(), Reason: reason}
	}
	return nil
}

// Check validates rawURL and resolves its host, so a destination that would
// be refused at delivery time can be rejected when it is configured. Every
// request is checked again: DNS answers change.
func (c *Client) Check(ctx context.Context, rawURL string) error {
	u, err := c.CheckURL(rawURL)
	if err != nil {
		return err
	}
	_, err = c.resolve(ctx, u.Hostname())
	return err
}

// Response is a completed exchange with a bounded body.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	// Truncated is set when the body was longer than MaxResponseBytes.
	Truncated bool
	Duration  time.Duration
}

// Do sends req after validating its URL. Errors never contain URL
// credentials. A redirect that is not followed is returned as a response.
func (c *Client) Do(req *http.Request) (*Response, error) {
	if _, err := c.CheckURL(req.URL.String()); err != nil {
		return nil, err
	}
	if c.o.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.o.UserAgent)
	}
	start := time.Now()
	res, err := c.http.Do(req) //nolint:gosec // G704: the destination is validated above and at dial time
	if err != nil {
		return nil, redactErr(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, c.o.MaxResponseBytes+1))
	out := &Response{StatusCode: res.StatusCode, Header: res.Header, Duration: time.Since(start)}
	if int64(len(body)) > c.o.MaxResponseBytes {
		body, out.Truncated = body[:c.o.MaxResponseBytes], true
	}
	out.Body = body
	if err != nil {
		return out, redactErr(err)
	}
	return out, nil
}

// redactErr strips credentials from URLs inside *url.Error.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		if u, perr := url.Parse(ue.URL); perr == nil {
			return &url.Error{Op: ue.Op, URL: Redact(u), Err: ue.Err}
		}
	}
	return err
}

// Redact renders u for logs: no user info, no query string, no fragment
// (query parameters often carry tokens).
func Redact(u *url.URL) string {
	if u == nil {
		return ""
	}
	c := *u
	c.User = nil
	c.RawQuery, c.ForceQuery = "", false
	c.Fragment, c.RawFragment = "", ""
	return c.String()
}
