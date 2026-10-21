import { createApi } from './createApi'

/** GET /api/game/rules (public): the token economy the admins set. */
export interface GameRules {
  daily_tokens: number
  /** The most question tokens a player can hold. */
  max_tokens: number
  /** HH:MM, 24-hour clock. */
  token_reset_time: string
  token_reset_timezone: string
}

/** Shown until the rules have loaded (and in the prerendered pages): the defaults. */
export const DEFAULT_RULES: GameRules = { daily_tokens: 10, max_tokens: 10, token_reset_time: '00:00', token_reset_timezone: 'UTC' }

export const gameApi = createApi({
  name: 'game',
  endpoints: (build) => ({
    getGameRules: build.query<GameRules>({
      query: () => '/api/game/rules',
      providesTags: ['GameRules'],
      staleTime: 300,
    }),
  }),
})

export const { useGetGameRulesQuery } = gameApi

/** The current rules, or the defaults while they load or if the request fails. */
export function useGameRules(): GameRules {
  return useGetGameRulesQuery().data ?? DEFAULT_RULES
}
