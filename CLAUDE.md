# Alibi

AI detective mystery game. Go backend in `backend/`, Vite + React 19 + TypeScript frontend in `frontend/`, infrastructure in `docker-compose.yml`. Phases and the checklist are in `docs/development_phases_and_todo_list.md`; tick items off there when they are done.

## Skills

Project skills are indexed in `.claude/skills/INDEX.md` (path, purpose and trigger for each). Check it before starting work and load every skill that applies:

- `alibi-ui` — frontend UI: shadcn components, golden-sunset theme (`.claude/skills/alibi-ui/SKILL.md`)
- `alibi-forms` — forms with `components/form` and zod (`.claude/skills/alibi-forms/SKILL.md`)
- `alibi-api` — frontend API calls via `createApi` hooks (`.claude/skills/alibi-api/SKILL.md`)
- `alibi-backend-api` — Go backend: input validation, JSON responses, logging, errors/startup, queues, health, tests, and the goose + sqlc database workflow (`.claude/skills/alibi-backend-api/SKILL.md`)

When you add or change a skill, update `INDEX.md` and this list.

## Checks before finishing

- Backend: `cd backend && go build ./... && go vet ./... && go test ./...`; if you touched `backend/sql/`, also `make sqlc` and `make sqlc-diff` (needs the `sqlc` and `goose` CLIs, see `backend/sql/README.md`)
- Frontend: `cd frontend && npm run build && npm run lint` (the stock shadcn "only-export-components" warnings in `components/ui/` are expected)
