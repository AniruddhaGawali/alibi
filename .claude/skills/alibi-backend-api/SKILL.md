---
name: alibi-backend-api
description: How to create API endpoints in the Alibi Go backend (backend/). Use this skill whenever you add or change an HTTP handler, route, or endpoint (auth, dashboard, stories, chat, accusation, admin, health, anything served over HTTP), or write any code that reads request data or sends an HTTP response. Also use it whenever backend code touches PostgreSQL, a table, a column, a query, or a migration: ALL database work must go through goose migrations (backend/sql/migrations) and sqlc-generated type-safe queries (backend/sql/queries -> internal/db/dbgen), never hand-written SQL in Go. Also use it for backend startup/config, queue workers, health checks and backend tests. All client input must be validated, all responses must go through the shared SuccessResponse/ErrorResponse JSON helpers, and every API must be logged with the logger functions.
---

# Creating API endpoints in the Go backend

Four rules apply to every endpoint:

1. Validate everything the client sends.
2. Respond only through the shared JSON helpers.
3. Log every API through the logger.
4. **Every database change is a goose migration and every query is a sqlc query.** No SQL strings in Go, ever.

They exist so the frontend can rely on one response shape, every request is observable in ClickHouse, and the database layer is type-checked at generate time instead of failing at runtime.

## Adding an endpoint: the order of work

1. **Schema** (only if a table or column is needed): `cd backend && make migrate-new name=<snake_case>`, write `Up` **and** `Down`, `make migrate-up`.
2. **Queries**: add named queries to `backend/sql/queries/<domain>.sql`, run `make sqlc`.
3. **Store**: a thin wrapper in `internal/<domain>/store.go` over `dbgen.Queries` that returns domain errors (rule 4 below).
4. **Handler**: validate, call the store, respond, log (rules 1-3).
5. **Route**: register it on the mux in `cmd/api/main.go`. It is protected automatically: `auth.Middleware` checks the JWT before any handler runs. Only add it to `auth.NewPublicRoutes(...)` if it must work without login (signup, login, refresh, health, public images), and say so in your summary.
6. **Tests**, the checks at the bottom, then tell the frontend side the path and response shape (`alibi-api` skill).

## 1. Validate all user input before using it

Nothing from the client is trusted: request bodies, path values, query strings and headers. Validate before touching the database, queue, LLM or token balance, and reject bad input with an error response. This protects data integrity and money-like resources (tokens), and it keeps garbage out of the LLM prompts.

**Request bodies:** declare a request struct with `json` and `validate` tags, then call `utils.BindAndValidate` (`pkg/utils/validate.go`). It does the decoding and validation together and writes the error response itself, so you just return when it says false:

```go
type startReq struct {
	StoryID string `json:"story_id" validate:"required,uuid"`
	Mode    string `json:"mode" validate:"required,oneof=easy hard"`
}

var req startReq
if !utils.BindAndValidate(w, r, &req) {
	return
}
```

- Bad JSON, unknown fields, trailing data or a body over 1 MiB gives **400** `invalid request body`.
- Rule failures give **422** `validation failed`, with `details` as `{"field": "message"}` keyed by the JSON field name. The frontend can show those next to the fields.
- Common tags: `required`, `email`, `min`, `max`, `oneof`, `uuid`, plus the project rules `password` and `username`. For anything custom, add a rule in `validate.go` (with a message in `fieldMessage`) instead of ad-hoc code in handlers.

**Path and query values:** there is no body to bind, so check them with `utils.Validate` on a small struct (for example `validate:"required,uuid"` for `r.PathValue("id")`) or by hand, and answer 400 through `ErrorResponse`: parse ints/UUIDs and reject failures, apply bounds to `limit`/`offset`, and allow-list sort or filter values. Example: `limit` outside 1-100 is a 400, not silently clamped.

**Multipart uploads:** cap the body with `http.MaxBytesReader` (answer 413 on `*http.MaxBytesError`), check the real size of what you read, and verify content by decoding it rather than trusting the filename or `Content-Type`. See `internal/user/handler.go` (`SetAvatar`) for the pattern.

**Beyond shape:** validity also means the request is allowed. Check ownership and state (the case belongs to this user, the user has tokens, no active case lock) and return 403, 404, 409 or 429 accordingly. Never take identity from the body; use `auth.UserID(r.Context())` from the validated JWT.

**LLM and chat input:** trim it, enforce a max length via `validate:"max=..."`, and run it through the moderation layer before spending a token.

Never echo raw invalid input back in a message. Keep `message` generic and put only field names and rules in `details`.

## 2. Responses: only `SuccessResponse` / `ErrorResponse`

Defined in `backend/pkg/utils/response.go`. Never call `json.NewEncoder(w)`, `w.Write`, `http.Error`, or set `Content-Type` by hand in a handler.

```go
utils.SuccessResponse(w, http.StatusOK, data)                       // {"status":"success","data":...}
utils.ErrorResponse(w, http.StatusBadRequest, "invalid body", nil)  // {"status":"error","message":"...","details":...}
```

- The frontend's `apiClient` unwraps exactly this envelope, so deviating breaks it.
- Use the right status: 200/201 success, 400 bad input, 401 unauthenticated, 403 forbidden, 404 missing, 409 conflict, 413 too large, 415 wrong content type, 422 validation, 429 rate/limit (e.g. out of tokens), 500 unexpected, 503 dependency down.
- `message` is safe to show a user. Put field-level validation problems in `details` (e.g. `map[string]string{"email": "invalid"}`).
- Never leak internal errors (SQL, stack traces, driver text) in `message` or `details`. Log the real error, return a generic message.
- Return right after writing an error response so you don't write twice.
- Handlers never serve image bytes: images live in the object store and the API returns their public URL (see Images below).

## 3. Logging: every API, via the logger

Use the logger functions in `pkg/utils`: `LogInfo` / `LogWarn` / `LogError`, and inside handlers the context-aware `LogInfoCtx(ctx, ...)` / `LogErrorCtx(ctx, ...)`. They all go through `slog`'s default logger, which `logger.Init` points at OpenTelemetry, so every record is exported to ClickHouse. The `Ctx` variants also link the record to the request's trace span. Do not use `fmt.Println`, the stdlib `log` package, or your own `slog.New(...)` logger, because those bypass the pipeline.

- **Request and response logging is automatic** (method, path, IP, status, latency, trace span) as long as the route is served through `middleware.Logging`. Register every route on the mux that is wrapped by it in `cmd/api`; never serve a route outside it.
- **Inside handlers,** log what the middleware can't know, with key/value pairs:
  - `LogInfoCtx` for meaningful business events: `utils.LogInfoCtx(r.Context(), "token deducted", "user_id", uid, "remaining", n)`.
  - `LogErrorCtx` for every failure path, with the real error, then respond with a generic `ErrorResponse`.
  - Always log before returning a 5xx. Log 4xx only when useful (auth failures, suspicious input).
- Use stable snake_case keys (`user_id`, `case_id`, `err`). Never log secrets: passwords, JWTs, API keys, full request bodies.

## 4. Database: goose + sqlc (mandatory)

All SQL lives in `backend/sql/`; read `backend/sql/README.md` for the full workflow. The tools are the `goose` and `sqlc` CLIs (`go install github.com/pressly/goose/v3/cmd/goose@latest`, `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`, with `~/go/bin` on PATH). Run all `make` targets from `backend/`.

```
sql/migrations/   goose migrations (the schema; sqlc reads these too)
sql/queries/      named sqlc queries, one file per domain
internal/db/dbgen/  GENERATED Go (committed, never hand-edited)
sqlc.yaml, Makefile
```

**Migrations (goose):**
- `make migrate-new name=add_cases` creates `sql/migrations/0000N_add_cases.sql`. Write `-- +goose Up` and a real `-- +goose Down` that undoes it.
- Never edit a migration that has shipped; add a new one. Apply with `make migrate-up`; the API also applies pending ones at startup unless `MIGRATE_ON_START=false`.
- Use `UUID` ids (`gen_random_uuid()`), `TIMESTAMPTZ`, `NOT NULL` by default, and name constraints and indexes explicitly (the store maps constraint names to domain errors, e.g. `users_email_key`).
- Keep large columns (images, blobs) in their own table so `SELECT *` on the main table stays cheap.
- Do not run DDL from Go or by hand against the database; if it is not in a migration it does not exist.

**Queries (sqlc):**
- Write `-- name: GetThing :one` / `:many` / `:exec` / `:execrows` queries with named parameters (`@id`), never string-built SQL.
- Run `make sqlc` (it vets the queries against the migrations first). A query that does not match the schema fails here, not in production.
- Multi-step writes that must succeed or fail together belong in one statement (a CTE, as in `avatars.sql`) or a transaction in the store.
- Counters and balances are updated in SQL (`SET x = x + @delta ... WHERE x + @delta >= 0`), never read-modify-write in Go.
- The `overrides` in `sqlc.yaml` map `uuid` to `string`, `timestamptz` to `time.Time` (`*time.Time` when nullable) and nullable `int2` to `*int16`; do not add per-query casts in handlers.

**Stores:**
- A store in `internal/<domain>/store.go` wraps `*dbgen.Queries`, takes the pool as a dependency (no globals), and hides generated types behind domain types where that helps.
- The store, not the handler, maps `pgx.ErrNoRows` to a domain `ErrNotFound` and `pgerrcode` violations (`UniqueViolation`, `ForeignKeyViolation`) to domain errors. Handlers branch on those with `errors.Is`.
- Always pass `r.Context()` through to the store.

**Redis stays a cache, Postgres is the source of truth** for anything durable (see `internal/wallet`: write Postgres first, then invalidate the Redis key; reads fall back to Postgres if Redis is down).

**Before finishing any change under `backend/sql/`:** `make sqlc`, then `make sqlc-diff` must report nothing, then commit the generated `internal/db/dbgen/` together with the SQL.

## Auth in handlers

- **Every route requires a valid JWT by default.** `auth.Middleware` (installed in `cmd/api/main.go` inside `middleware.Logging`) runs before the mux: it answers 401 for a missing, malformed, expired, forged, wrong-type (refresh) or blacklisted access token and 503 if the blacklist cannot be checked, and only then passes the request on with the claims in the context. Handlers never re-check the token; read the caller with `auth.UserID(r.Context())`.
- **Public routes are an explicit allow-list**, `auth.NewPublicRoutes("METHOD /path", ...)` in `main.go` (a trailing `/` means "everything under it"). Currently `GET /health` and `POST /api/auth/{signup,resend-otp,verify-otp,login,refresh}`. Unknown paths and wrong methods get 401 too, so the API does not reveal which routes exist. Never add a route to the list just to make a test or a frontend call pass.
- **Signup is OTP-based and creates no user until the code is verified.** `signup` checks availability, takes an OTP request from the budget (`auth.Limiter`, Lua-atomic, 429 with `Retry-After`: per email a 60 s cooldown and 5/day, per IP 10 per 15 min), stores `otp:signup:<email>` (10 min, holding only an HMAC of the code) and queues the email; `verify-otp` creates the user with the 10 starting tokens and signs in; `resend-otp` issues a fresh code. Wrong codes are capped (`OTP_MAX_ATTEMPTS`) by `OTPStore.Verify`, which charges the attempt atomically *before* comparing. Every cheap-to-abuse endpoint first charges the per-IP budget (`Handler.gateIP`) before any DB or bcrypt work; login adds per-email/IP lockout with exponential backoff, and bcrypt runs under `Handler.acquireHash`. Any failure after the cooldown was taken must `Release` it. Use `utils.ClientIP(r, cfg.TrustedProxies)` for client IPs: forwarding headers are only believed from trusted proxies.
- Access tokens are short-lived JWTs (`sub`, `email`, `jti`, `exp`); refresh tokens rotate (each works once). Logout needs the access token header and the `refresh_token` body and blacklists both in Redis (`blacklist:jwt:{jti}` = `true`) for their remaining lifetime.
- Passwords are hashed with bcrypt and never logged or returned. OTPs are secrets: never log an `email.Message`.
- Question tokens (the game currency) go through `wallet.Service` (`Balance`, `Grant`, `Spend`, `Cache`), never by writing the Redis key (`user:coins:{id}`) or the column directly.

## Images: object storage, WebP only

Images (avatars, case thumbnails, scenes, suspect portraits) never go in Postgres. They live in a bucket behind `storage.Provider` (`internal/storage`): `UploadImage(ctx, path, data) (publicURL, error)`, `Delete`, `PublicURL`, `Ping`. Today that is `SupabaseProvider` (REST API with `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_BUCKET_NAME`); an S3 or GCS provider only needs to implement the interface and be chosen in `cmd/api/main.go`, with no handler changes.

- **WebP only.** The provider rejects anything that is not a WebP file (`storage.ErrNotWebP`) and any path that is not a clean relative `….webp` (`storage.ErrInvalidPath`). Handlers that accept user images decode, resize and re-encode them to WebP first (see `user/avatar.go`, `github.com/gen2brain/webp`, pure Go, works with `CGO_ENABLED=0`).
- **Bucket layout** mirrors `frontend/public`: `avatars/default/avatar-N.webp`, `avatars/<user_id>_<uuid>.webp` (custom uploads), `cases/<case>/thumbnail.webp`, `cases/<case>/scenes/…`, `cases/<case>/suspects/…`.
- **Unique names, no overwrites.** Give every upload a fresh name (UUID) so URLs can be cached forever. Store the public URL for the client and the object path for cleanup (`users.avatar_url` / `avatar_path`); when an image is replaced, delete the old object best-effort (log on failure, never fail the request). If the DB write fails after an upload, delete the new object.
- **Optional dependency.** With `SUPABASE_URL` unset the API starts with uploads disabled (503 on custom avatars) and no `storage` health check; when set, `/health` pings the bucket.
- **Manual uploads** until the admin panel exists: `make assets-upload` (`cmd/assetctl`) uploads every WebP under `frontend/public` to the same path and refuses the whole batch if any file is not WebP.

## Case data: never leak the solution

Anything a client can read about a case (`internal/cases`) comes from queries that list their columns explicitly and a response struct that has no field for solution data: `suspects.is_culprit` and any prompt/truth column are server-only (accusation evaluation, LLM prompts). Never `SELECT *` from `suspects` or `stories` for a client response, and add a test that the JSON does not contain "culprit". A player locked into one case may only read that case's dossier (403 otherwise); starting a case while locked into another is a 400.

## Email: never send mail from a handler

Handlers only **publish** an `email.Message` (`email.Publish(ctx, broker, msg)`) to `queue:email:outbound`; the separate **email worker** (`cmd/worker`, `make run-worker`) consumes it and sends through Brevo with `utils.Mailer`. This keeps slow SMTP calls off the request path. In the worker, return `*models.WorkerError` for temporary failures (retried with backoff) and `*models.DlqError` for permanent ones (bad payload, SMTP 5xx), so a bad address can never wedge it; exhausted retries land in `queue:email:dlq`. To add an email type: add a `Type…` constant and a case in `email/render.go`. Set `ExpiresAt` on messages that go stale (the worker drops them after it). The worker is a separate deployable (`backend/README.md`): never import worker-only code into the API or call SMTP from it. Dead letters are monitored by `email.Monitor` (ERROR log with `dlq_depth`) and handled with `emailctl` (`make emailctl-stats|list|requeue|purge`).

## Errors, startup and shutdown

- Never `panic` or call `os.Exit` in handlers or library code; return errors. Startup failures (config, database connections, migrations) are returned up to `run()` in `cmd/api/main.go`, which logs them and exits with status 1. Config and DB constructors return `(value, error)`.
- Configuration comes from `config.Config`. Pool sizes, timeouts and server settings are env-tunable (`PG_MAX_CONNS`, `PG_MIN_CONNS`, `MONGO_MAX_POOL_SIZE`, `REDIS_POOL_SIZE`, `DB_CONNECT_TIMEOUT_SECONDS`, `HTTP_ADDR`, `HTTP_WRITE_TIMEOUT_SECONDS`, `JWT_*`, `MIGRATE_ON_START`); add new tunables there with a default (use `envInt` with bounds), not as constants in feature code. Secrets such as `JWT_SECRET` have no default and must be validated at startup.
- The server runs with timeouts and drains in-flight requests on SIGINT/SIGTERM. Handlers must respect `r.Context()` so they stop promptly during shutdown.
- `GET /health` checks Postgres, MongoDB and Redis and returns 503 with per-dependency status when one is down. When you add a new dependency, register a check for it in `main.go`.

## Queues

Use `queue.RedisListBroker` for background work. A handler returns `*models.DlqError` to dead-letter immediately, `*models.WorkerError` to re-queue with rewritten data, or any other error to retry. Every message carries its own ID and attempt count; retries back off exponentially and after the configured attempts (default 5) the message goes to the DLQ. `Consume` returns `queue.ErrNoMessage` when nothing arrived; treat that as "poll again", not a failure. Handlers must be idempotent, since a message can be delivered more than once and a process crash mid-handler loses the in-flight message (Redis lists have no ack).

## Handler skeleton

```go
func (h *Handler) GetStories(w http.ResponseWriter, r *http.Request) {
	// no body to bind here; a handler with a body starts with utils.BindAndValidate
	stories, err := h.store.ListStories(r.Context()) // store wraps a sqlc query
	if err != nil {
		utils.LogErrorCtx(r.Context(), "list stories failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load stories", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, stories)
}
```

Register with Go 1.22 method patterns on the shared mux: `mux.HandleFunc("GET /api/stories", h.GetStories)`, or `requireAuth(h.GetStories)` for logged-in routes. Paths start with `/api/`, matching the frontend `src/api/` functions. Bind request bodies with `utils.BindAndValidate`, never with a bare `json.NewDecoder`.

## Layout and tests

Handlers go under `backend/internal/` (a package per domain, e.g. `internal/stories`) with `handler.go` and `store.go`; the handler depends on a small interface the store satisfies, which keeps handlers testable with fakes and avoids import cycles between domain packages. Pass `r.Context()` to DB and queue calls so cancellation and tracing work.

Add tests next to new code (`*_test.go`):
- Handlers: `httptest` with a fake store; `miniredis` for Redis-backed code.
- Stores and SQL: a test gated on `TEST_POSTGRES_URI` (see `internal/user/store_integration_test.go`) that runs the real migrations on a throwaway Postgres (`docker run --rm -d -p 55432:5432 -e POSTGRES_PASSWORD=pw postgres:18`). Add one whenever you add queries with non-trivial SQL (constraints, CTEs, atomic updates).

Checks before finishing: `cd backend && go build ./... && go vet ./... && go test ./... && go mod tidy -diff`, plus `make sqlc` and `make sqlc-diff` if anything under `sql/` changed. Then tell the frontend side the new path and response type so `src/api/` can be added (see the `alibi-api` skill).
