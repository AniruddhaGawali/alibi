---
name: alibi-api
description: How to integrate backend APIs in the Alibi frontend. Use this skill whenever the frontend needs to fetch, send, or mutate data from the Go backend, such as dashboard, stories, token balance, leaderboard, chat, auth, accusation, or admin endpoints, or whenever you would otherwise write fetch/axios in a component. Endpoints are declared once with createApi in frontend/src/api/, which generates the React Query hooks that components call directly.
---

# Integrating APIs in Alibi

All network access lives in `frontend/src/api/`. Components, pages, hooks and stores never call `fetch` or build URLs. This keeps endpoints in one place, makes auth and error handling uniform, and lets React Query cache everything consistently.

## What lives in `src/api/`

| File | Role |
|---|---|
| `apiClient.ts` | `fetch` wrapper used by `createApi`. Reads the JWT from the cookie (`getJwt()` in `src/utils/cookieHelpers.ts`) and sends `Authorization: Bearer`. Unwraps the backend envelope `{status, data}`, and throws `ApiError` (`status`, `message`, `details`) for non-2xx responses or `status: "error"`. `isRetryableError` treats 5xx, 429 and network failures as retryable, other 4xx as not. |
| `queryClient.ts` | The single `QueryClient`, provided in `main.tsx`. Defaults: `staleTime` 30s, `gcTime` 5min, up to 2 retries only for retryable errors, no refetch on window focus, mutations never retry. |
| `createApi.ts` | The `createApi` factory, plus `invalidateTags`. Don't edit it for a single endpoint. |
| `<domain>.ts` | One file per domain, each calling `createApi` (e.g. `dashboard.ts` exports `useGetDashboardQuery`). |

The backend side of the contract is the Go `SuccessResponse` / `ErrorResponse` envelope (see `alibi-backend-api`). Error text shown to users comes from the envelope's `message`, so `error.message` is safe to put in a toast.

## Workflow

1. **Check `src/api/` first.** If an endpoint for it already exists, use its hook. Don't create a duplicate.
2. **If not, declare it with `createApi`** in a domain file in `src/api/` (`auth.ts`, `dashboard.ts`, `stories.ts`, `chat.ts`).
3. **In the component, call the generated hook.** Never import `useQuery`, `useMutation` or `useQueryClient` in components or pages; `createApi` wraps React Query so there is no per-page setup.

## `createApi` (src/api/createApi.ts)

Works like Redux Toolkit's `createApi`. Each endpoint becomes a hook named `use<Name>Query` or `use<Name>Mutation`. It builds on `apiClient`, which adds the JWT, unwraps the `{status, data}` envelope, and throws `ApiError`.

```ts
// src/api/stories.ts
import { createApi } from './createApi'

export interface Story { id: string; title: string; difficulty: 'easy' | 'medium' | 'hard' }

export const storiesApi = createApi({
  name: 'stories',
  endpoints: (build) => ({
    getStories: build.query<Story[]>({
      query: () => '/api/stories',
      providesTags: ['Stories'],
    }),
    getStory: build.query<Story, string>({
      query: (id) => `/api/stories/${id}`,
      providesTags: (_result, id) => [{ type: 'Stories', id }],
      staleTime: 60,
    }),
    startStory: build.mutation<{ caseId: string }, string>({
      query: (id) => ({ url: `/api/stories/${id}/start`, method: 'POST' }),
      invalidatesTags: ['Stories', 'Dashboard'],
    }),
  }),
})

export const { useGetStoriesQuery, useGetStoryQuery, useStartStoryMutation } = storiesApi
```

- `build.query<Result, Arg = void>` and `build.mutation<Result, Arg = void>`. `query` returns a path string, or `{ url, method, body }` (pass `body` as an object; it is JSON-stringified).
- **Caching is tag-based.** A query `providesTags` (a list, or a function of `(result, arg)`); a mutation `invalidatesTags`. When a mutation succeeds, every cached query whose tags match refetches on its own. Use `{ type, id }` for per-item tags; a type-only tag matches any id. Tags are global across all APIs, so a mutation in `chat.ts` can refresh `'Dashboard'` data from `dashboard.ts`.
- **Per-endpoint cache control, in seconds:** `staleTime` (how long data is fresh) and `keepUnusedDataFor` (how long unused data is kept). The defaults (30s / 5min) live in `src/api/queryClient.ts`, which also keeps mutations from auto-retrying so token-spending actions are never repeated.
- Export types for responses next to the endpoints, and export the destructured hooks from the file. Paths start with `/api/`, matching the Go routes. The base URL comes from `VITE_API_URL`.
- Outside React (for example on logout) use `queryClient.clear()` or `invalidateTags(queryClient, [...])` from `src/api/`.

## Using the hooks in a component

```tsx
const { data, isPending, error } = useGetStoriesQuery()          // void arg: no argument
const story = useGetStoryQuery(id, { enabled: !!id })             // second arg = optional React Query options
const start = useStartStoryMutation({ onError: (e) => toast.error(e.message) })
start.mutate(id)                                                  // or await start.mutateAsync(id)
```

- Queries return the React Query result (`data`, `isPending`, `error`, `refetch`...). Mutations return the mutation result (`mutate`, `mutateAsync`, `isPending`...).
- Always handle loading (shadcn `Skeleton`) and error states in the UI.
- You don't invalidate by hand: declare `invalidatesTags` on the endpoint instead.
- Mirror server values into Zustand only when components outside the query need them; otherwise read straight from the hook.
- Forms submit through a mutation hook; see the `alibi-forms` skill.

## Auth and cache notes

- **Expired access tokens renew themselves.** `apiClient` answers a 401 (on any non-auth path) by calling `/api/auth/refresh` once with the refresh cookie, saving the new token pair and repeating the request. Concurrent 401s share one refresh (refresh tokens work once). Only a rejected refresh token ends the session: `setSessionExpiredHandler` (registered in `App.tsx`) clears the session and goes to `/login`. Network or server errors during refresh never log the user out. Route guards and the session store use `hasSession()` (access or refresh cookie), not the access cookie alone.

- The JWT is read in exactly one place, `apiClient.ts`, from `src/utils/cookieHelpers.ts`. Don't read or attach tokens anywhere else. A planned move to an HttpOnly cookie set by the backend would then only change `apiClient.ts` (send `credentials: 'include'`), not any endpoint or component.
- Invalidation scans the cached queries for matching tags, which is cheap at this size. Keep tags coarse and few (`'Dashboard'`, `'Stories'`, `{type, id}` for items) rather than one tag per field, and don't invalidate on every keystroke or poll.
- Query failures arrive as `ApiError` with the backend's `message`; a 503 from `/health`-style degraded states and 429 (e.g. out of tokens) are meaningful, so show `error.message` rather than a generic failure.

## When the backend endpoint doesn't exist yet

Still declare the endpoint with `createApi` against the agreed path from `docs/development_phases_and_todo_list.md`, and say in your reply that the endpoint is pending. Don't mock data inside components.

`apiClient` expects a JSON body on every response, so an endpoint returning `204 No Content` needs `apiClient` extended first.

Finish with `cd frontend && npm run build && npm run lint`.
