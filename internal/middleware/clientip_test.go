package middleware

import (
	"github.com/teslacost/teslacost/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func runClientIP(t *testing.T, trusted []string, remote string, headers map[string]string) (clientIP string, https bool) {
	t.Helper()
	prefixes, err := ParseTrustedProxies(trusted)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	var secure bool
	h := ClientIP(prefixes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, secure = r.RemoteAddr, IsHTTPS(r)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got, secure
}

func TestClientIPIgnoresForwardingHeadersFromUntrustedPeers(t *testing.T) {
	// Someone reaching the port directly cannot choose the address they are rate limited under.
	got, https := runClientIP(t, config.DefaultTrustedProxies, "203.0.113.9:51000",
		map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8", "X-Forwarded-Proto": "https"})
	if got != "203.0.113.9" {
		t.Errorf("got %q, want the direct peer 203.0.113.9", got)
	}
	if https {
		t.Error("X-Forwarded-Proto must not be believed from an untrusted peer")
	}
}

func TestClientIPTakesTheAddressAddedByTheTrustedProxy(t *testing.T) {
	// The proxy appends the address it saw; whatever the client put on the left is not believed.
	got, https := runClientIP(t, config.DefaultTrustedProxies, "172.18.0.2:40000",
		map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.7", "X-Forwarded-Proto": "https"})
	if got != "198.51.100.7" {
		t.Errorf("got %q, want 198.51.100.7 (rightmost untrusted entry), not the client-supplied 6.6.6.6", got)
	}
	if !https {
		t.Error("X-Forwarded-Proto from a trusted proxy must be honoured")
	}
}

func TestClientIPSkipsChainedTrustedProxies(t *testing.T) {
	got, _ := runClientIP(t, config.DefaultTrustedProxies, "10.0.0.5:1234",
		map[string]string{"X-Forwarded-For": "198.51.100.7, 10.0.0.9, 172.16.3.4"})
	if got != "198.51.100.7" {
		t.Errorf("got %q, want 198.51.100.7", got)
	}
}

func TestClientIPFallsBackToThePeerWhenTheChainIsUnusable(t *testing.T) {
	for name, header := range map[string]string{
		"missing":              "",
		"garbage":              "not-an-ip",
		"only trusted proxies": "10.0.0.9, 172.16.3.4",
	} {
		headers := map[string]string{}
		if header != "" {
			headers["X-Forwarded-For"] = header
		}
		if got, _ := runClientIP(t, config.DefaultTrustedProxies, "192.168.1.20:5555", headers); got != "192.168.1.20" {
			t.Errorf("%s: got %q, want the proxy address 192.168.1.20", name, got)
		}
	}
}

func TestClientIPWithNoTrustedProxy(t *testing.T) {
	got, _ := runClientIP(t, []string{"none"}, "10.0.0.5:1234", map[string]string{"X-Forwarded-For": "198.51.100.7"})
	if got != "10.0.0.5" {
		t.Errorf("got %q, want the peer address when no proxy is trusted", got)
	}
}

func TestClientIPIPv6AndMappedAddresses(t *testing.T) {
	got, _ := runClientIP(t, []string{"::1", "10.0.0.0/8"}, "[::1]:9000", map[string]string{"X-Forwarded-For": "2001:db8::7"})
	if got != "2001:db8::7" {
		t.Errorf("IPv6: got %q", got)
	}
	// An IPv4-mapped form of a trusted address must not alias its way past the trust check.
	got, _ = runClientIP(t, []string{"10.0.0.0/8"}, "10.0.0.5:1", map[string]string{"X-Forwarded-For": "198.51.100.7, ::ffff:10.0.0.9"})
	if got != "198.51.100.7" {
		t.Errorf("mapped address: got %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	p, err := ParseTrustedProxies([]string{" 192.168.1.10 ", "10.1.2.3/8", "", "none"})
	if err != nil || len(p) != 2 {
		t.Fatalf("got %v, %v", p, err)
	}
	if p[0].String() != "192.168.1.10/32" || p[1].String() != "10.0.0.0/8" {
		t.Errorf("got %v", p)
	}
	if _, err := ParseTrustedProxies([]string{"proxy.local"}); err == nil {
		t.Error("a host name must be rejected: only addresses can be checked against a peer")
	}
}
