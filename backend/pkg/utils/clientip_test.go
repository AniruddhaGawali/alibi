package utils

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	_, private, _ := net.ParseCIDR("10.0.0.0/8")
	trusted := []*net.IPNet{private}

	cases := []struct {
		name, remote, xff, realIP, want string
	}{
		{"direct client ignores forged headers", "203.0.113.9:1234", "1.2.3.4", "5.6.7.8", "203.0.113.9"},
		{"trusted proxy, single hop", "10.0.0.5:80", "198.51.100.7", "", "198.51.100.7"},
		{"client cannot prepend a fake address", "10.0.0.5:80", "6.6.6.6, 198.51.100.7", "", "198.51.100.7"},
		{"skips our own proxies", "10.0.0.5:80", "198.51.100.7, 10.0.0.9", "", "198.51.100.7"},
		{"falls back to X-Real-IP", "10.0.0.5:80", "", "198.51.100.8", "198.51.100.8"},
		{"garbage header falls back", "10.0.0.5:80", "not-an-ip", "", "10.0.0.5"},
		{"no headers", "10.0.0.5:80", "", "", "10.0.0.5"},
		{"ipv6 peer is masked to its /64", "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:443", "", "", "2001:db8:1:2::"},
		{"ipv6 client behind proxy is masked", "10.0.0.5:80", "2001:db8:1:2:1::9", "", "2001:db8:1:2::"},
		{"ipv4-mapped ipv6 stays ipv4", "[::ffff:203.0.113.9]:443", "", "", "203.0.113.9"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if c.realIP != "" {
			r.Header.Set("X-Real-IP", c.realIP)
		}
		if got := ClientIP(r, trusted); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
