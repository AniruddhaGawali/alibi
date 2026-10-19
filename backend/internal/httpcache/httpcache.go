// Package httpcache caches GET responses in one place, so handlers and stores stay
// untouched: a route is cached by wrapping it in main.go with a Policy, and a route
// that changes data says which cached data it makes stale with Invalidates.
//
// Server side, a response is stored in Redis (only 200s, per user unless Shared) with
// a TTL. It carries the version of each of its tags; Invalidate bumps a tag's counter,
// which makes every response stored under that tag a miss, without scanning keys. A
// stored response is also checked against the tags' current versions on every hit, so
// a stale response is never served.
//
// Browser side, the policy sets Cache-Control and an ETag. MaxAge > 0 lets the browser
// reuse the response without asking (rarely-changing data); MaxAge == 0 sends
// "private, no-cache", so the browser revalidates with If-None-Match and the server
// answers 304 straight from Redis. Errors are never cached anywhere.
//
// Redis trouble never fails a request: the handler just runs.
package httpcache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"backend/pkg/utils"
	"github.com/redis/go-redis/v9"
)

const (
	entryPrefix = "hc:e:"
	tagPrefix   = "hc:t:"
	// tagTTL outlives every entry TTL, so an expired counter can never bring a stale entry back.
	tagTTL = 48 * time.Hour
	// maxBody is the largest response worth keeping.
	maxBody = 512 << 10
)

// Policy says how a route's responses are cached.
type Policy struct {
	// TTL is how long Redis keeps a response. 0 = no server cache (browser headers only).
	TTL time.Duration
	// MaxAge is how long the browser may reuse a response without asking the server.
	// 0 = "private, no-cache": reuse only after the server confirms (304).
	MaxAge time.Duration
	// Shared stores one response for all users. Leave it false for anything that depends
	// on who is asking, or whose handler checks permissions.
	Shared bool
	// Tags name the data the response depends on; bumping any of them drops it.
	// A tag may use {uid} (the caller) and {name} (a path value, e.g. {id}).
	Tags []string
}

// Cache is the Redis-backed response cache.
type Cache struct {
	rdb    *redis.Client
	userID func(context.Context) string
	off    bool
}

// New creates a Cache. userID reads the authenticated user from the request context.
func New(rdb *redis.Client, userID func(context.Context) string) *Cache {
	return &Cache{rdb: rdb, userID: userID}
}

type entry struct {
	Versions []int64 `json:"v"`
	Type     string  `json:"t"`
	Body     []byte  `json:"b"`
	ETag     string  `json:"e"`
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// expand fills a tag template from the request.
func (c *Cache) expand(tmpl string, r *http.Request, uid string) string {
	return placeholder.ReplaceAllStringFunc(tmpl, func(m string) string {
		name := m[1 : len(m)-1]
		if name == "uid" {
			return uid
		}
		return r.PathValue(name)
	})
}

func (c *Cache) tags(tmpls []string, r *http.Request, uid string) []string {
	out := make([]string, len(tmpls))
	for i, t := range tmpls {
		out[i] = c.expand(t, r, uid)
	}
	return out
}

// BypassParam is the query parameter that skips the cache for one request:
// GET /api/...?force_refresh=true runs the handler, refreshes the stored response and
// tells the browser not to reuse it. It is not part of the cache key.
const BypassParam = "force_refresh"

// bypassed reports whether the request asked to skip the cache, and returns the URI
// without the parameter (the cache key).
func bypassed(r *http.Request) (bool, string) {
	q := r.URL.Query()
	if !q.Has(BypassParam) {
		return false, r.URL.RequestURI()
	}
	force := q.Get(BypassParam) == "true" || q.Get(BypassParam) == "1"
	q.Del(BypassParam)
	uri := r.URL.Path
	if enc := q.Encode(); enc != "" {
		uri += "?" + enc
	}
	return force, uri
}

func entryKey(uri, uid string, shared bool) string {
	h := sha256.New()
	if !shared {
		h.Write([]byte(uid))
	}
	h.Write([]byte{0})
	h.Write([]byte(uri))
	return entryPrefix + hex.EncodeToString(h.Sum(nil))
}

func cacheControl(p Policy) string {
	if p.MaxAge > 0 {
		return "private, max-age=" + strconv.Itoa(int(p.MaxAge.Seconds()))
	}
	return "private, no-cache"
}

// SetEnabled switches the cache on or off. Off, Wrap and Invalidates return their handler
// unchanged: no Redis access and no cache headers.
func (c *Cache) SetEnabled(on bool) { c.off = !on }

// Wrap caches GET responses of h according to p.
func (c *Cache) Wrap(p Policy, h http.HandlerFunc) http.HandlerFunc {
	if c.off {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h(w, r)
			return
		}
		if p.TTL <= 0 {
			if force, _ := bypassed(r); force {
				p.MaxAge = 0
			}
			// Browser headers only: set them when (and only when) the answer is a 200.
			h(&headerWriter{ResponseWriter: w, value: cacheControl(p)}, r)
			return
		}
		ctx := r.Context()
		uid := c.userID(ctx)
		if uid == "" && !p.Shared {
			h(w, r)
			return
		}
		tags := c.tags(p.Tags, r, uid)
		force, uri := bypassed(r)
		key := entryKey(uri, uid, p.Shared)
		if force {
			p.MaxAge = 0 // a forced response is not reused by the browser without asking
		}

		// One round trip: the stored response and the current version of each tag. The
		// versions are read before the handler runs, so a change made while it runs
		// leaves the stored response already stale instead of wrongly fresh.
		versions, cached, err := c.lookup(ctx, key, tags)
		if err != nil {
			utils.LogWarnCtx(ctx, "response cache unavailable", "err", err)
			h(w, r)
			return
		}
		if cached != nil && !force && equal(cached.Versions, versions) {
			c.serve(w, r, p, cached, "HIT")
			return
		}

		rec := &recorder{header: http.Header{}, status: http.StatusOK}
		h(rec, r)
		if rec.status != http.StatusOK || rec.body.Len() > maxBody || rec.body.Len() == 0 {
			rec.replay(w)
			return
		}
		sum := sha256.Sum256(rec.body.Bytes())
		e := &entry{
			Versions: versions, Type: rec.header.Get("Content-Type"), Body: rec.body.Bytes(),
			ETag: `"` + hex.EncodeToString(sum[:12]) + `"`,
		}
		if raw, err := json.Marshal(e); err == nil {
			if err := c.rdb.Set(ctx, key, raw, p.TTL).Err(); err != nil {
				utils.LogWarnCtx(ctx, "response cache write failed", "err", err)
			}
		}
		c.serve(w, r, p, e, "MISS")
	}
}

func (c *Cache) lookup(ctx context.Context, key string, tags []string) ([]int64, *entry, error) {
	pipe := c.rdb.Pipeline()
	get := pipe.Get(ctx, key)
	tagKeys := make([]string, len(tags))
	for i, t := range tags {
		tagKeys[i] = tagPrefix + t
	}
	var mget *redis.SliceCmd
	if len(tagKeys) > 0 {
		mget = pipe.MGet(ctx, tagKeys...)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, nil, err
	}
	versions := make([]int64, len(tags))
	if mget != nil {
		for i, v := range mget.Val() {
			if s, ok := v.(string); ok {
				versions[i], _ = strconv.ParseInt(s, 10, 64)
			}
		}
	}
	raw, err := get.Bytes()
	if err != nil {
		return versions, nil, nil // a miss
	}
	var e entry
	if json.Unmarshal(raw, &e) != nil {
		return versions, nil, nil
	}
	return versions, &e, nil
}

func equal(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// serve writes a stored response, or a 304 when the browser already has it.
func (c *Cache) serve(w http.ResponseWriter, r *http.Request, p Policy, e *entry, state string) {
	h := w.Header()
	h.Set("Cache-Control", cacheControl(p))
	h.Set("ETag", e.ETag)
	h.Set("X-Cache", state)
	if matches(r.Header.Get("If-None-Match"), e.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", e.Type)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(e.Body)
}

func matches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimPrefix(strings.TrimSpace(part), "W/") == etag {
			return true
		}
	}
	return false
}

// Invalidates wraps a route that changes data: when it succeeds (status below 300),
// every response cached under the given tags is dropped. Tags use the same
// placeholders as a Policy.
//
// The handler's response is held back until the tags are bumped, so by the time a client
// learns the write succeeded, its follow-up GET can no longer be answered from the old cache.
func (c *Cache) Invalidates(h http.HandlerFunc, tags ...string) http.HandlerFunc {
	if c.off {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &recorder{header: http.Header{}, status: http.StatusOK}
		h(rec, r)
		if rec.status < 300 {
			c.Bump(r.Context(), c.tags(tags, r, c.userID(r.Context()))...)
		}
		rec.replay(w)
	}
}

// Bump invalidates tags directly, for code that changes data outside an HTTP route
// (the chat worker).
func (c *Cache) Bump(ctx context.Context, tags ...string) {
	if c == nil || c.off || len(tags) == 0 {
		return
	}
	// Detached from the request: the change is already committed, so the bump must land
	// even if the client has gone.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	pipe := c.rdb.Pipeline()
	for _, t := range tags {
		pipe.Incr(ctx, tagPrefix+t)
		pipe.Expire(ctx, tagPrefix+t, tagTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		utils.LogErrorCtx(ctx, "response cache invalidation failed", "tags", tags, "err", err)
	}
}

// recorder buffers a handler's response.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header         { return r.header }
func (r *recorder) WriteHeader(code int)        { r.status = code }
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }

func (r *recorder) replay(w http.ResponseWriter) {
	for k, v := range r.header {
		w.Header()[k] = v
	}
	w.WriteHeader(r.status)
	_, _ = w.Write(r.body.Bytes())
}

// headerWriter adds Cache-Control to 200 responses only.
type headerWriter struct {
	http.ResponseWriter
	value string
	wrote bool
}

func (h *headerWriter) WriteHeader(code int) {
	if !h.wrote {
		h.wrote = true
		if code == http.StatusOK {
			h.Header().Set("Cache-Control", h.value)
		}
	}
	h.ResponseWriter.WriteHeader(code)
}

func (h *headerWriter) Write(b []byte) (int, error) {
	if !h.wrote {
		h.WriteHeader(http.StatusOK)
	}
	return h.ResponseWriter.Write(b)
}
