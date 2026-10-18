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
| Load dev test data (stories, leaderboard users) | `make seed-dev` (docker-compose DB only; see `sql/seed/`, never a migration) |

`make migrate-*` use `POSTGRES_URI` from the repo-root `.env`. The API also runs pending migrations at startup unless `MIGRATE_ON_START=false`.

Rules:
- Never edit a migration that has shipped; add a new numbered one.
- Never write SQL strings in Go. Add a named query and call `dbgen.Queries`.
- List columns explicitly when a table has large ones. (Images are never stored in Postgres: they live in the object store, see `internal/storage`.)

## Migrations

| File | What it does |
|---|---|
| `00001_create_users.sql` | `users`: credentials, default avatar, trophies, `token_balance` (source of truth, CHECK >= 0), case-insensitive unique email/username |
| `00002_create_user_avatars.sql` | `user_avatars`: custom avatar JPEG as `BYTEA` (dropped by 00008) |
| `00005_create_stories.sql` | `stories`: crime story card metadata (title, synopsis, thumbnail, `easy`/`medium`/`hard`, trophy and token rewards); only `is_published` rows reach the dashboard |
| `00006_create_active_case_locks.sql` | `active_case_locks`: one row per user inside an unfinished case (the primary key on `user_id` is the lock) |
| `00007_add_users_cases_solved.sql` | `users.cases_solved`: how many cases the player has solved (CHECK >= 0); whatever finishes a case increments it |
| `00008_avatars_in_object_storage.sql` | custom avatars move to the object store: `users.avatar_url` (public URL) + `avatar_path` (object path, for deleting a replaced image); drops `user_avatars` and `avatar_updated_at` |
| `00009_create_case_briefing.sql` | `stories.main_plot`; `case_scenes` (image URL + display order, per case); `suspects` (role, avatar, initial testimony, server-only `is_culprit`, at most one culprit per case) |
| `00010_create_case_notes.sql` | `case_notes`: a player's private notes per case, optionally about a suspect (`ON DELETE SET NULL`), body 1-1000 chars |
| `00016_accusation_trophies.sql` | `stories.penalty_trophies` (trophies lost on a wrong accusation) and `stories.resolution` (closing text); with `reward_trophies` these are set per case by the admin |
| `00017_case_results.sql` | `case_results`: one row per player per case, written by the accusation transaction. Closes the case (starting it again is a 409) and lets `GET /api/cases/{id}/result` show the verdict after a reload |
| `00018_case_hints.sql` | `stories.hints` (up to 3 admin-written strings, server only until revealed), `active_case_locks.hints_used` (0-3) and `case_results.hint_penalty` |

There are no custom SQL functions or stored procedures; `gen_random_uuid()` is built into PostgreSQL 13+. The `alibi` database itself is created by `docker-compose.yml`.

## Queries (`sql/queries/`)

| File | Query | Purpose |
|---|---|---|
| `users.sql` | `CreateUser` (with starting `token_balance`), `GetUserByEmail`, `GetUserByID`, `CheckAvailability` | OTP verification creates the user; login; profile; signup checks email/username are free before sending an OTP |
| `avatars.sql` | `SetDefaultAvatar`, `SetCustomAvatar` | switch between a default avatar and an uploaded image; both return the replaced image's object path so it can be deleted from storage |
| `cases.sql` | `GetCaseBriefing`, `ListCaseScenes`, `ListCaseSuspects` (explicit columns, no `is_culprit`), `GetUserLock`, `CaseIsPublished` | the case briefing endpoints and starting a case (uses `LockUserToCase`) |
| `notes.sql` | `ListCaseNotes`, `CreateCaseNote` (refuses past the per-case cap), `SuspectInCase`, `DeleteCaseNote` (owner only) | the player's case notes |
| `dashboard.sql` | `ListStories`, `GetUserStats`, `GetUsernames`, `TopPlayers`, `GetActiveCase` (with the story title), `LockUserToCase`, `ReleaseCaseLock` | dashboard stories, stats, leaderboard (and its Postgres fallback) and the active-case lock |
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
| `leaderboard` | ZSET: member = user ID, score = trophies. The dashboard reads it; `POST /api/cases/{id}/accuse` `ZINCRBY`s it by the exact trophy change after the Postgres commit (best effort) | none |
| `leaderboard:synced` | Marker: the ZSET was rebuilt from Postgres (top 1000) within the last 5 minutes. Without it the next dashboard leaderboard read serves Postgres and rebuilds the set | 5 min |
| `user:coins:{user_id}` | cache of `users.token_balance`; filled on read, deleted on every change | 1 hour |
| `queue:email:outbound` | list of email jobs `{to, otp, type}` (API publishes, worker consumes) | none |
| `queue:email:dlq` | emails that failed permanently or ran out of retries | none |
