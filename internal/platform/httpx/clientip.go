package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// ClientMeta records the client IP, user agent and request ID in the request
// context.
//
// The client IP is taken from the right-most X-Forwarded-For entry only when
// the direct peer is on a private network (the Caddy edge or the panel BFF,
// both of which overwrite rather than append client-supplied values).
// Otherwise the direct peer address is used, so the header cannot be spoofed
// by callers that reach the API directly.
func ClientMeta(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.UserAgent()
		if len(ua) > 512 {
			ua = ua[:512]
		}
		m := principal.RequestMeta{RequestID: RequestIDFrom(r.Context()), IP: clientIP(r), UserAgent: ua}
		next.ServeHTTP(w, r.WithContext(principal.WithMeta(r.Context(), m)))
	})
}

func clientIP(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	peer = peer.Unmap()
	if !peer.IsPrivate() && !peer.IsLoopback() {
		return peer
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return peer
	}
	parts := strings.Split(xff, ",")
	if ip, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
		return ip.Unmap()
	}
	return peer
}
