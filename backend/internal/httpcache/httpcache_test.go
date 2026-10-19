package httpcache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type ctxKey struct{}

func setup(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb, func(ctx context.Context) string { s, _ := ctx.Value(ctxKey{}).(string); return s }), mr
}

func call(h http.Handler, method, path, uid string, hdr ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, uid))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCacheHitMissAndInvalidate(t *testing.T) {
	c, _ := setup(t)
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/cases/{id}/notes", c.Wrap(Policy{TTL: time.Minute, Tags: []string{"u:{uid}:notes:{id}"}}, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"n":` + strconv.Itoa(calls) + `}`))
	}))
	mux.HandleFunc("POST /api/cases/{id}/notes", c.Invalidates(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}, "u:{uid}:notes:{id}"))

	first := call(mux, "GET", "/api/cases/c1/notes", "alice")
	second := call(mux, "GET", "/api/cases/c1/notes", "alice")
	if calls != 1 || first.Header().Get("X-Cache") != "MISS" || second.Header().Get("X-Cache") != "HIT" || second.Body.String() != `{"n":1}` {
		t.Fatalf("calls=%d first=%s second=%s %s", calls, first.Header().Get("X-Cache"), second.Header().Get("X-Cache"), second.Body)
	}
	if cc := second.Header().Get("Cache-Control"); cc != "private, no-cache" {
		t.Fatalf("cache-control %q", cc)
	}
	// another user and another case do not share the entry
	call(mux, "GET", "/api/cases/c1/notes", "bob")
	call(mux, "GET", "/api/cases/c2/notes", "alice")
	if calls != 3 {
		t.Fatalf("calls=%d, want 3", calls)
	}
	// someone else's change does not drop alice's entry; her own does
	call(mux, "POST", "/api/cases/c1/notes", "bob")
	call(mux, "GET", "/api/cases/c1/notes", "alice")
	if calls != 3 {
		t.Fatalf("bob's write invalidated alice's cache (calls=%d)", calls)
	}
	call(mux, "POST", "/api/cases/c1/notes", "alice")
	if got := call(mux, "GET", "/api/cases/c1/notes", "alice"); calls != 4 || got.Body.String() != `{"n":4}` {
		t.Fatalf("not refreshed after invalidation: calls=%d %s", calls, got.Body)
	}
}

func TestRevalidation304(t *testing.T) {
	c, _ := setup(t)
	h := c.Wrap(Policy{TTL: time.Minute, MaxAge: time.Hour}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"x":1}`))
	})
	first := call(h, "GET", "/p", "u")
	if cc := first.Header().Get("Cache-Control"); cc != "private, max-age=3600" {
		t.Fatalf("cache-control %q", cc)
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no etag")
	}
	again := call(h, "GET", "/p", "u", "If-None-Match", etag)
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Fatalf("expected an empty 304, got %d %q", again.Code, again.Body)
	}
	if other := call(h, "GET", "/p", "u", "If-None-Match", `"stale"`); other.Code != http.StatusOK {
		t.Fatalf("a different etag must get the body: %d", other.Code)
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	c, _ := setup(t)
	calls := 0
	h := c.Wrap(Policy{TTL: time.Minute, MaxAge: time.Hour}, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	})
	a, b := call(h, "GET", "/p", "u"), call(h, "GET", "/p", "u")
	if calls != 2 || a.Header().Get("Cache-Control") != "" || b.Code != http.StatusNotFound {
		t.Fatalf("an error was cached: calls=%d cc=%q", calls, a.Header().Get("Cache-Control"))
	}
}

func TestHeadersOnlyPolicy(t *testing.T) {
	c, _ := setup(t)
	ok := c.Wrap(Policy{MaxAge: time.Minute}, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })
	if cc := call(ok, "GET", "/p", "u").Header().Get("Cache-Control"); cc != "private, max-age=60" {
		t.Fatalf("cache-control %q", cc)
	}
	bad := c.Wrap(Policy{MaxAge: time.Minute}, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	if cc := call(bad, "GET", "/p", "u").Header().Get("Cache-Control"); cc != "" {
		t.Fatalf("a 500 got cache-control %q", cc)
	}
}

func TestSharedAndFailOpen(t *testing.T) {
	c, mr := setup(t)
	calls := 0
	h := c.Wrap(Policy{TTL: time.Minute, Shared: true}, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte("shared"))
	})
	call(h, "GET", "/s", "alice")
	call(h, "GET", "/s", "bob")
	if calls != 1 {
		t.Fatalf("a shared response was computed %d times", calls)
	}
	mr.Close() // Redis goes away: requests still work
	if rec := call(h, "GET", "/s", "alice"); rec.Code != http.StatusOK || rec.Body.String() != "shared" {
		t.Fatalf("not fail-open: %d", rec.Code)
	}
	// without a user, a per-user policy never caches
	pu := c.Wrap(Policy{TTL: time.Minute}, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })
	if rec := call(pu, "GET", "/u", ""); rec.Header().Get("X-Cache") != "" {
		t.Fatal("anonymous response was cached per user")
	}
}

func TestFailedWriteDoesNotInvalidate(t *testing.T) {
	c, _ := setup(t)
	calls := 0
	get := c.Wrap(Policy{TTL: time.Minute, Tags: []string{"t"}}, func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = w.Write([]byte("x")) })
	post := c.Invalidates(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusConflict) }, "t")
	call(get, "GET", "/g", "u")
	call(post, "POST", "/g", "u")
	call(get, "GET", "/g", "u")
	if calls != 1 {
		t.Fatalf("a failed write dropped the cache (calls=%d)", calls)
	}
}

func TestForceRefreshBypass(t *testing.T) {
	c, _ := setup(t)
	calls := 0
	h := c.Wrap(Policy{TTL: time.Minute, MaxAge: time.Hour}, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte("v" + strconv.Itoa(calls)))
	})
	call(h, "GET", "/p?x=1", "u")
	if rec := call(h, "GET", "/p?x=1", "u"); rec.Body.String() != "v1" {
		t.Fatalf("expected a hit, got %s", rec.Body)
	}
	forced := call(h, "GET", "/p?x=1&force_refresh=true", "u")
	if forced.Body.String() != "v2" || forced.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("forced: %s cc=%q", forced.Body, forced.Header().Get("Cache-Control"))
	}
	// the forced response refreshed the stored one, under the same key
	if rec := call(h, "GET", "/p?x=1", "u"); rec.Body.String() != "v2" || calls != 2 {
		t.Fatalf("stored response not refreshed: %s calls=%d", rec.Body, calls)
	}
}
