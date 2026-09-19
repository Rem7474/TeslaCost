package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ParseTrustedProxies turns CIDR ranges or bare addresses into prefixes. "none" (alone) trusts no proxy.
func ParseTrustedProxies(entries []string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" || strings.EqualFold(entry, "none") {
			continue
		}
		if p, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("%q is neither an address nor a CIDR range", entry)
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
	}
	return prefixes, nil
}

func inPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	addr = addr.Unmap()
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// peerAddr is the address of the direct TCP peer.
func peerAddr(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// clientFromForwardedFor walks X-Forwarded-For from the right, where the proxies closest to us append the
// address they saw, and returns the first address that is not a trusted proxy. Entries further left are
// supplied by the client and are never believed: reading the first entry, as many middlewares do, lets any
// client pick the address it is seen from. An unparseable entry stops the walk (no client address).
func clientFromForwardedFor(headers []string, trusted []netip.Prefix) (netip.Addr, bool) {
	for h := len(headers) - 1; h >= 0; h-- {
		entries := strings.Split(headers[h], ",")
		for i := len(entries) - 1; i >= 0; i-- {
			entry := strings.TrimSpace(entries[i])
			if entry == "" {
				continue
			}
			addr, err := netip.ParseAddr(entry)
			if err != nil {
				return netip.Addr{}, false
			}
			addr = addr.Unmap()
			if !inPrefixes(addr, trusted) {
				return addr, true
			}
		}
	}
	return netip.Addr{}, false
}

// ClientIP makes r.RemoteAddr the address of the real client. Forwarding headers are honoured only when the
// direct peer is a trusted proxy; anyone else connecting to the port is identified by their own address, so
// they cannot dodge rate limits by sending X-Forwarded-For. The same trust decides whether X-Forwarded-Proto is
// believed, see IsHTTPS.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secure := r.TLS != nil
			if peer, ok := peerAddr(r.RemoteAddr); ok {
				client := peer
				if inPrefixes(peer, trusted) {
					if found, ok := clientFromForwardedFor(r.Header.Values("X-Forwarded-For"), trusted); ok {
						client = found
					}
					if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
						secure = true
					}
				}
				r.RemoteAddr = client.String()
			}
			ctx := context.WithValue(r.Context(), clientIPKey{}, secure)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// IsHTTPS reports whether the client reached us over TLS, directly or through a trusted proxy.
func IsHTTPS(r *http.Request) bool {
	secure, _ := r.Context().Value(clientIPKey{}).(bool)
	return secure || r.TLS != nil
}
