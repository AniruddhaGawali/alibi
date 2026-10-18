package config

import (
	"net"
	"strings"
	"testing"

	"github.com/joho/godotenv"
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

func TestEnvFilesSearchParentDirectories(t *testing.T) {
	got := envFiles()
	want := []string{".env", "../.env", "../../.env", "../../../.env"} // cwd + 3 parents: backend/cmd/<name> reaches the repo root
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCheckSupabase(t *testing.T) {
	ok := []Config{
		{}, // not configured: uploads disabled, not an error
		{SupabaseURL: "https://abc.supabase.co", SupabaseServiceRoleKey: "k"},
		{SupabaseURL: "http://localhost:54321", SupabaseServiceRoleKey: "k"},
	}
	for _, c := range ok {
		if err := checkSupabase(&c); err != nil {
			t.Errorf("%+v: %v", c, err)
		}
	}
	bad := []Config{
		{SupabaseURL: "https://abc.supabase.co"}, // key missing
		{SupabaseURL: "abc.supabase.co", SupabaseServiceRoleKey: "k"},
		{SupabaseURL: "ftp://abc", SupabaseServiceRoleKey: "k"},
		{SupabaseURL: "http://abc.supabase.co", SupabaseServiceRoleKey: "k"}, // key would travel in clear text
	}
	for _, c := range bad {
		if err := checkSupabase(&c); err == nil {
			t.Errorf("%+v: want error", c)
		}
	}
}

// Copying .env.example to .env must give a working config: no value may carry an
// inline "# comment" (godotenv would keep it as part of the value), and the
// secret must be left blank so nobody runs with a key that is in git.
func TestEnvExampleIsSafeToCopy(t *testing.T) {
	vars, err := godotenv.Read("../../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range vars {
		if strings.Contains(v, "#") {
			t.Errorf("%s=%q has an inline comment; put comments on their own line", k, v)
		}
	}
	if vars["JWT_SECRET"] != "" {
		t.Errorf("JWT_SECRET must be blank in .env.example")
	}
	if err := checkSupabase(&Config{SupabaseURL: vars["SUPABASE_URL"], SupabaseServiceRoleKey: vars["SUPABASE_SERVICE_ROLE_KEY"]}); err != nil {
		t.Errorf("example Supabase settings rejected: %v", err)
	}
}
