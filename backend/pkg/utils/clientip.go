package utils

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the caller's IP address.
//
// X-Forwarded-For and X-Real-IP are plain headers any client can forge, so
// they are only believed when the TCP peer (RemoteAddr) is one of the trusted
// proxies. Then X-Forwarded-For is read right to left, skipping our own
// proxies, and the first address that is not one of them is the client;
// X-Real-IP is the fallback. Otherwise the peer address is returned.
//
// The result is meant as a rate-limit key, so IPv6 addresses are masked to their
// /64: one subscriber owns a whole /64 and could otherwise rotate addresses
// to dodge per-IP limits.
func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	return normalizeIP(rawClientIP(r, trusted))
}

// normalizeIP returns IPv4 (including IPv4-mapped IPv6) in dotted form, masks
// IPv6 to /64 and leaves unparseable input as is.
func normalizeIP(s string) string {
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String()
}

func rawClientIP(r *http.Request, trusted []*net.IPNet) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	peerIP := net.ParseIP(peer)
	if peerIP == nil || !inNets(peerIP, trusted) {
		return peer
	}

	if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
		hops := strings.Split(strings.Join(xff, ","), ",")
		for i := len(hops) - 1; i >= 0; i-- {
			ip := net.ParseIP(strings.TrimSpace(hops[i]))
			if ip == nil {
				break // garbage in the chain: stop trusting it
			}
			if !inNets(ip, trusted) {
				return ip.String()
			}
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return peer
}

func inNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
