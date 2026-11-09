# Backend

Two services share this module and one Docker image build (`backend/Dockerfile`):

| Service | Command | Does |
|---|---|---|
| **api** | `cmd/api` | HTTP API (auth, profile, ...). Publishes email jobs; never talks to SMTP. |
| **email-worker** | `cmd/worker` | Consumes `queue:email:outbound` from Redis and sends via Brevo SMTP. No database, no JWT secret. |

They communicate only through Redis (`queue.RedisListBroker`), so each can be restarted, deployed and scaled on its own.

## Run

```
# locally, two terminals (needs infra: docker compose up -d postgres mongo redis)
make run-api
make run-worker

# everything in containers (infra + api + worker)
make docker-up                 # WORKERS=3 make docker-up   to run three workers
make docker-logs
make docker-down
```

Secrets come from the repo-root `.env` (see `.env.example`). In compose each service gets an explicit allow-list of variables (the API never sees `BREVO_*`, the worker never sees `JWT_SECRET` or the database URIs), and the hostnames are overridden so containers reach each other by service name. `api` and `email-worker` wait for healthy Postgres / MongoDB / Redis before starting.

## Client IPs and `TRUSTED_PROXIES`

Rate limits are keyed by client IP. `X-Forwarded-For` / `X-Real-IP` are only believed when the TCP peer is in `TRUSTED_PROXIES`, which **defaults to loopback only**. Private ranges are intentionally not trusted by default: with Docker's published ports every external client reaches the API from the bridge gateway (`172.x`), so trusting `172.16.0.0/12` would let anyone choose their own IP with a forged header and bypass the per-IP limits.

In production set `TRUSTED_PROXIES` to the exact addresses/CIDRs of your reverse proxy or load balancer (`none` = never believe forwarding headers). IPv6 clients are limited per `/64`, since one subscriber owns a whole `/64`.

## Auth abuse protection

| What | Protection |
|---|---|
| OTP requests (`signup`, `resend-otp`) | per email 60 s cooldown + 5/day, per IP 10 per 15 min (`OTP_*`) |
| Cheap-to-abuse endpoints (`signup` availability check, `resend-otp`, `verify-otp`, `login`, `refresh`) | per-IP budget charged *before* any DB/bcrypt work: `AUTH_IP_MAX` per `AUTH_IP_WINDOW_MINUTES` (429 + `Retry-After`) |
| Wrong OTP codes | counted atomically in one Redis script *before* comparing, so parallel guesses cannot exceed `OTP_MAX_ATTEMPTS`; only a hash (HMAC) of the code is stored in Redis |
| Login | after `LOGIN_MAX_FAILURES` wrong passwords per email (20 per IP) login is refused with 429, with a lockout that doubles per extra failure (1 min up to 1 h); failures are logged as `login_failed` (hashed email + IP) |
| bcrypt CPU | at most `BCRYPT_WORKERS` (default: CPU count) run at once; extra requests queue for up to 5 s, then get 503 |
| Account enumeration | `resend-otp` answers identically for unknown and pending signups. `signup` still reports "already registered / taken" (needed so users can pick another), limited by the per-IP budget |
| Avatar uploads | max 4096 px / 8 MP (rejected from the image header), 3 concurrent decodes (503 when busy), 5 uploads per user per hour |

## Email delivery and the dead-letter queue

1. `signup` / `resend-otp` store the OTP in Redis and push `{to, otp, type, expires_at}` onto `queue:email:outbound`.
2. A worker pops it and sends. Temporary failures (network, SMTP 4xx) are retried with exponential backoff (500 ms doubling, max 30 s, 5 attempts). Permanent ones (bad payload, SMTP 5xx) skip the retries.
3. Either way a message that cannot be delivered ends in `queue:email:dlq` instead of blocking the worker. A message whose code has already expired (`expires_at`) is dropped, not sent.
4. Several consumers per process (`WORKER_CONCURRENCY`, default 1) and several processes are safe; each message is popped by exactly one.
5. An idle consumer sends one `BRPOP` per `WORKER_POP_TIMEOUT_SECONDS` (default 30) and still picks a message up the instant it arrives. On a Redis billed per command (Upstash) every idle consumer costs about 86,400 commands a month at 30 s, so keep consumers few and the timeout long. Shutdown can wait up to one timeout, so give the container a longer stop grace period. The chat storage consumers in the API use `CHAT_WORKER_CONCURRENCY` and `CHAT_WORKER_POP_TIMEOUT_SECONDS` the same way.

### Watching it

- Every `WORKER_MONITOR_SECONDS` (default 900) the worker logs queue depths. A DLQ that grows is logged at **ERROR** ("email dead-letter queue is not empty", `dlq_depth`), re-reminded every 5 minutes while non-empty, and a backlog over 100 pending is logged at WARN (`queue_pending`). These records go to ClickHouse through OpenTelemetry; build the alert on `dlq_depth > 0`.
- `GET :8081/health` on each worker returns 200 with `{"redis":"ok","queues":{"pending":N,"dead_letter":N}}` (503 if Redis is down). Compose uses it as the container healthcheck.

### Operating it

```
make emailctl-stats     # pending / dead-letter counts
make emailctl-list      # dead-lettered emails (codes are masked)
make emailctl-requeue   # give them another full round of retries (after fixing the cause)
make emailctl-purge     # throw them away
# in a container: docker compose exec email-worker emailctl stats
```

The DLQ is capped at 1000 entries (oldest dropped first), and the worker's monitor removes messages past their `expires_at` from both queues, so addresses and dead codes do not linger in Redis. `requeue all` repeats until the DLQ is empty; `emailctl` only needs `REDIS_URI`. The move is one atomic Lua script.

Dead-lettered signup codes are only valid for 10 minutes, so after an outage most old entries should be purged rather than requeued (a requeued one past its `expires_at` is dropped by the worker anyway).

## Database

See `sql/README.md` (goose migrations + sqlc queries).

Migrations run under a Postgres advisory lock (`goose.WithSessionLocker`), so several API replicas starting together apply each one exactly once. For large changes you can also set `MIGRATE_ON_START=false` and run `make migrate-up` as a one-off job before a rollout.

`TEST_POSTGRES_URI` (store integration test) only needs a server where the user may `CREATE DATABASE`; the test works in a throwaway `alibi_test_<random>` database and drops it afterwards.
