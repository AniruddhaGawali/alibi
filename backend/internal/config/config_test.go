package config

import "testing"

func TestEnvIntFallsBackOnInvalid(t *testing.T) {
	t.Setenv("X_NUM", "42")
	if got := envInt("X_NUM", 7); got != 42 {
		t.Fatalf("got %d", got)
	}
	for _, bad := range []string{"", "abc", "0", "-3"} {
		t.Setenv("X_NUM", bad)
		if got := envInt("X_NUM", 7); got != 7 {
			t.Fatalf("value %q: got %d, want default 7", bad, got)
		}
	}
}
