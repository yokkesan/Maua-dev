// Package clientip resolves rate-limit identities without trusting arbitrary
// forwarding headers. Only the transport peer can authorize proxy headers.
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type Resolver struct{ trusted []netip.Prefix }

// New accepts a comma-separated list of explicit proxy IPs or CIDRs. Empty
// configuration trusts no proxies. Universal trust ranges are rejected.
func New(config string) (*Resolver, error) {
	r := &Resolver{}
	if strings.TrimSpace(config) == "" {
		return r, nil
	}
	for _, entry := range strings.Split(config, ",") {
		entry = strings.TrimSpace(entry)
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			addr, parseErr := netip.ParseAddr(entry)
			if parseErr != nil || addr.Zone() != "" {
				return nil, fmt.Errorf("invalid trusted proxy IP/CIDR")
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		if prefix.Bits() == 0 || prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("invalid trusted proxy range")
		}
		r.trusted = append(r.trusted, prefix.Masked())
	}
	return r, nil
}
func (r *Resolver) trusts(addr netip.Addr) bool {
	if r == nil {
		return false
	}
	for _, prefix := range r.trusted {
		if prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}
func (r *Resolver) IP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || peer.Zone() != "" {
		return "unknown"
	}
	peer = peer.Unmap()
	if !r.trusts(peer) {
		return peer.String()
	}
	forwarded := strings.Join(request.Header.Values("X-Forwarded-For"), ",")
	if forwarded == "" || len(forwarded) > 4096 {
		return peer.String()
	}
	hops := strings.Split(forwarded, ",")
	if len(hops) > 32 {
		return peer.String()
	}
	// Walk back from the trusted peer and stop at the first untrusted hop.
	// Client-supplied entries further left can never replace that identity.
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil || addr.Zone() != "" {
			return peer.String()
		}
		addr = addr.Unmap()
		if !r.trusts(addr) || i == 0 {
			return addr.String()
		}
	}
	return peer.String()
}
