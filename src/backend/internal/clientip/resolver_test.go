package clientip

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolverTrustBoundary(t *testing.T) {
	for _, tc := range []struct{ name, config, peer, xff, real, want string }{
		{"unset ignores forwarding", "", "192.0.2.1:123", "203.0.113.1", "203.0.113.2", "192.0.2.1"},
		{"untrusted peer", "192.0.2.2/32", "192.0.2.1:123", "203.0.113.1", "", "192.0.2.1"},
		{"trusted edge", "192.0.2.1", "192.0.2.1:123", "203.0.113.1", "", "203.0.113.1"},
		{"spoofed left hop", "192.0.2.1", "192.0.2.1:123", "198.51.100.7,203.0.113.1", "", "203.0.113.1"},
		{"two trusted proxies", "192.0.2.0/24", "192.0.2.1:123", "198.51.100.7,203.0.113.1,192.0.2.2", "", "203.0.113.1"},
		{"missing XFF ignores XRealIP", "192.0.2.1", "192.0.2.1:123", "", "203.0.113.1", "192.0.2.1"},
		{"malformed closest hop", "192.0.2.1", "192.0.2.1:123", "203.0.113.1,bad", "", "192.0.2.1"},
		{"malformed attacker hop ignored", "192.0.2.1", "192.0.2.1:123", "bad,203.0.113.1", "", "203.0.113.1"},
		{"IPv6", "2001:db8::1/128", "[2001:db8::1]:123", "2001:db8:1::1", "", "2001:db8:1::1"},
		{"IPv4 mapped canonical", "192.0.2.1", "[::ffff:192.0.2.1]:123", "::ffff:203.0.113.1", "", "203.0.113.1"},
		{"oversized header", "192.0.2.1", "192.0.2.1:123", strings.Repeat("a", 4097), "", "192.0.2.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := New(tc.config)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("X-Forwarded-For", tc.xff)
			req.Header.Set("X-Real-IP", tc.real)
			if got := r.IP(req); got != tc.want {
				t.Fatalf("IP=%s want %s", got, tc.want)
			}
		})
	}
	for _, config := range []string{"0.0.0.0/0", "::/0", "bad", "192.0.2.1,", "::ffff:192.0.2.0/120"} {
		if _, err := New(config); err == nil {
			t.Fatalf("unsafe/invalid config accepted: %q", config)
		}
	}
}
