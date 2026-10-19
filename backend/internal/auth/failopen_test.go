package auth

import (
	"net/http"
	"testing"
)

func TestMiddlewareFailsOpenWhenConfigured(t *testing.T) {
	e := newEnv(t)
	e.bl.SetFailOpen(true)
	tk := signup(t, e)
	e.mr.Close() // Redis goes away
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != http.StatusOK {
		t.Fatalf("a valid token must still work while Redis is down, got %d", rec.Code)
	}
	// forged or missing tokens are still refused: only the revoked-token lookup is skipped
	if rec := e.do("GET", "/api/me", "", tk.JWT+"x"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged token: %d", rec.Code)
	}
	if rec := e.do("GET", "/api/me", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
}

func TestLogoutStaysEffectiveOnThisInstanceWhenRedisDies(t *testing.T) {
	e := newEnv(t)
	e.bl.SetFailOpen(true)
	tk := signup(t, e)
	if rec := e.do("POST", "/api/auth/logout", `{"refresh_token":"`+tk.Refresh+`"}`, tk.JWT); rec.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	e.mr.Close()
	if rec := e.do("GET", "/api/me", "", tk.JWT); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a logged-out token was accepted after Redis went down: %d", rec.Code)
	}
}

func TestLogoutWhileRedisIsDown(t *testing.T) {
	for _, open := range []bool{true, false} {
		e := newEnv(t)
		e.bl.SetFailOpen(open)
		tk := signup(t, e)
		e.mr.Close()
		rec := e.do("POST", "/api/auth/logout", `{"refresh_token":"`+tk.Refresh+`"}`, tk.JWT)
		want := http.StatusInternalServerError
		if open {
			want = http.StatusOK
		}
		// (with fail-closed the middleware itself answers 503 before the handler runs)
		if !open {
			want = http.StatusServiceUnavailable
		}
		if rec.Code != want {
			t.Fatalf("failOpen=%v: logout got %d, want %d", open, rec.Code, want)
		}
		// either way the token is dead on this instance
		if open {
			if r := e.do("GET", "/api/me", "", tk.JWT); r.Code != http.StatusUnauthorized {
				t.Fatalf("token still works after logout: %d", r.Code)
			}
		}
	}
}

func TestLoginAndRefreshWorkWhileRedisIsDown(t *testing.T) {
	e := newEnv(t)
	e.bl.SetFailOpen(true)
	e.h.Limiter.WithFallback(true)
	tk := signup(t, e)
	e.mr.Close()

	if rec := e.do("POST", "/api/auth/login", `{"email":"sherlock@example.com","password":"{{PW}}"}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("login with Redis down: %d %s", rec.Code, rec.Body)
	}
	// refresh works once, then the old refresh token is refused (rotation held in memory)
	body := `{"refresh_token":"` + tk.Refresh + `"}`
	if rec := e.do("POST", "/api/auth/refresh", body, ""); rec.Code != http.StatusOK {
		t.Fatalf("refresh with Redis down: %d %s", rec.Code, rec.Body)
	}
	if rec := e.do("POST", "/api/auth/refresh", body, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a refresh token worked twice: %d", rec.Code)
	}
}

func TestLoginLockoutStillAppliesWhileRedisIsDown(t *testing.T) {
	e := newEnv(t)
	e.bl.SetFailOpen(true)
	e.h.Limiter.WithFallback(true)
	signup(t, e)
	e.mr.Close()

	wrong := `{"email":"sherlock@example.com","password":"Wrong1234"}`
	for i := 0; i < 5; i++ {
		if rec := e.do("POST", "/api/auth/login", wrong, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d, want 401", i+1, rec.Code)
		}
	}
	rec := e.do("POST", "/api/auth/login", wrong, "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("after the failures login must be locked: %d", rec.Code)
	}
	// even the right password is refused while locked
	if rec := e.do("POST", "/api/auth/login", `{"email":"sherlock@example.com","password":"{{PW}}"}`, ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked account accepted the right password: %d", rec.Code)
	}
}

func TestSignupNeedsRedisAndSaysSo(t *testing.T) {
	e := newEnv(t)
	e.bl.SetFailOpen(true)
	e.h.Limiter.WithFallback(true)
	e.mr.Close()
	rec := e.do("POST", "/api/auth/signup", signupBody, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("signup with Redis down: %d, want 503", rec.Code)
	}
}
