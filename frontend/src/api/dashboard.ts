import { createApi } from './createApi'

export type Difficulty = 'easy' | 'medium' | 'hard'

export interface Story {
  id: string
  title: string
  synopsis: string
  thumbnail_url: string
  difficulty_level: Difficulty
  reward_trophies: number
}

export interface DashboardStats {
  trophies: number
  tokens: number
  cases_solved: number
  /** Title of the case in progress, or null when the user is free to pick one. */
  active_case_title: string | null
  /** Id of the story in progress (match cards on this, titles are not unique), null when free. */
  active_story_id: string | null
}

export interface LeaderboardEntry {
  rank: number
  username: string
  trophies: number
  /** The signed-in player's own row. */
  is_me: boolean
}

/** The live panels refetch every 5 minutes (React Query pauses this while the tab is hidden). */
export const STATS_POLL_MS = 5 * 60 * 1000
export const LEADERBOARD_POLL_MS = 5 * 60 * 1000

export const dashboardApi = createApi({
  name: 'dashboard',
  endpoints: (build) => ({
    getStories: build.query<Story[]>({
      query: () => '/api/dashboard/stories',
      providesTags: ['Stories'],
      staleTime: 60,
    }),
    getStats: build.query<DashboardStats>({
      query: () => '/api/dashboard/stats',
      providesTags: ['Stats'],
      staleTime: 0,
    }),
    getLeaderboard: build.query<LeaderboardEntry[]>({
      query: () => '/api/dashboard/leaderboard',
      providesTags: ['Leaderboard'],
      staleTime: 0,
    }),
  }),
})

export const {
  useGetStoriesQuery,
  useGetStatsQuery,
  useGetLeaderboardQuery,
} = dashboardApi
