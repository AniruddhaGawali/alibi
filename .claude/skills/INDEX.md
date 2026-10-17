# Alibi skills index

Project skills live in `.claude/skills/<name>/SKILL.md`. Use this index to find the right one. Several usually apply to one task, for example a login screen needs `alibi-ui`, `alibi-forms` and `alibi-api`, plus `alibi-backend-api` for the endpoint behind it.

| Skill | Path | What it does | Use it when |
|---|---|---|---|
| `alibi-ui` | `.claude/skills/alibi-ui/SKILL.md` | Frontend UI conventions: build from shadcn components (install missing ones, never edit `components/ui/`), the golden-sunset sharp-corner game theme in `src/index.css`, glow and shine rules, thin pages split into feature components, providers in `App.tsx`, lean dependencies, JWT handling kept in one place. | Building or changing any page, component, dialog, button, layout or style in `frontend/`. |
| `alibi-forms` | `.claude/skills/alibi-forms/SKILL.md` | Forms only from `src/components/form/` (`FormInput`, `FormSelect`, ...) with a zod schema and react-hook-form; errors shown by shadcn `FieldError`. | Creating or changing any form or validated user input. |
| `alibi-api` | `.claude/skills/alibi-api/SKILL.md` | Frontend API integration: declare endpoints with `createApi` in `src/api/`, tag-based caching, generated `use...Query` / `use...Mutation` hooks so pages need no React Query setup. | A component needs data from, or must send data to, the Go backend. |
| `alibi-backend-api` | `.claude/skills/alibi-backend-api/SKILL.md` | Go backend: validate all client input (`utils.BindAndValidate`), respond only with `SuccessResponse` / `ErrorResponse`, log every API with the context-aware logger through `middleware.Logging`, return errors instead of panicking, env-tunable pools, `/health` checks, reliable queues (retries, backoff, DLQ), tests, and the mandatory goose migrations + sqlc type-safe queries workflow (`backend/sql/`). | Adding or changing any HTTP handler, route, queue worker, config, startup code or any database query/schema in `backend/`. |

## Adding or changing a skill

1. Create `.claude/skills/<name>/SKILL.md` with `name` and `description` frontmatter. The description decides when it loads, so say what it does and the situations that should trigger it.
2. Add a row to the table above (name, path, what it does, when to use it).
3. If the skill documents code, update it in the same change as that code so it doesn't drift.
