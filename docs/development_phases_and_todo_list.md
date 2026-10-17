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
- [ ] Implement Supabase Auth APIs for Email/Password (with OTP) and Google Login.
- [ ] Create the user profile record in PostgreSQL upon successful signup.
- [ ] Implement JWT generation, parsing, and validation middleware.
- [ ] Initialize the Redis JWT Blacklist logic for secure logouts.

### P1.2 Auth Frontend (React)
- [ ] Build the Signup form (Email/Username/Password) and direct Google Login button.
- [ ] Build the OTP verification screen.
- [ ] Build the Login screen for returning users.
- [ ] Build the Avatar Selection UI for the onboarding flow.

### P1.3 Auth Integration
- [ ] Connect frontend auth forms to the Go backend.
- [ ] Handle routing post-login (redirect to Dashboard).
- [ ] Implement logic to automatically allocate the initial 10 question tokens to the user's wallet upon completing onboarding.
- [ ] Connect the frontend logout button to the backend Redis Blacklist endpoint.

---

## Phase 2: The Home Page & Case Selection (P2)

### P2.1 Dashboard Backend (Go)
- [ ] Create API endpoint to fetch available crime stories from PostgreSQL.
- [ ] Create API endpoint to fetch the user's current token balance and total trophies.
- [ ] Create API endpoint to fetch the top players from the Redis Leaderboard (ZSET).
- [ ] Implement "Active Case Lock" logic to verify if a user is currently locked into an unfinished case.

### P2.2 Dashboard Frontend (React)
- [ ] Build Crime Story Cards displaying synopsis, title, difficulty, and rewards.
- [ ] Build the live Token Balance and Trophies display component.
- [ ] Build the Global Leaderboard component.

### P2.3 Case Selection Integration
- [ ] Connect the dashboard UI to fetch live data from the backend using React Query.
- [ ] Implement frontend routing restrictions based on the "Active Case Lock" (preventing users from clicking new stories if one is active).

---

## Phase 3: The Story and Briefing Page (P3)

### P3.1 Briefing Backend (Go)
- [ ] Create API endpoint to fetch the specific active case's main plot and scene images.
- [ ] Create API endpoint to fetch all suspect avatars and their initial testimonies for the active case.

### P3.2 Briefing Frontend (React)
- [ ] Build the Story Dashboard UI to render the main plot and visuals.
- [ ] Build interactive Suspect Avatar components that display initial testimonies in a tooltip on hover.
- [ ] Build the detailed Testimony Modal/View that opens on click, containing the "Chat" button.

### P3.3 Chat History UI (React)
- [ ] Build the global chat history section on the main story dashboard.
- [ ] Implement filtering logic so clicking a specific character shows only their chat previews.

---

## Phase 4: The Interrogation (Core Gameplay Loop) (P4)

### P4.1 LLM Backend Integration (Go)
- [ ] Set up the LLM Microservice to connect with the Ollama/OpenRouter APIs.
- [ ] Implement the input moderation layer to detect and block off-topic queries (e.g., code, math).
- [ ] Build the dynamic System Prompt swapping logic based on the requested character ID (Dual-State prompting for culprits).
- [ ] Optimize the request chain to ensure AI responses return within the 2-3 second NFR limit.

### P4.2 Token Economy Backend (Go)
- [ ] Implement the 1-token deduction logic in Redis (DECR) executed *before* calling the LLM.
- [ ] Implement the token refund logic (INCR) if the query is blocked by moderation or if the LLM times out.
- [ ] Create the daily cron job (or lazy evaluation script) to reset user token balances to 10 (without rolling over unused tokens).

### P4.3 Chat Storage Backend (Go)
- [ ] Configure the LLM service to asynchronously push the generated Q&A payload to the Message Queue.
- [ ] Build the background worker (Chat Storage Microservice) to consume queue events and insert documents into MongoDB.
- [ ] Create API endpoint to fetch historical character chats from MongoDB.

### P4.4 Chat Frontend (React)
- [ ] Build the real-time chat interface (message bubbles, input field, loading states).
- [ ] Connect the chat UI to the LLM backend via standard HTTP or WebSockets/SSE for fast delivery.
- [ ] Integrate the visual token counter to update instantly when a message is sent (using Zustand/React Query for optimistic updates).
- [ ] Add state logic to disable and visually block the chat input when the token balance hits zero.

---

## Phase 5: Case Resolution & Admin (P5)

### P5.1 Accusation Backend (Go)
- [ ] Create the final accusation submission API.
- [ ] Implement logic to evaluate the selected suspect against the true culprit in PostgreSQL.
- [ ] Update the user's Trophies and bonus tokens in PostgreSQL.
- [ ] Update the user's score in the Redis Leaderboard (ZINCRBY).
- [ ] Unlock the user's "Active Case Lock".

### P5.2 Accusation Frontend (React)
- [ ] Build the final Accusation UI displaying all suspect avatars for selection.
- [ ] Build the "Submit Deduction" confirmation flow.
- [ ] Build the Case Outcome reveal screen (Win/Loss animations, updated trophies/tokens display).

### P5.3 Admin Panel (Go & React)
- [ ] **Backend:** Create secured Admin APIs to update global configurations (daily token limits, specific refresh times).
- [ ] **Frontend:** Build a simple Admin UI dashboard to adjust these settings.