---
name: alibi-backend-api
description: How to create API endpoints in the Alibi Go backend (backend/). Use this skill whenever you add or change an HTTP handler, route, or endpoint (auth, dashboard, stories, chat, accusation, admin, health, anything served over HTTP), or write any code that reads request data or sends an HTTP response. Also use it for backend startup/config, database connections, queue workers, health checks and backend tests. All client input must be validated and rejected with an error response when invalid, all responses must go through the shared SuccessResponse/ErrorResponse JSON helpers, and every API must be logged with the logger functions.
---

# Creating API endpoints in the Go backend

Three rules apply to every endpoint: validate everything the client sends, respond only through the shared JSON helpers, and log every API through the logger. They exist so the frontend can rely on one response shape and so every request is observable in ClickHouse.

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
- Common tags: `required`, `email`, `min`, `max`, `oneof`, `uuid`. For anything custom (password strength, usernames), add a rule or check in `validate.go` instead of ad-hoc code in handlers.

**Path and query values:** there is no body to bind, so check them by hand and answer 400 through `ErrorResponse`: parse ints/UUIDs and reject failures, apply bounds to `limit`/`offset`, and allow-list sort or filter values. Example: `limit` outside 1–100 is a 400, not silently clamped.

**Beyond shape:** validity also means the request is allowed. Check ownership and state (the case belongs to this user, the user has tokens, no active case lock) and return 403, 404, 409 or 429 accordingly. Never take identity from the body; use the user ID from the validated JWT.

**LLM and chat input:** trim it, enforce a max length via `validate:"max=..."`, and run it through the moderation layer before spending a token.

Never echo raw invalid input back in a message. Keep `message` generic and put only field names and rules in `details`.

## 2. Responses: only `SuccessResponse` / `ErrorResponse`

Defined in `backend/pkg/utils/response.go`. Never call `json.NewEncoder(w)`, `w.Write`, `http.Error`, or set `Content-Type` by hand in a handler.

```go
utils.SuccessResponse(w, http.StatusOK, data)                       // {"status":"success","data":...}
utils.ErrorResponse(w, http.StatusBadRequest, "invalid body", nil)  // {"status":"error","message":"...","details":...}
```

- The frontend's `apiClient` unwraps exactly this envelope, so deviating breaks it.
- Use the right status: 200/201 success, 400 bad input, 401 unauthenticated, 403 forbidden, 404 missing, 409 conflict, 429 rate/limit (e.g. out of tokens), 500 unexpected.
- `message` is safe to show a user. Put field-level validation problems in `details` (e.g. `map[string]string{"email": "invalid"}`).
- Never leak internal errors (SQL, stack traces, driver text) in `message` or `details`. Log the real error, return a generic message.
- Return right after writing an error response so you don't write twice.

## 3. Logging: every API, via the logger

Use the logger functions in `pkg/utils`: `LogInfo` / `LogError`, and inside handlers the context-aware `LogInfoCtx(ctx, ...)` / `LogErrorCtx(ctx, ...)`. They all go through `slog`'s default logger, which `logger.Init` points at OpenTelemetry, so every record is exported to ClickHouse. The `Ctx` variants also link the record to the request's trace span. Do not use `fmt.Println`, the stdlib `log` package, or your own `slog.New(...)` logger, because those bypass the pipeline.

- **Request and response logging is automatic** (method, path, IP, status, latency, trace span) as long as the route is served through `middleware.Logging`. Register every route on the mux that is wrapped by it in `cmd/api`; never serve a route outside it.
- **Inside handlers,** log what the middleware can't know, with key/value pairs:
  - `LogInfoCtx` for meaningful business events: `utils.LogInfoCtx(r.Context(), "token deducted", "user_id", uid, "remaining", n)`.
  - `LogErrorCtx` for every failure path, with the real error, then respond with a generic `ErrorResponse`.
  - Always log before returning a 5xx. Log 4xx only when useful (auth failures, suspicious input).
- Use stable snake_case keys (`user_id`, `case_id`, `err`). Never log secrets: passwords, JWTs, API keys, full request bodies.

## Errors, startup and shutdown

- Never `panic` or call `os.Exit` in handlers or library code; return errors. Startup failures (config, database connections) are returned up to `run()` in `cmd/api/main.go`, which logs them and exits with status 1. Config and DB constructors return `(value, error)`.
- Configuration comes from `config.Config`. Pool sizes and timeouts are env-tunable (`PG_MAX_CONNS`, `PG_MIN_CONNS`, `MONGO_MAX_POOL_SIZE`, `REDIS_POOL_SIZE`, `DB_CONNECT_TIMEOUT_SECONDS`); add new tunables there with a default, not as constants in feature code.
- The server runs with timeouts and drains in-flight requests on SIGINT/SIGTERM. Handlers must respect `r.Context()` so they stop promptly during shutdown.
- `GET /health` checks Postgres, MongoDB and Redis and returns 503 with per-dependency status when one is down. When you add a new dependency, register a check for it in `main.go`.

## Queues

Use `queue.RedisListBroker` for background work. A handler returns `*models.DlqError` to dead-letter immediately, `*models.WorkerError` to re-queue with rewritten data, or any other error to retry. Retries are counted and back off exponentially; after the configured attempts (default 5) the message goes to the DLQ. `Consume` returns `queue.ErrNoMessage` when nothing arrived; treat that as "poll again", not a failure. Handlers must be idempotent, since a message can be delivered more than once and a process crash mid-handler loses the in-flight message (Redis lists have no ack).

## Handler skeleton

```go
func (h *Handler) GetStories(w http.ResponseWriter, r *http.Request) {
	// no body to bind here; a handler with a body starts with utils.BindAndValidate
	stories, err := h.store.ListStories(r.Context())
	if err != nil {
		utils.LogErrorCtx(r.Context(), "list stories failed", "err", err)
		utils.ErrorResponse(w, http.StatusInternalServerError, "could not load stories", nil)
		return
	}
	utils.SuccessResponse(w, http.StatusOK, stories)
}
```

Register with Go 1.22 method patterns on the shared mux: `mux.HandleFunc("GET /api/stories", h.GetStories)`. Paths start with `/api/`, matching the frontend `src/api/` functions. Bind request bodies with `utils.BindAndValidate`, never with a bare `json.NewDecoder`.

## Layout

Handlers go under `backend/internal/` (a package per domain, e.g. `internal/stories`), with database access kept out of the handler where practical. Pass `r.Context()` to DB and queue calls so cancellation and tracing work.

Add tests next to new code (`*_test.go`; `miniredis` is available for Redis-backed code). After changes run `cd backend && go build ./... && go vet ./... && go test ./...`, and tell the frontend side the new path and response type so `src/api/` can be added (see the `alibi-api` skill).
