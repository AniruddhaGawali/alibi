# SQL

All PostgreSQL schema and queries live here. Two tools, both required:

- **goose** applies migrations (`sql/migrations/`).
- **sqlc** turns queries (`sql/queries/`) into type-safe Go in `internal/db/dbgen/` (generated, committed, never edited by hand).

sqlc reads the migrations as its schema (it ignores `-- +goose Down`), so there is no separate schema file to keep in sync.

## Install the tools (once)

```
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
```

## Daily workflow (run from `backend/`)

| Task | Command |
|---|---|
| Change the schema | `make migrate-new name=add_cases`, write `Up` **and** `Down`, then `make migrate-up` |
| Add or change a query | edit a file in `sql/queries/`, then `make sqlc` |
| Check generated code is current (CI) | `make sqlc-diff` |
| Roll back / inspect | `make migrate-down`, `make migrate-status` |

`make migrate-*` use `POSTGRES_URI` from the repo-root `.env`. The API also runs pending migrations at startup unless `MIGRATE_ON_START=false`.

Rules:
- Never edit a migration that has shipped; add a new numbered one.
- Never write SQL strings in Go. Add a named query and call `dbgen.Queries`.
- List columns explicitly when a table has large ones. (Avatar bytes live in their own table so `SELECT * FROM users` is safe.)

## Migrations

| File | What it does |
|---|---|
| `00001_create_users.sql` | `users`: credentials, default avatar, trophies, `token_balance` (source of truth, CHECK >= 0), case-insensitive unique email/username |
| `00002_create_user_avatars.sql` | `user_avatars`: custom avatar JPEG as `BYTEA`, one row per user |

There are no custom SQL functions or stored procedures; `gen_random_uuid()` is built into PostgreSQL 13+. The `alibi` database itself is created by `docker-compose.yml`.

## Queries (`sql/queries/`)

| File | Query | Purpose |
|---|---|---|
| `users.sql` | `CreateUser` (with starting `token_balance`), `GetUserByEmail`, `GetUserByID`, `CheckAvailability` | OTP verification creates the user; login; profile; signup checks email/username are free before sending an OTP |
| `avatars.sql` | `SetDefaultAvatar`, `SetCustomAvatar`, `GetAvatar` | choose a default avatar (deletes the custom image), store/replace the custom image, serve it |
| `wallet.sql` | `GetTokenBalance`, `AdjustTokenBalance` | read the balance; atomic grant/spend that cannot go below zero |

## Redis keys (not SQL, listed for completeness)

| Key | Value | TTL |
|---|---|---|
| `otp:signup:{email}` | JSON `{otp, username, hashed_password}`: the pending signup (no Postgres row yet) | 10 min |
| `otpattempts:signup:{email}` | wrong-code counter; the session is discarded at 5 | 10 min |
| `ratelimit:otp:email:{email}` | `1`; cooldown between codes sent to one email | 60 s |
| `ratelimit:otpdaily:email:{email}` | counter; at most 5 codes per email per day | 24 h |
| `ratelimit:otp:ip:{ip}` | counter; at most 10 codes per client IP per window | 15 min |
| `blacklist:jwt:{jti}` | `true`; logged-out access and refresh tokens | remaining lifetime of that token |
| `user:coins:{user_id}` | cache of `users.token_balance`; filled on read, deleted on every change | 1 hour |
| `queue:email:outbound` | list of email jobs `{to, otp, type}` (API publishes, worker consumes) | none |
| `queue:email:dlq` | emails that failed permanently or ran out of retries | none |
