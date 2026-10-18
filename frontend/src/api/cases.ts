import { createApi } from './createApi'
import type { Difficulty } from './dashboard'

export interface Scene {
  id: string
  scene_name: string
  image_url: string
  display_order: number
}

/** The case dossier: GET /api/cases/{id}/briefing. Scenes arrive ordered. */
export interface Briefing {
  id: string
  title: string
  synopsis: string
  main_plot: string
  difficulty_level: Difficulty
  reward_trophies: number
  reward_tokens: number
  scenes: Scene[]
}

/** GET /api/cases/{id}/suspects. The server never sends who the culprit is. */
export interface Suspect {
  id: string
  name: string
  role: string
  avatar_url: string
  initial_testimony: string
}

/** POST /api/cases/{id}/start: `case_id` is this play-through, `story_id` the case. */
export interface CaseLock {
  case_id: string
  story_id: string
}

export const casesApi = createApi({
  name: 'cases',
  endpoints: (build) => ({
    getBriefing: build.query<Briefing, string>({
      query: (id) => `/api/cases/${id}/briefing`,
      providesTags: (_result, id) => [{ type: 'Cases', id }],
    }),
    getSuspects: build.query<Suspect[], string>({
      query: (id) => `/api/cases/${id}/suspects`,
      providesTags: (_result, id) => [{ type: 'Cases', id }],
    }),
    startCase: build.mutation<CaseLock, string>({
      query: (id) => ({ url: `/api/cases/${id}/start`, method: 'POST' }),
      // the dashboard's active-case banner and stats depend on the lock
      invalidatesTags: ['Stats', 'Cases'],
    }),
  }),
})

export const { useGetBriefingQuery, useGetSuspectsQuery, useStartCaseMutation } = casesApi
