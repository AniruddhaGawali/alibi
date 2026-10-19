# AI Detective Mystery Game - Development Phases & TODO List

## Phase 0: Project Initialization & Setup (P0)

### P0.1 Backend Setup (Go)
- [x] Initialize Go module and define project directory structure (`cmd`, `internal`, `pkg`, `api`).
- [x] Set up database connection pools for PostgreSQL (Supabase), MongoDB, and Redis (Upstash).
- [x] Configure the Message Queue (Redis Lists or RabbitMQ).
- [x] **Initialize LLM Clients:** Install and configure the official Ollama Go client (`github.com/ollama/ollama/api`) for local model testing and the OpenRouter Go SDK (`github.com/OpenRouterTeam/go-sdk`) for cloud model access.
- [x] Create basic backend utility functions (error handling, structured logging, environment variable management).

### P0.2 Frontend Setup (React)
- [x] Initialize React project (using Vite).
- [x] Set up client-side routing (e.g., React Router).
- [x] **State & Data Management:** Install and configure **Zustand** for lightweight client-side state management (UI toggles, user session) and **React Query** for server state management, caching, and data fetching (token balances, chat history).
- [x] Establish the base UI theme, layout wrappers, and core reusable components (buttons, modals, inputs).

### P0.3 Shared Helpers & Utilities
- [x] **Frontend:** Create an Axios/Fetch API wrapper instance with request/response interceptors for injecting JWTs.
- [x] **Frontend:** Build token management helper functions (store/remove JWT in local storage or secure cookies).
- [x] **Backend:** Write helper functions for standardizing API JSON responses.

---

## Phase 1: Onboarding and Authentication (P1)

### P1.1 Auth Backend (Go)
- [x] Custom email/password signup with OTP verification (`signup` -> `verify-otp`, `resend-otp`); no Postgres row until the code is verified. Local PostgreSQL for now, Supabase later.
- [ ] Google Login.
- [x] Create the user profile record in PostgreSQL upon successful OTP verification (starting balance of 10 question tokens, 0 trophies).
- [x] Implement JWT generation, parsing, and validation middleware (deny-by-default `auth.Middleware`, public-route allow-list).
- [x] Initialize the Redis JWT Blacklist logic for secure logouts (`blacklist:jwt:<jti>`, access and refresh tokens, remaining-TTL expiry).
- [x] Refresh-token rotation (each refresh token works once) and login.
- [x] Dual-layer OTP rate limiting in Redis (429 + `Retry-After`): per email a 60 s cooldown and 5 codes/day; per client IP 10 codes per 15 min (so shared networks still work); wrong-code attempt cap; trusted-proxy aware client IP.
- [x] Auth abuse hardening (PR #2 review): atomic wrong-OTP cap and HMAC-only code storage, per-IP budget before DB/bcrypt work, login lockout with exponential backoff + bcrypt concurrency bound, IPv6 /64 keys, loopback-only default `TRUSTED_PROXIES`, avatar decode/upload limits and conditional avatar GETs, migration advisory lock, BIGINT token balance, least-privilege compose env + healthchecks. See `backend/README.md`.
- [ ] Bot protection (Cloudflare Turnstile / hCaptcha) on signup and resend-otp (needs a frontend widget and a secret key).
- [x] User profile (`GET /api/user/profile`) and avatar (`POST /api/user/avatar`: default preset 1-4 or custom image, max 1 MB, resized to 256x256 WebP and uploaded to object storage; the profile returns its public URL).
- [x] Image storage behind `storage.Provider` (`internal/storage`), Supabase Storage for now (swappable for S3/GCS), WebP only; `make assets-upload` for seeded case/avatar art; the admin panel uploads new case art through `POST /api/admin/upload`.
- [x] Supabase project + public bucket `alibi-assets` set up, `SUPABASE_*` in `.env`, `VITE_ASSET_BASE_URL` in `frontend/.env.local`; case and avatar art uploaded with `make assets-upload`, local copies removed from `frontend/public`.
- [x] Token balance: Postgres is the source of truth, Redis `user:coins:<id>` is the cache (`wallet` service).
- [x] Database workflow: goose migrations + sqlc type-safe queries (`backend/sql`).

### P1.1b Email Delivery (Go, async)
- [x] Brevo SMTP helper in `pkg/utils/email.go` (`net/smtp`, STARTTLS/implicit TLS, HTML + plain text, context timeout).
- [x] API publishes OTP emails to `queue:email:outbound` instead of sending synchronously.
- [x] Separate email worker process (`cmd/worker`, `make run-worker`) consumes the queue and sends via Brevo.
- [x] Failures: temporary SMTP/network errors return `WorkerError` (retry with backoff); permanent rejections and bad payloads go straight to the DLQ `queue:email:dlq`; exhausted retries are dead-lettered.
- [x] Run the worker next to the API as its own service: one Dockerfile with `api` and `worker` targets, compose services under `--profile app` (scalable: `--scale email-worker=3`), healthchecks, restart policy, graceful shutdown. See `backend/README.md`.
- [x] Worker concurrency (`WORKER_CONCURRENCY`), `GET /health` on the worker, and dropping OTP emails whose code already expired.
- [x] DLQ monitoring: queue depths logged on an interval (ERROR when the DLQ grows, with reminders), and `emailctl` (stats / list / requeue / purge) for inspection and recovery.
- [ ] Create the actual alert on `dlq_depth > 0` in the log/ClickHouse tooling, and set the real Brevo credentials and verified sender in production.

### P1.2 Auth Frontend (React)
- [x] Build the Signup form (Email/Username/Password) and a Google Login button (placeholder; backend Google login is still open).
- [x] Build the OTP verification screen (shadcn `InputOTP`, resend code).
- [x] Build the Login screen for returning users.
- [x] Build the Avatar Selection UI for the onboarding flow (1 MB client-side check, 4 placeholder defaults).

### P1.3 Auth Integration
- [x] Connect frontend auth forms to the Go backend (`src/api/auth.ts`, `user.ts`; tokens, `pending_otp_email` and `user_data` in cookies). Not yet exercised against a running backend.
- [x] Handle routing post-login (redirect to Dashboard).
- [x] Implement logic to automatically allocate the initial 10 question tokens to the user's wallet upon completing onboarding. (Backend: allocated when the OTP is verified; frontend just displays it.)
- [x] Connect the frontend logout button to the backend Redis Blacklist endpoint.

---

## Phase 2: The Home Page & Case Selection (P2)

### P2.1 Dashboard Backend (Go)
- [x] Create API endpoint to fetch available crime stories from PostgreSQL.
- [x] Create API endpoint to fetch the user's current token balance and total trophies.
- [x] Create API endpoint to fetch the top players (`GET /api/dashboard/leaderboard`, read from Postgres through `users_trophies_rank_idx`; rows carry `is_me`, not user ids). A Redis ZSET can come back in Phase 5 if it is maintained and backfilled wherever trophies change.
- [x] Implement "Active Case Lock" logic to verify if a user is currently locked into an unfinished case.

### P2.2 Dashboard Frontend (React)
- [x] Build Crime Story Cards displaying synopsis, title, difficulty, and rewards.
- [x] Build the live Token Balance and Trophies display component.
- [x] Build the Global Leaderboard component.

### P2.3 Case Selection Integration
- [x] Connect the dashboard UI to fetch live data from the backend using React Query.
- [x] Implement frontend routing restrictions based on the "Active Case Lock" (preventing users from clicking new stories if one is active). The active story id comes from `GET /api/dashboard/stats` (`active_story_id`): that card shows Continue, the rest are Locked (redacted, CLASSIFIED stamp, no button).

---

## Phase 3: The Story and Briefing Page (P3)

### P3.1 Briefing Backend (Go)
- [x] Create API endpoint to fetch the specific active case's main plot and scene images. (`GET /api/cases/{id}/briefing`; case = a `stories` row with `main_plot` and `case_scenes`.)
- [x] Create API endpoint to fetch all suspect avatars and their initial testimonies for the active case. (`GET /api/cases/{id}/suspects`; `is_culprit` is never selected.)
- [x] Start a case: `POST /api/cases/{id}/start` takes the Active Case Lock (400 "Finish your active case first" if locked into another case; idempotent for the same case). The dashboard Start / Continue buttons call it and open `/case/:id`.
- [x] Case #01 seeded (plot, 3 scenes, 4 suspects, Beatrice as culprit) in `backend/sql/seed/dev_seed.sql`; testimonies are drafts to review.

### P3.2 Briefing Frontend (React)
- [x] Build the Story Dashboard UI to render the main plot and visuals. (`/case/:id`: title, framed scene gallery beside the letter-shaped case file.)
- [x] Suspect cards in a horizontal carousel show avatar, name, role and the initial testimony on the card; hovering the portrait also shows the testimony in a tooltip.
- [x] Chat button on each suspect card opens the floating chat window (Zustand `useChatStore`) with that suspect pre-selected; the window shows the suspect's details and a placeholder where messages go in P3.3.

### P3.3 Chat History UI (React)
- [x] Build the global chat history section on the main story dashboard. (`ChatHistorySection` reads the stored exchanges through `GET /api/chat/{case_id}/{suspect_id}`, one request per suspect, via `useCaseChatHistory` in `src/api/chat.ts`.)
- [x] Implement filtering logic so clicking a specific character shows only their chat previews. (Filter pills, and clicking a suspect card filters and scrolls to the logs; `selectedSuspectFilter` in `useGameStore`.)
- [x] Detective's notes live inside the interrogation log (`NoteComposer` + `NoteCard`): an "Add a note" box above the logs, and notes appear in the same scrolling stream as the logs, newest first.
- [x] Case notes are saved to the player's account: `case_notes` table (migration 00010) and `GET/POST /api/cases/{id}/notes`, `DELETE /api/cases/{id}/notes/{note_id}` (own notes only, max 1000 chars, max 200 per case), used through `createApi` hooks.
- [x] A note can be about one suspect (`suspect_id`) or the case in general: with a suspect filter selected the note is about that suspect, under All Logs a dropdown picks who. Notes show under the matching filter.
- [x] Tie a note to a specific interrogation log entry: "Add note" on each log card saves a note with `chat_message_id` (`case_notes.chat_message_id`, migration 00012). The API checks the entry is the caller's own, from this case, and takes the note's suspect from it; the note card quotes the question it is about.

---

## Phase 4: The Interrogation (Core Gameplay Loop) (P4)

### P4.1 LLM Backend Integration (Go)
- [x] Set up the LLM Microservice to connect with the Ollama/OpenRouter APIs. OpenRouter answers interrogations (`internal/llm`, `internal/interrogation`, `POST /api/cases/{id}/suspects/{suspect_id}/interrogate`); Ollama stays a local-only client.
- [x] Per-case model: `stories.llm_model` (default `google/gemma-4-31b-it`), set by admins through `POST /api/admin/cases` and `PATCH /api/admin/cases/{id}` (needs the admin role, `users.role`). Model ids are format-checked (no `:online`, no `openrouter/*` routers) and must be text-output-only on OpenRouter.
- [x] Text-to-text only: no tools or tool choice, web search and file parser plugins switched off, output modality `text`; tool calls, images or audio in a reply are discarded (the token is kept: the provider has already generated the reply).
- [x] Implement the input moderation layer to detect and block off-topic queries (e.g., code, math, "bubble sort"), plus prompt-injection attempts (`llm.Moderate`; blocked questions get a fixed in-character reply and cost no token). Matching runs on a normalised copy (NFKD, accents and variation selectors dropped, Cyrillic/Greek look-alikes folded, leetspeak and spelled-out words rejoined) and knows the chat-template tokens of ChatML, Gemma, Llama and Phi. This is a speed bump, not the whole defence (NFR2.1): every blocked injection counts as a strike (`LLM_INJECTION_STRIKES_PER_HOUR`, then a lockout), each suspect reply is checked for leaked secrets (`llm.LeaksSecret`: 8 words in a row from the hidden truth or confession triggers, or quoted rules) and replaced with a fixed line, and only the questioned suspect's secrets are loaded and put in the prompt. The confession is gated on the server: each suspect has `confession_keywords` (migration 00015, groups like `last+study`); until a question (or one of the last 6) matches, the prompt holds no hidden truth, no confession triggers and no culprit flag, and every suspect gets the same neutral "you are hiding something" line, so a successful extraction reveals nothing and guilty and innocent look alike. A suspect without keywords is never gated.
- [x] Build the dynamic System Prompt swapping logic based on the requested character ID (Dual-State prompting for culprits). Three sections: persona with public alibi / hidden truth / confession triggers, cross-character awareness (other suspects' public facts only, never their secrets), strict rules (length, stay in character, injection defence).
- [x] Optimize the request chain to ensure AI responses return within the 2-3 second NFR limit. No SDK retries, 160-token reply cap, `LLM_TIMEOUT_SECONDS` (default 20), the lock, history and (cached) case/suspect/prompt reads run concurrently, a Redis balance check comes before any database read, the system prompts are built once per case and suspect so providers can cache their prefix, and the chat reply no longer waits for a stats refetch. Measured live against `google/gemma-4-31b-it` through OpenRouter (with `data_collection: deny`): 1.6 to 2.5 s per answer, one of five calls took 3.1 s. Streaming is a possible later improvement.

### P4.2 Token Economy Backend (Go)
- [x] Implement the 1-token deduction executed *before* calling the LLM (after moderation and a cheap balance check): `wallet.SpendQuestion` runs an atomic Redis DECR on `user:coins:<id>` that is undone straight away if it goes below zero (429 "Out of tokens"), then mirrors the spend to Postgres; on any disagreement the key is dropped and reloaded from Postgres.
- [x] Implement the token refund logic. The model call no longer depends on the client connection (a dropped request still completes and its answer is stored), and only a failure before any answer (timeout, upstream error) is refunded (`wallet.RefundQuestion`: Postgres, then Redis INCR), at most `LLM_REFUNDS_PER_DAY` times per player per day; an unusable answer keeps the token. The API only says "refunded" when the refund worked. Moderation runs before the spend, so a blocked query never costs a token.
- [x] Create the daily cron job to reset user token balances to 10 (without rolling over unused tokens): `internal/jobs` with robfig/cron inside the API process; the allowance, time and zone are in `app_settings` (edited in the admin panel, P5.3; `DAILY_TOKENS` / `TOKEN_RESET_TIME` / `TOKEN_RESET_TIMEZONE` only seed it). A finished day is recorded in `done:cron:token-reset:<day>` and a short in-progress lock (released when the run ends, expiring after 3 minutes if the process dies) keeps replicas from overlapping, so a crash never skips the day; a check at startup and every minute re-reads the schedule and runs today's reset if it is due and missing, so a missed or failed run heals itself. Postgres is set to exactly the daily allowance, then the Redis balance keys are removed with UNLINK.
- [x] Protect the OpenRouter credit (`interrogation.RedisGuard`, `LLM_*` settings): a per-address and a per-player requests-per-minute limit that also counts blocked questions, one question in flight per player, a never-refunded daily model-call allowance per player (`DAILY_TOKENS` + refunds), a global concurrency cap, daily call and token budgets fed by the usage OpenRouter reports, and a circuit breaker for upstream failures (429 with `Retry-After`, or 503). Accounts are capped per address (`SIGNUP_IP_DAILY_MAX`) and the OTP limits treat `+tag` and Gmail-dot variants as one mailbox. The OpenRouter key itself has a $2 hard credit limit. Requests ask OpenRouter for providers that do not log prompts (`data_collection: deny`).

### P4.3 Chat Storage Backend (Go)
- [x] Configure the LLM service to asynchronously push the generated Q&A payload to the Message Queue (`queue:chat:events`, `chat.Event`: user_id, case_id, suspect_id, user_query, ai_response, timestamp, plus event_id, play_id, suspect_name, model).
- [x] Build the background worker (Chat Storage Microservice) to consume queue events and insert documents into MongoDB (`chat.Worker`, goroutines in the API process, `CHAT_WORKER_CONCURRENCY`; collection `chat_messages`, `_id` = event id so redeliveries are no-ops; retries and `queue:chat:dlq` from the broker).
- [x] Create API endpoint to fetch historical character chats from MongoDB: `GET /api/chat/{case_id}/{suspect_id}`, the caller's own exchanges oldest first (latest 200), from the play-through they are in now (`play_id` of their case lock; a replayed case starts with a blank conversation, and the model's memory is scoped the same way). The last 6 exchanges are also fed back to the model so suspects remember the conversation.
- [x] Exchanges that the queue refuses are retried (3 attempts, then kept in a small in-memory list and tried again with the next publish and on shutdown), and `chat.Monitor` logs the queue and DLQ depths (`chat_dlq_depth`) like the email monitor.

### P4.4 Chat Frontend (React)
- [x] Build the real-time chat interface (message bubbles, input field, loading states): `components/chat/` (`ChatWindow`, `MessageBubble`, `ThinkingBubble`, `ChatInput`, `TokenCounter`). Detective bubbles are modern and right-aligned, suspects are aged paper in typewriter type on the left, errors are centred notices; "Suspect is thinking…" shows the moment a question is sent; the window follows new messages (`useStickToBottom`) and earlier conversation loads from `GET /api/chat/{case_id}/{suspect_id}`.
- [x] Connect the chat UI to the LLM backend via standard HTTP: `useInterrogateMutation` (`api/chat.ts`) posts to `POST /api/cases/{id}/suspects/{suspect_id}/interrogate` (the built route, not `/api/chat`). A moderation block (422) shows the suspect's in-character brush-off; timeouts and failures show a notice; answers are added to the cached history so the chat and the case log update at once.
- [x] Integrate the visual token counter to update instantly when a message is sent: `useGameStore.tokenBalance` is the one balance (kept in step with the server by `useSyncTokenBalance` in the layout), drops by 1 on send, is refunded on an error and set to the server's figure on success. The navbar menu, the stats panel and the chat all read it.
- [x] Add state logic to disable and visually block the chat input when the token balance hits zero (disabled, "Out of tokens. Make your final deduction.", red border, greyed field, not-allowed cursor).
- [x] Show testimonies and chat logs in the typewriter font (`font-typewriter`, `Typewriter` reveal): new answers type out character by character (`useTypewriter`), a click, Enter or Space skips to the full text, stored answers appear instantly.

---

## Phase 5: Case Resolution & Admin (P5)

### P5.1 Accusation Backend (Go)
- [x] Create the final accusation submission API (`POST /api/cases/{id}/accuse` `{suspect_id}`; 403 unless the player holds the lock; answers `correct`, `trophies`, `trophies_change`, `culprit_*`, `resolution`).
- [x] Implement logic to evaluate the selected suspect against the true culprit in PostgreSQL.
- [x] Update the user's Trophies in PostgreSQL (`reward_trophies` on a win, `penalty_trophies` on a loss, never below zero; both are set per case by the admin, along with the `resolution` text). A win pays trophies only: no bonus tokens (the `reward_tokens` column was removed in migration 00013).
- [x] Update the user's score in the Redis Leaderboard (ZINCRBY on the `leaderboard` set by the exact change; the dashboard leaderboard reads that set and rebuilds it from Postgres every 5 minutes, or when it is empty or Redis was down).
- [x] Unlock the user's "Active Case Lock".

### P5.2 Accusation Frontend (React)
- [x] Build the final Accusation UI displaying all suspect avatars for selection.
- [x] Build the "Submit Deduction" confirmation flow.
- [x] Build the Case Outcome reveal screen (Win/Loss animations, updated trophies/tokens display).
- [x] Stamp the outcome onto the case file with the existing `Stamp` component ("SOLVED" / "GUILTY" / "WRONG").

### P5.3 Admin Panel (Go & React)
- [x] **Auth:** `users.role` (`user` / `admin`, migration 00019, replaces `is_admin`) is in the JWT claims, the login response and the profile. `admin.Handler.AdminOnly` guards every `/api/admin/*` route: the token's role first, then `users.role` in the database, so a demotion applies at once. Admins log in to `/admin/dashboard`.
- [x] **Backend:** Secured global config API `GET/PUT /api/admin/config` (daily tokens 1-10, reset time HH:MM, IANA time zone) stored in `app_settings` (`internal/settings`, cached 30 s per replica). The reset job re-reads it every minute; signup and the guard's daily call cap follow the allowance.
- [x] **Backend:** Case CRUD `GET/POST /api/admin/cases`, `GET/PATCH/DELETE /api/admin/cases/{id}` (title, synopsis, difficulty, reward/penalty trophies, main plot, resolution, up to 3 hints, model, thumbnail, published). New cases are drafts; publishing needs 2+ suspects and one culprit; played cases cannot be deleted (unpublish instead).
- [x] **Backend:** Suspect CRUD `GET/POST /api/admin/cases/{id}/suspects`, `PATCH/DELETE .../{suspect_id}` with the LLM persona fields (persona, public alibi, hidden truth, confession triggers, confession keywords) that `llm.BuildSystemPrompt` assembles into the prompt; one culprit per case (the flag moves), secrets need keywords. Scene CRUD `POST /api/admin/cases/{id}/scenes`, `PATCH/DELETE .../{scene_id}`.
- [x] **Backend:** Admin image upload `POST /api/admin/upload` (thumbnail, scene, suspect) through `storage.Provider`: re-encoded to WebP, scaled down, unique names cached for a year; replaced or deleted admin images are removed from the bucket.
- [x] **Frontend:** Admin panel under `/admin` (`AdminLayout` lets only admins in): case list with status, readiness and delete; global settings form; long-scroll case builder (basic info, thumbnail, main case file, resolution, 3 progressive hints, dynamic suspect cards with avatar, testimony, culprit switch and AI persona, scene images, publish) saved in one go by `useSaveCase` with "Save as draft" / "Save & publish" ("Move to draft" / "Save" for a live case). The AI model is a searchable dropdown fed by `GET /api/admin/models` (only text models within the price cap), so ids cannot be mistyped.
- [x] **Admin:** Give tokens to one player from the settings page: `GET /api/admin/users?q=` finds players by username or email, `POST /api/admin/users/{id}/tokens` adds 1-10 (capped at the 10 a player can hold; the daily reset still applies).

---

## Game UI Polish (cross-phase)

- [x] Magnifying-glass cursor, fingerprint mark on button hover, gold `frame` on key panels, film grain and vignette.
- [x] Case-file props in `components/game/`: `Tape`, `Stamp`, `Redacted`, `Typewriter`, `RedString`; applied to the dashboard story cards (redacted + CLASSIFIED stamp on locked cases).
- [x] Sound effects (button click, paper rustle on opening a case, typewriter keys, stamp) with a mute toggle. Synthesised with Web Audio, no files. See `docs/ui_todo.md`.
- [x] Background music: static files in the bucket at `music/<file>.ogg` (`main.ogg` plus 4 case tracks). `MusicPlayer` (in `Layout`, so every page) loops the active case's track (`stories.music_track`, picked by the admin in the case builder; sent as `active_music_track` in `GET /api/dashboard/stats`) while the player is in a case and `main.ogg` everywhere else, crossfading on change and starting on the first click if the browser blocks autoplay. Navbar music button opens a panel with an on/off switch and a volume slider (remembered per browser).
- [ ] Rank titles (Rookie, Detective, Inspector, Chief) shown on the leaderboard and avatar.
- [ ] Loading screen ("Searching the archives...") and page transitions.
