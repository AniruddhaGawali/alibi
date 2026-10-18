/**
 * createApi — an RTK-Query-style layer on top of React Query.
 *
 * Declare endpoints once in src/api/; components only call the generated
 * hooks (`useGetDashboardQuery()`, `useLoginMutation()`), with no React Query
 * setup of their own. Caching is tag-based: queries `providesTags`, mutations
 * `invalidatesTags`, and every matching cached query refetches automatically.
 */
import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationOptions,
  type UseMutationResult,
  type UseQueryOptions,
  type UseQueryResult,
} from '@tanstack/react-query'
import { apiClient, type ApiError } from './apiClient'

/** A cache tag: a plain type name, or a type plus an id for per-item caching. */
export type Tag = string | { type: string; id?: string | number }

export interface RequestDef {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  /** A plain object is sent as JSON; a FormData is sent as multipart. */
  body?: unknown
}

type QueryTags<R, A> = Tag[] | ((result: R | undefined, arg: A) => Tag[])
type MutationTags<R, A> = Tag[] | ((result: R, arg: A) => Tag[])

export interface QueryDef<R, A> {
  kind: 'query'
  query: (arg: A) => RequestDef | string
  /** Tags this query's data is labelled with. */
  providesTags?: QueryTags<R, A>
  /** Seconds the data counts as fresh (no refetch). Defaults to the global 30s. */
  staleTime?: number
  /** Seconds unused data stays cached. Defaults to the global 5 minutes. */
  keepUnusedDataFor?: number
}

export interface MutationDef<R, A> {
  kind: 'mutation'
  query: (arg: A) => RequestDef
  /** Tags to invalidate (refetch) after the mutation succeeds. */
  invalidatesTags?: MutationTags<R, A>
}

const build = {
  query: <R, A = void>(def: Omit<QueryDef<R, A>, 'kind'>): QueryDef<R, A> => ({ kind: 'query', ...def }),
  mutation: <R, A = void>(def: Omit<MutationDef<R, A>, 'kind'>): MutationDef<R, A> => ({
    kind: 'mutation',
    ...def,
  }),
}
type Builder = typeof build

type QueryOptions<R> = Omit<UseQueryOptions<R, ApiError>, 'queryKey' | 'queryFn'>
type QueryHook<R, A> = (arg: A, options?: QueryOptions<R>) => UseQueryResult<R, ApiError>
type MutationHook<R, A> = (
  options?: Omit<UseMutationOptions<R, ApiError, A>, 'mutationFn'>,
) => UseMutationResult<R, ApiError, A>

type UnionToIntersection<U> = (U extends unknown ? (k: U) => void : never) extends (k: infer I) => void ? I : never

type Hooks<E> = UnionToIntersection<
  {
    [K in keyof E & string]: E[K] extends QueryDef<infer R, infer A>
      ? { [P in `use${Capitalize<K>}Query`]: QueryHook<R, A> }
      : E[K] extends MutationDef<infer R, infer A>
        ? { [P in `use${Capitalize<K>}Mutation`]: MutationHook<R, A> }
        : never
  }[keyof E & string]
>

interface QueryMeta extends Record<string, unknown> {
  providesTags?: QueryTags<unknown, unknown>
  arg?: unknown
}

const toRequest = (r: RequestDef | string): RequestDef => (typeof r === 'string' ? { url: r } : r)

const run = <R>(req: RequestDef | string) => {
  const { url, method = 'GET', body } = toRequest(req)
  const payload = body === undefined ? undefined : body instanceof FormData ? body : JSON.stringify(body)
  return apiClient<R>(url, { method, body: payload })
}

const norm = (t: Tag) => (typeof t === 'string' ? { type: t, id: undefined } : t)

function tagsMatch(provided: Tag[], invalidated: Tag[]): boolean {
  return invalidated.some((inv) => {
    const i = norm(inv)
    return provided.some((p) => {
      const pn = norm(p)
      // a provided tag without an id matches any id of that type
      return pn.type === i.type && (i.id === undefined || pn.id === undefined || pn.id === i.id)
    })
  })
}

/** Refetch every cached query whose provided tags match. Usable outside React too. */
export function invalidateTags(client: QueryClient, tags: Tag[]) {
  if (!tags.length) return Promise.resolve()
  return client.invalidateQueries({
    predicate: (query) => {
      const meta = query.meta as QueryMeta | undefined
      if (!meta?.providesTags) return false
      const provided =
        typeof meta.providesTags === 'function'
          ? meta.providesTags(query.state.data, meta.arg)
          : meta.providesTags
      return tagsMatch(provided, tags)
    },
  })
}

export function createApi<E extends Record<string, QueryDef<any, any> | MutationDef<any, any>>>(config: {
  /** Unique name for this API; namespaces its React Query keys. */
  name: string
  endpoints: (build: Builder) => E
}): Hooks<E> {
  const endpoints = config.endpoints(build)
  const hooks: Record<string, unknown> = {}

  for (const [key, def] of Object.entries(endpoints)) {
    const cap = key.charAt(0).toUpperCase() + key.slice(1)

    if (def.kind === 'query') {
      hooks[`use${cap}Query`] = (arg: unknown, options?: QueryOptions<unknown>) =>
        useQuery({
          queryKey: [config.name, key, arg ?? null],
          queryFn: () => run(def.query(arg)),
          meta: { providesTags: def.providesTags, arg } satisfies QueryMeta,
          // Only set when given: an explicit undefined would replace the QueryClient defaults.
          ...(def.staleTime !== undefined && { staleTime: def.staleTime * 1000 }),
          ...(def.keepUnusedDataFor !== undefined && { gcTime: def.keepUnusedDataFor * 1000 }),
          ...options,
        })
    } else {
      hooks[`use${cap}Mutation`] = (options?: UseMutationOptions<unknown, ApiError, unknown>) => {
        const client = useQueryClient()
        return useMutation({
          ...options,
          mutationFn: (arg: unknown) => run(def.query(arg)),
          onSuccess: async (result, arg, ...rest) => {
            const tags =
              typeof def.invalidatesTags === 'function'
                ? def.invalidatesTags(result, arg)
                : (def.invalidatesTags ?? [])
            await invalidateTags(client, tags)
            return (options?.onSuccess as ((...a: unknown[]) => unknown) | undefined)?.(result, arg, ...rest)
          },
        })
      }
    }
  }

  return hooks as Hooks<E>
}
