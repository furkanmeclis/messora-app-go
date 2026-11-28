package config

import (
	"fmt"
	"net/netip"
	"strings"
)

// neverAllowed are addresses no webhook allowlist may contain: the cloud
// metadata services and loopback.
var neverAllowed = []netip.Addr{
	netip.MustParseAddr("169.254.169.254"),
	netip.MustParseAddr("169.254.170.2"),
	netip.MustParseAddr("100.100.100.200"),
	netip.MustParseAddr("fd00:ec2::254"),
	netip.MustParseAddr("127.0.0.1"),
	netip.MustParseAddr("::1"),
}

// ParseWebhookCIDRs parses WEBHOOK_ALLOWED_CIDRS: comma-separated networks
// such as "10.20.0.0/16,fd12:3456::/48". Networks wider than /8 (IPv4) or
// /32 (IPv6) and networks that contain a metadata service or loopback are
// refused.
func ParseWebhookCIDRs(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("WEBHOOK_ALLOWED_CIDRS: %q is not a network (e.g. 10.20.0.0/16)", part)
		}
		p = p.Masked()
		if (p.Addr().Is4() && p.Bits() < 8) || (p.Addr().Is6() && p.Bits() < 32) {
			return nil, fmt.Errorf("WEBHOOK_ALLOWED_CIDRS: %s is too wide", p)
		}
		for _, a := range neverAllowed {
			if p.Contains(a) {
				return nil, fmt.Errorf("WEBHOOK_ALLOWED_CIDRS: %s contains %s (metadata service or loopback)", p, a)
			}
		}
		out = append(out, p)
	}
	return out, nil
}
