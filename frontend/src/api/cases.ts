import { currentUserId } from '@/utils/session'
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
  scenes: Scene[]
  /** How many hints the case has. */
  hints_total: number
  /** The case's detective tips for investigators; empty = the generic ones. */
  chat_tips: string[]
}

/** GET /api/cases/{id}/hints: what this player has revealed (per player, so not part of the briefing). */
export interface HintState {
  hints_total: number
  hints_used: number
  hints: string[]
}

/** POST /api/cases/{id}/hints. `number` is the hint's position and what it costs if the case is solved. */
export interface RevealedHint {
  hint: string
  number: number
  cost: number
  hints_total: number
}

/** GET /api/cases/{id}/suspects. The server never sends who the culprit is. */
export interface Suspect {
  id: string
  name: string
  role: string
  avatar_url: string
  initial_testimony: string
  /** Background the town already knows ('' when the case has none). Shown to investigators only. */
  public_knowledge: string
}

/** POST /api/cases/{id}/start: `case_id` is this play-through, `story_id` the case. */
export interface CaseLock {
  case_id: string
  story_id: string
}

/**
 * One of the detective's own notes; `suspect_id` is null when it is about the case in general,
 * and `chat_message_id` is set when it is tied to one interrogation log entry.
 */
export interface CaseNoteItem {
  id: string
  suspect_id: string | null
  chat_message_id: string | null
  body: string
  created_at: string
}

export interface AddNoteInput {
  caseId: string
  body: string
  suspectId: string | null
  /** Ties the note to one interrogation log entry (a stored one, not a just-answered local one). */
  chatMessageId?: string
}

/** POST /api/cases/{id}/accuse: the settled outcome. The culprit is revealed only now that the case is closed. */
export interface AccusationOutcome {
  correct: boolean
  /** The player's trophy balance after the settlement. */
  trophies: number
  /** What the balance really moved by (negative for a loss, and a loss stops at zero). */
  trophies_change: number
  culprit_id: string
  culprit_name: string
  /** The full truth of the crime. */
  resolution: string
  /** What the revealed hints took off the reward (only when the case was solved). */
  hint_penalty: number
}

export const casesApi = createApi({
  name: 'cases',
  endpoints: (build) => ({
    getBriefing: build.query<Briefing, string>({
      query: (id) => `/api/cases/${id}/briefing`,
      providesTags: (_result, id) => [{ type: 'Cases', id }],
      staleTime: 600, // rarely changes; the server also lets the browser keep it
    }),
    getHints: build.query<HintState, string>({
      query: (id) => `/api/cases/${id}/hints`,
      providesTags: (_result, id) => [{ type: 'Hints', id }],
    }),
    getSuspects: build.query<Suspect[], string>({
      query: (id) => `/api/cases/${id}/suspects`,
      providesTags: (_result, id) => [{ type: 'Cases', id }],
      staleTime: 600,
    }),
    // 404 while the case is still open: the solution is never sent early
    getResult: build.query<AccusationOutcome, string>({
      // the browser keeps a verdict for a day: the user id in the URL keeps accounts apart (the server ignores it)
      query: (id) => `/api/cases/${id}/result?u=${currentUserId()}`,
      providesTags: (_result, id) => [{ type: 'Result', id }],
    }),
    getNotes: build.query<CaseNoteItem[], string>({
      query: (id) => `/api/cases/${id}/notes`,
      providesTags: (_result, id) => [{ type: 'Notes', id }],
    }),
    addNote: build.mutation<CaseNoteItem, AddNoteInput>({
      query: ({ caseId, body, suspectId, chatMessageId }) => ({
        url: `/api/cases/${caseId}/notes`,
        method: 'POST',
        body: { body, suspect_id: suspectId, chat_message_id: chatMessageId ?? null },
      }),
      invalidatesTags: (_result, arg) => [{ type: 'Notes', id: arg.caseId }],
    }),
    deleteNote: build.mutation<{ message: string }, { caseId: string; noteId: string }>({
      query: ({ caseId, noteId }) => ({ url: `/api/cases/${caseId}/notes/${noteId}`, method: 'DELETE' }),
      invalidatesTags: (_result, arg) => [{ type: 'Notes', id: arg.caseId }],
    }),
    // nothing is charged now: hint n takes n trophies off the reward if the case is solved
    revealHint: build.mutation<RevealedHint, string>({
      query: (caseId) => ({ url: `/api/cases/${caseId}/hints`, method: 'POST' }),
      invalidatesTags: (_result, caseId) => [{ type: 'Hints', id: caseId }],
    }),
    accuse: build.mutation<AccusationOutcome, { caseId: string; suspectId: string }>({
      query: ({ caseId, suspectId }) => ({
        url: `/api/cases/${caseId}/accuse`,
        method: 'POST',
        body: { suspect_id: suspectId },
      }),
      // the lock is gone and the trophies moved: stats, the profile, the leaderboard and the case view change
      invalidatesTags: (_result, arg) => ['Stats', 'User', 'Leaderboard', 'Stories', 'Cases', 'Hints', { type: 'Result', id: arg.caseId }],
    }),
    startCase: build.mutation<CaseLock, string>({
      query: (id) => ({ url: `/api/cases/${id}/start`, method: 'POST' }),
      // the dashboard's active-case banner and stats depend on the lock
      invalidatesTags: ['Stats', 'Cases', 'Hints'],
    }),
  }),
})

export const {
  useGetBriefingQuery,
  useGetHintsQuery,
  useGetSuspectsQuery,
  useGetResultQuery,
  useGetNotesQuery,
  useAddNoteMutation,
  useDeleteNoteMutation,
  useStartCaseMutation,
  useRevealHintMutation,
  useAccuseMutation,
} = casesApi
