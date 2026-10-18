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
    startCase: build.mutation<CaseLock, string>({
      query: (id) => ({ url: `/api/cases/${id}/start`, method: 'POST' }),
      // the dashboard's active-case banner and stats depend on the lock
      invalidatesTags: ['Stats', 'Cases'],
    }),
  }),
})

export const {
  useGetBriefingQuery,
  useGetSuspectsQuery,
  useGetNotesQuery,
  useAddNoteMutation,
  useDeleteNoteMutation,
  useStartCaseMutation,
} = casesApi
