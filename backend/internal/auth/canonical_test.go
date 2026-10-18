package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestCanonicalEmail(t *testing.T) {
	for in, want := range map[string]string{
		"Me@Gmail.com":         "me@gmail.com",
		"me+1@gmail.com":       "me@gmail.com",
		"m.e+x@googlemail.com": "me@gmail.com",
		"me+1@example.com":     "me@example.com",
		"a.b@example.com":      "a.b@example.com", // only Gmail ignores dots
		"+x@example.com":       "+x@example.com",
		"no-at":                "no-at",
	} {
		if got := canonicalEmail(in); got != want {
			t.Errorf("canonicalEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlusAddressesShareOneOTPBudgetAndSignupsAreCapped(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	l := NewLimiter(rdb, Policy{Cooldown: time.Minute, EmailDailyMax: 5, IPMax: 100, IPWindow: time.Hour, SignupIPDailyMax: 2})
	ctx := context.Background()

	if wait, err := l.Take(ctx, "me+1@gmail.com", "1.1.1.1"); err != nil || wait != 0 {
		t.Fatalf("first: %v %v", wait, err)
	}
	if wait, _ := l.Take(ctx, "m.e+2@gmail.com", "2.2.2.2"); wait == 0 {
		t.Fatal("another +tag of the same mailbox must hit the same cooldown")
	}

	for i := range 2 {
		if wait, err := l.HitSignup(ctx, "9.9.9.9"); err != nil || wait != 0 {
			t.Fatalf("signup %d: %v %v", i, wait, err)
		}
	}
	if wait, _ := l.HitSignup(ctx, "9.9.9.9"); wait == 0 {
		t.Fatal("third account from one address in a day should be refused")
	}
	if wait, _ := l.HitSignup(ctx, "8.8.8.8"); wait != 0 {
		t.Fatal("another address is not affected")
	}
}
