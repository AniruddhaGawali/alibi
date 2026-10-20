import { createApi } from './createApi'

export type Difficulty = 'easy' | 'medium' | 'hard'

/** Global game settings (GET/PUT /api/admin/config). */
export interface AdminConfig {
  daily_tokens: number
  /** HH:MM, 24-hour clock. */
  token_reset_time: string
  /** IANA zone the reset time is read in. */
  token_reset_timezone: string
  updated_at?: string
}

export interface AdminCaseSummary {
  id: string
  title: string
  synopsis: string
  thumbnail_url: string
  difficulty_level: Difficulty
  reward_trophies: number
  penalty_trophies: number
  is_published: boolean
  llm_model: string
  music_track: string
  created_at: string
  suspect_count: number
  culprit_count: number
  /** Settled accusations. A played case cannot be deleted. */
  plays: number
}

export interface AdminCaseFields {
  title: string
  synopsis: string
  main_plot: string
  thumbnail_url: string
  difficulty_level: Difficulty
  reward_trophies: number
  penalty_trophies: number
  resolution: string
  hints: string[]
  is_published: boolean
  llm_model: string
  /** A case track key (utils/music CASE_TRACKS); '' plays the main theme. */
  music_track: string
}

export interface AdminCase extends AdminCaseFields {
  id: string
  created_at: string
}

/** Every suspect field, secrets included. Only admins ever see these. */
export interface AdminSuspectFields {
  name: string
  role: string
  avatar_url: string
  initial_testimony: string
  is_culprit: boolean
  display_order: number
  relationship_to_victim: string
  persona: string
  public_alibi: string
  hidden_truth: string
  confession_triggers: string
  /** Groups of word starts joined by "+"; the secrets enter the prompt only once a question matches one. */
  confession_keywords: string[]
}

export interface AdminSuspect extends AdminSuspectFields {
  id: string
  case_id: string
}

export interface AdminSceneFields {
  scene_name: string
  image_url: string
  display_order: number
}

export interface AdminScene extends AdminSceneFields {
  id: string
  case_id: string
}

export interface AdminCaseDetail extends AdminCase {
  suspects: AdminSuspect[]
  scenes: AdminScene[]
}

/** An OpenRouter model a case may use (text output only, within the price cap). */
export interface AdminModel {
  id: string
  name: string
  description: string
  context_length: number
  /** USD per million tokens. */
  prompt_price: number
  completion_price: number
  free: boolean
}

export interface AdminModelList {
  models: AdminModel[]
  /** The model new cases get. */
  default: string
}

/** A player as the admin panel lists them. */
export interface AdminPlayer {
  id: string
  username: string
  email: string
  role: 'user' | 'admin'
  token_balance: number
  trophies: number
  created_at: string
}

export interface TokenGrant {
  user_id: string
  token_balance: number
  /** What was actually added: a player never holds more than 10. */
  granted: number
  capped: boolean
}

/** The most question tokens a player can hold (user.MaxTokenBalance on the server). */
export const MAX_TOKEN_BALANCE = 10

export type ImageKind = 'thumbnail' | 'scene' | 'suspect'

export interface UploadedImage {
  url: string
  path: string
}

type CaseRef = { caseId: string }

export const adminApi = createApi({
  name: 'admin',
  endpoints: (build) => ({
    getAdminConfig: build.query<AdminConfig>({
      query: () => '/api/admin/config',
      providesTags: ['AdminConfig'],
    }),
    updateAdminConfig: build.mutation<AdminConfig, Omit<AdminConfig, 'updated_at'>>({
      query: (body) => ({ url: '/api/admin/config', method: 'PUT', body }),
      invalidatesTags: ['AdminConfig'],
    }),
    getAdminModels: build.query<AdminModelList>({
      query: () => '/api/admin/models',
      providesTags: ['AdminModels'],
      staleTime: 600, // the server caches the OpenRouter list for 10 minutes too
    }),
    searchAdminUsers: build.query<AdminPlayer[], string>({
      query: (q) => `/api/admin/users?q=${encodeURIComponent(q)}`,
      providesTags: ['AdminUsers'],
      staleTime: 0,
    }),
    grantAdminTokens: build.mutation<TokenGrant, { id: string; amount: number }>({
      query: ({ id, amount }) => ({ url: `/api/admin/users/${id}/tokens`, method: 'POST', body: { amount } }),
      invalidatesTags: ['AdminUsers'],
    }),
    resetAdminUserLimits: build.mutation<{ user_id: string }, string>({
      query: (id) => ({ url: `/api/admin/users/${id}/reset-limits`, method: 'POST' }),
    }),
    getAdminCases: build.query<AdminCaseSummary[]>({
      query: () => '/api/admin/cases',
      providesTags: ['AdminCases'],
    }),
    getAdminCase: build.query<AdminCaseDetail, string>({
      query: (id) => `/api/admin/cases/${id}`,
      providesTags: (_r, id) => [{ type: 'AdminCases', id }],
      staleTime: 0,
    }),
    // The case builder saves a case in several requests (case, suspects, scenes) and
    // invalidates 'AdminCases' and 'Stories' itself once all of them succeeded
    // (useSaveCase), so these write endpoints declare no tags: a refetch half-way
    // would reload the builder over the edits still being saved.
    createAdminCase: build.mutation<AdminCase, Omit<AdminCaseFields, 'is_published'>>({
      query: (body) => ({ url: '/api/admin/cases', method: 'POST', body }),
    }),
    updateAdminCase: build.mutation<AdminCase, { id: string } & Partial<AdminCaseFields>>({
      query: ({ id, ...body }) => ({ url: `/api/admin/cases/${id}`, method: 'PATCH', body }),
    }),
    deleteAdminCase: build.mutation<{ id: string }, string>({
      query: (id) => ({ url: `/api/admin/cases/${id}`, method: 'DELETE' }),
      invalidatesTags: ['AdminCases', 'Stories'],
    }),
    createAdminSuspect: build.mutation<AdminSuspect, CaseRef & AdminSuspectFields>({
      query: ({ caseId, ...body }) => ({ url: `/api/admin/cases/${caseId}/suspects`, method: 'POST', body }),
    }),
    updateAdminSuspect: build.mutation<AdminSuspect, CaseRef & { id: string } & Partial<AdminSuspectFields>>({
      query: ({ caseId, id, ...body }) => ({ url: `/api/admin/cases/${caseId}/suspects/${id}`, method: 'PATCH', body }),
    }),
    deleteAdminSuspect: build.mutation<{ id: string }, CaseRef & { id: string }>({
      query: ({ caseId, id }) => ({ url: `/api/admin/cases/${caseId}/suspects/${id}`, method: 'DELETE' }),
    }),
    createAdminScene: build.mutation<AdminScene, CaseRef & AdminSceneFields>({
      query: ({ caseId, ...body }) => ({ url: `/api/admin/cases/${caseId}/scenes`, method: 'POST', body }),
    }),
    updateAdminScene: build.mutation<AdminScene, CaseRef & { id: string } & Partial<AdminSceneFields>>({
      query: ({ caseId, id, ...body }) => ({ url: `/api/admin/cases/${caseId}/scenes/${id}`, method: 'PATCH', body }),
    }),
    deleteAdminScene: build.mutation<{ id: string }, CaseRef & { id: string }>({
      query: ({ caseId, id }) => ({ url: `/api/admin/cases/${caseId}/scenes/${id}`, method: 'DELETE' }),
    }),
    /** Re-encoded to WebP on the server; save the returned url with the case, suspect or scene. */
    uploadAdminImage: build.mutation<UploadedImage, { kind: ImageKind; caseId: string; file: File }>({
      query: ({ kind, caseId, file }) => {
        const body = new FormData()
        body.append('kind', kind)
        body.append('case_id', caseId)
        body.append('file', file)
        return { url: '/api/admin/upload', method: 'POST', body }
      },
    }),
  }),
})

export const {
  useGetAdminConfigQuery,
  useUpdateAdminConfigMutation,
  useGetAdminModelsQuery,
  useSearchAdminUsersQuery,
  useGrantAdminTokensMutation,
  useResetAdminUserLimitsMutation,
  useGetAdminCasesQuery,
  useGetAdminCaseQuery,
  useCreateAdminCaseMutation,
  useUpdateAdminCaseMutation,
  useDeleteAdminCaseMutation,
  useCreateAdminSuspectMutation,
  useUpdateAdminSuspectMutation,
  useDeleteAdminSuspectMutation,
  useCreateAdminSceneMutation,
  useUpdateAdminSceneMutation,
  useDeleteAdminSceneMutation,
  useUploadAdminImageMutation,
} = adminApi
