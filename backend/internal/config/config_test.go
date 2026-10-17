package config

import (
	"net"
	"testing"
)

func TestEnvIntFallsBackOnInvalid(t *testing.T) {
	t.Setenv("X_NUM", "42")
	if got := envInt("X_NUM", 7, 1, 100); got != 42 {
		t.Fatalf("got %d", got)
	}
	for _, bad := range []string{"", "abc", "0", "-3", "101"} {
		t.Setenv("X_NUM", bad)
		if got := envInt("X_NUM", 7, 1, 100); got != 7 {
			t.Fatalf("value %q: got %d, want default 7", bad, got)
		}
	}
}

func TestEnvIntAcceptsZeroWhenMinIsZero(t *testing.T) {
	t.Setenv("X_NUM", "0")
	if got := envInt("X_NUM", 2, 0, 100); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}

func TestOTLPHostPort(t *testing.T) {
	cases := map[string]string{
		"localhost:4318":            "localhost:4318",
		"http://collector:4318":     "collector:4318",
		"https://collector:4318/":   "collector:4318",
		"http://collector:4318/v1/": "collector:4318",
	}
	for in, want := range cases {
		if got := otlpHostPort(in); got != want {
			t.Errorf("otlpHostPort(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCIDRs(t *testing.T) {
	if n, err := parseCIDRs("127.0.0.0/8, 10.0.0.0/8"); err != nil || len(n) != 2 {
		t.Fatalf("got %v %v", n, err)
	}
	for _, in := range []string{"", "none", " NONE "} {
		if n, err := parseCIDRs(in); err != nil || len(n) != 0 {
			t.Errorf("%q: got %v %v", in, n, err)
		}
	}
	if _, err := parseCIDRs("10.0.0.0/8,not-a-cidr"); err == nil {
		t.Fatal("expected an error for an invalid CIDR")
	}
}

func TestDefaultTrustedProxiesAreLoopbackOnly(t *testing.T) {
	nets, err := parseCIDRs(defaultTrustedProxies)
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"172.18.0.1", "10.1.2.3", "192.168.1.5"} {
		for _, n := range nets {
			if n.Contains(net.ParseIP(ip)) {
				t.Errorf("%s must not be trusted by default (Docker gateway / private ranges)", ip)
			}
		}
	}
}
