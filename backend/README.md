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

Secrets come from the repo-root `.env` (see `.env.example`); compose overrides the hostnames so containers reach each other by service name.

## Email delivery and the dead-letter queue

1. `signup` / `resend-otp` store the OTP in Redis and push `{to, otp, type, expires_at}` onto `queue:email:outbound`.
2. A worker pops it and sends. Temporary failures (network, SMTP 4xx) are retried with exponential backoff (500 ms doubling, max 30 s, 5 attempts). Permanent ones (bad payload, SMTP 5xx) skip the retries.
3. Either way a message that cannot be delivered ends in `queue:email:dlq` instead of blocking the worker. A message whose code has already expired (`expires_at`) is dropped, not sent.
4. Several consumers per process (`WORKER_CONCURRENCY`) and several processes are safe; each message is popped by exactly one.

### Watching it

- Every `WORKER_MONITOR_SECONDS` the worker logs queue depths. A DLQ that grows is logged at **ERROR** ("email dead-letter queue is not empty", `dlq_depth`), re-reminded every 5 minutes while non-empty, and a backlog over 100 pending is logged at WARN (`queue_pending`). These records go to ClickHouse through OpenTelemetry; build the alert on `dlq_depth > 0`.
- `GET :8081/health` on each worker returns 200 with `{"redis":"ok","queues":{"pending":N,"dead_letter":N}}` (503 if Redis is down). Compose uses it as the container healthcheck.

### Operating it

```
make emailctl-stats     # pending / dead-letter counts
make emailctl-list      # dead-lettered emails (codes are masked)
make emailctl-requeue   # give them another full round of retries (after fixing the cause)
make emailctl-purge     # throw them away
# in a container: docker compose exec email-worker emailctl stats
```

Dead-lettered signup codes are only valid for 10 minutes, so after an outage most old entries should be purged rather than requeued (a requeued one past its `expires_at` is dropped by the worker anyway).

## Database

See `sql/README.md` (goose migrations + sqlc queries).
