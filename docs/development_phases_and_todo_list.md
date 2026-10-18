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
- [x] Image storage behind `storage.Provider` (`internal/storage`), Supabase Storage for now (swappable for S3/GCS), WebP only; `make assets-upload` for manual case/avatar art until the admin panel exists.
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
- [x] Create API endpoint to fetch the top players from the Redis Leaderboard (ZSET).
- [x] Implement "Active Case Lock" logic to verify if a user is currently locked into an unfinished case.

### P2.2 Dashboard Frontend (React)
- [x] Build Crime Story Cards displaying synopsis, title, difficulty, and rewards.
- [x] Build the live Token Balance and Trophies display component.
- [x] Build the Global Leaderboard component.

### P2.3 Case Selection Integration
- [x] Connect the dashboard UI to fetch live data from the backend using React Query.
- [x] Implement frontend routing restrictions based on the "Active Case Lock" (preventing users from clicking new stories if one is active). Cards use `GET /api/dashboard/active-case`: the active one shows In progress, the rest Locked.

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
- [x] Build the global chat history section on the main story dashboard. (`ChatHistorySection`; reads `GET /api/cases/{id}/chat-history`, which does not exist yet (P4.3), so the UI shows the empty state until it does. Expected item shape: `ChatMessage` in `src/api/chatHistory.ts`.)
- [x] Implement filtering logic so clicking a specific character shows only their chat previews. (Filter pills, and clicking a suspect card filters and scrolls to the logs; `selectedSuspectFilter` in `useGameStore`.)
- [x] Detective's notes live inside the interrogation log (`NoteComposer` + `NoteCard`): an "Add a note" box above the logs, and notes appear in the same scrolling stream as the logs, newest first.
- [x] Case notes are saved to the player's account: `case_notes` table (migration 00010) and `GET/POST /api/cases/{id}/notes`, `DELETE /api/cases/{id}/notes/{note_id}` (own notes only, max 1000 chars, max 200 per case), used through `createApi` hooks.
- [x] A note can be about one suspect (`suspect_id`) or the case in general: with a suspect filter selected the note is about that suspect, under All Logs a dropdown picks who. Notes show under the matching filter.
- [x] Tie a note to a specific interrogation log entry: "Add note" on each log card saves a note with `chat_message_id` (`case_notes.chat_message_id`, migration 00012). The API checks the entry is the caller's own, from this case, and takes the note's suspect from it; the note card quotes the question it is about.

---

## Phase 4: The Interrogation (Core Gameplay Loop) (P4)

### P4.1 LLM Backend Integration (Go)
- [x] Set up the LLM Microservice to connect with the Ollama/OpenRouter APIs. OpenRouter answers interrogations (`internal/llm`, `internal/interrogation`, `POST /api/cases/{id}/suspects/{suspect_id}/interrogate`); Ollama stays a local-only client.
- [x] Per-case model: `stories.llm_model` (default `google/gemma-4-31b-it`), set by admins through `POST /api/admin/cases` and `PATCH /api/admin/cases/{id}` (needs `users.is_admin`). Model ids are format-checked (no `:online`, no `openrouter/*` routers) and must be text-output-only on OpenRouter.
- [x] Text-to-text only: no tools or tool choice, web search and file parser plugins switched off, output modality `text`; tool calls, images or audio in a reply are discarded and the token is refunded.
- [x] Implement the input moderation layer to detect and block off-topic queries (e.g., code, math), plus obvious prompt-injection attempts (`llm.Moderate`; blocked questions get a fixed in-character reply and cost no token).
- [x] Build the dynamic System Prompt swapping logic based on the requested character ID (Dual-State prompting for culprits). Three sections: persona with public alibi / hidden truth / confession triggers, cross-character awareness (other suspects' public facts only, never their secrets), strict rules (length, stay in character, injection defence).
- [ ] Optimize the request chain to ensure AI responses return within the 2-3 second NFR limit. Done so far: no SDK retries, 220-token reply cap, `LLM_TIMEOUT_SECONDS` (default 20) with refund. Still to do: measure real latency with an `OPENROUTER_API_KEY` and pick a faster default model if Gemma misses the target.

### P4.2 Token Economy Backend (Go)
- [x] Implement the 1-token deduction executed *before* calling the LLM (after moderation): `wallet.SpendQuestion` runs an atomic Redis DECR on `user:coins:<id>` that is undone straight away if it goes below zero (403 "Out of tokens"), then mirrors the spend to Postgres; on any disagreement the key is dropped and reloaded from Postgres.
- [x] Implement the token refund logic if the LLM times out or fails (`wallet.RefundQuestion`: Postgres, then Redis INCR; runs even if the client disconnects). Moderation runs before the spend, so a blocked query never costs a token.
- [x] Create the daily cron job to reset user token balances to 10 (without rolling over unused tokens): `internal/jobs` with robfig/cron inside the API process, `TOKEN_RESET_CRON` / `TOKEN_RESET_TIMEZONE` / `DAILY_TOKENS`, a Redis lock per calendar day so the reset runs exactly once a day across all replicas, for all users in one update (retried 3 times if Postgres fails, lock released on failure), Postgres set to exactly 10 (whatever the balance was, low, empty or higher) then the Redis balance keys cleared. The reset time comes from env for now; making it editable from the admin panel is P5.3.

### P4.3 Chat Storage Backend (Go)
- [x] Configure the LLM service to asynchronously push the generated Q&A payload to the Message Queue (`queue:chat:events`, `chat.Event`: user_id, case_id, suspect_id, user_query, ai_response, timestamp, plus event_id, play_id, suspect_name, model).
- [x] Build the background worker (Chat Storage Microservice) to consume queue events and insert documents into MongoDB (`chat.Worker`, goroutines in the API process, `CHAT_WORKER_CONCURRENCY`; collection `chat_messages`, `_id` = event id so redeliveries are no-ops; retries and `queue:chat:dlq` from the broker).
- [x] Create API endpoint to fetch historical character chats from MongoDB: `GET /api/chat/{case_id}/{suspect_id}`, the caller's own exchanges oldest first (latest 200). The last 6 exchanges are also fed back to the model so suspects remember the conversation.

### P4.4 Chat Frontend (React)
- [x] Build the real-time chat interface (message bubbles, input field, loading states): `components/chat/` (`ChatWindow`, `MessageBubble`, `ThinkingBubble`, `ChatInput`, `TokenCounter`). Detective bubbles are modern and right-aligned, suspects are aged paper in typewriter type on the left, errors are centred notices; "Suspect is thinking…" shows the moment a question is sent; the window follows new messages (`useStickToBottom`) and earlier conversation loads from `GET /api/chat/{case_id}/{suspect_id}`.
- [x] Connect the chat UI to the LLM backend via standard HTTP: `useInterrogateMutation` (`api/chat.ts`) posts to `POST /api/cases/{id}/suspects/{suspect_id}/interrogate` (the built route, not `/api/chat`). A moderation block (422) shows the suspect's in-character brush-off; timeouts and failures show a notice; answers are added to the cached history so the chat and the case log update at once.
- [x] Integrate the visual token counter to update instantly when a message is sent: `useGameStore.tokenBalance` is the one balance (kept in step with the server by `useSyncTokenBalance` in the layout), drops by 1 on send, is refunded on an error and set to the server's figure on success. The navbar menu, the stats panel and the chat all read it.
- [x] Add state logic to disable and visually block the chat input when the token balance hits zero (disabled, "Out of tokens. Make your final deduction.", red border, greyed field, not-allowed cursor).
- [x] Show testimonies and chat logs in the typewriter font (`font-typewriter`, `Typewriter` reveal): new answers type out character by character (`useTypewriter`), a click, Enter or Space skips to the full text, stored answers appear instantly.

---

## Phase 5: Case Resolution & Admin (P5)

### P5.1 Accusation Backend (Go)
- [ ] Create the final accusation submission API.
- [ ] Implement logic to evaluate the selected suspect against the true culprit in PostgreSQL.
- [ ] Update the user's Trophies in PostgreSQL. A win pays trophies only: no bonus tokens (the `reward_tokens` column was removed in migration 00013).
- [ ] Update the user's score in the Redis Leaderboard (ZINCRBY).
- [ ] Unlock the user's "Active Case Lock".

### P5.2 Accusation Frontend (React)
- [ ] Build the final Accusation UI displaying all suspect avatars for selection.
- [ ] Build the "Submit Deduction" confirmation flow.
- [ ] Build the Case Outcome reveal screen (Win/Loss animations, updated trophies/tokens display).
- [ ] Stamp the outcome onto the case file with the existing `Stamp` component ("SOLVED" / "GUILTY" / "WRONG").

### P5.3 Admin Panel (Go & React)
- [ ] **Backend:** Create secured Admin APIs to update global configurations (daily token limits, specific refresh times).
- [ ] **Backend:** Admin image upload API for case art (thumbnails, scenes, suspects) through `storage.Provider`, replacing `make assets-upload`.
- [ ] **Frontend:** Build a simple Admin UI dashboard to adjust these settings.

---

## Game UI Polish (cross-phase)

- [x] Magnifying-glass cursor, fingerprint mark on button hover, gold `frame` on key panels, film grain and vignette.
- [x] Case-file props in `components/game/`: `Tape`, `Stamp`, `Redacted`, `Typewriter`, `RedString`; applied to the dashboard story cards (redacted + CLASSIFIED stamp on locked cases).
- [x] Sound effects (button click, paper rustle on opening a case, typewriter keys, stamp) with a mute toggle. Synthesised with Web Audio, no files. See `docs/ui_todo.md`.
- [ ] Rank titles (Rookie, Detective, Inspector, Chief) shown on the leaderboard and avatar.
- [ ] Loading screen ("Searching the archives...") and page transitions.
