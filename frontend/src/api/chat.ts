import { useQueries, useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { apiClient, type ApiError } from './apiClient'
import { createApi } from './createApi'
import { queryClient } from './queryClient'
import { useGetSuspectsQuery } from './cases'

/** One recorded question and answer, as `GET /api/chat/{case_id}/{suspect_id}` returns it. */
export interface ChatMessage {
  id: string
  case_id: string
  suspect_id: string
  suspect_name: string
  user_query: string
  ai_response: string
  timestamp: string
  /** Client only: answered a moment ago, not in the stored history yet. Notes need a stored entry. */
  pending?: boolean
}

export interface InterrogateInput {
  caseId: string
  suspectId: string
  question: string
}

/** The suspect's answer and the player's balance after paying for it. */
export interface InterrogateReply {
  suspect_id: string
  reply: string
  tokens_remaining: number
  /** The id the exchange gets in the stored history, so it keeps one id from the first render. */
  message_id: string
}

/** Exchanges answered a moment ago that the server may not have stored yet. */
export const isPendingMessage = (m: Pick<ChatMessage, 'pending'>) => m.pending === true

/**
 * Ids of answers received in this tab. They are typed out when they first appear, and
 * stay marked after the stored copy replaces the local one, so the reveal is not cut short.
 */
const freshIds = new Set<string>()
export const isFreshMessage = (id: string) => freshIds.has(id)

export const chatApi = createApi({
  name: 'chat',
  endpoints: (build) => ({
    /**
     * Ask a suspect a question. Costs one token. The backend answers 422 with an in-character
     * line when the question is off topic (no token taken), 429 when out of tokens or over a
     * rate limit (see `details`), and 502/504 when the model fails (`details.token` says whether
     * the token was refunded). The roadmap says POST /api/chat; the built route carries the
     * case and suspect in the path.
     *
     * No `invalidatesTags: ['Stats']`: the reply carries `tokens_remaining`, and invalidating
     * would hold the answer back until the stats had been fetched again.
     */
    interrogate: build.mutation<InterrogateReply, InterrogateInput>({
      query: ({ caseId, suspectId, question }) => ({
        url: `/api/cases/${caseId}/suspects/${suspectId}/interrogate`,
        method: 'POST',
        body: { question },
      }),
    }),
  }),
})

export const { useInterrogateMutation } = chatApi

const historyKey = (caseId: string, suspectId: string) => ['chat', 'history', caseId, suspectId] as const

/**
 * The stored exchanges with one suspect, oldest first. Exchanges answered a moment ago
 * may not be in MongoDB yet (the storage worker runs behind a queue), so any local copy
 * the server has not caught up with is kept rather than dropped by the refetch.
 */
function historyQuery(caseId: string, suspectId: string): UseQueryOptions<ChatMessage[], ApiError> {
  const queryKey = historyKey(caseId, suspectId)
  return {
    queryKey,
    staleTime: 15_000,
    queryFn: async () => {
      const server = await apiClient<ChatMessage[]>(`/api/chat/${caseId}/${suspectId}`)
      const waiting = (queryClient.getQueryData<ChatMessage[]>(queryKey) ?? []).filter(
        (m) => isPendingMessage(m) && !server.some((s) => s.id === m.id),
      )
      return [...server, ...waiting]
    },
  }
}

export function useSuspectChatHistory(caseId: string, suspectId: string) {
  return useQuery(historyQuery(caseId, suspectId))
}

/** How long to wait before asking the server whether an answer has been stored, per attempt. */
const STORED_CHECK_DELAYS = [2500, 6000, 15000]

/**
 * Puts a just-answered exchange at the end of the cached history, so the chat and the
 * logs show it at once. It already has the id it will have in the stored history, so
 * nothing remounts when the stored copy arrives; the history is refetched a few times
 * until the storage worker has saved it (notes can only be tied to stored entries).
 */
export function appendExchange(input: InterrogateInput, suspectName: string, reply: InterrogateReply): ChatMessage {
  const message: ChatMessage = {
    id: reply.message_id,
    case_id: input.caseId,
    suspect_id: input.suspectId,
    suspect_name: suspectName,
    user_query: input.question,
    ai_response: reply.reply,
    timestamp: new Date().toISOString(),
    pending: true,
  }
  freshIds.add(message.id)
  const key = historyKey(input.caseId, input.suspectId)
  queryClient.setQueryData<ChatMessage[]>(key, (old) => [...(old ?? []), message])
  confirmStored(key, message.id, 0)
  return message
}

function confirmStored(key: ReturnType<typeof historyKey>, id: string, attempt: number) {
  const delay = STORED_CHECK_DELAYS[attempt]
  if (delay === undefined) return
  setTimeout(async () => {
    await queryClient.invalidateQueries({ queryKey: key })
    const stored = queryClient.getQueryData<ChatMessage[]>(key)?.find((m) => m.id === id)
    if (!stored || isPendingMessage(stored)) confirmStored(key, id, attempt + 1)
  }, delay)
}

/**
 * Every recorded exchange of a case across all its suspects, oldest first: one history
 * request per suspect, sharing the cache with the chat window.
 */
export function useCaseChatHistory(caseId: string) {
  const suspects = useGetSuspectsQuery(caseId)
  const results = useQueries({ queries: (suspects.data ?? []).map((s) => historyQuery(caseId, s.id)) })

  const error = suspects.error ?? results.find((r) => r.error)?.error ?? null
  const isPending = suspects.isPending || results.some((r) => r.isPending)
  const data =
    isPending || error
      ? undefined
      : results.flatMap((r) => r.data ?? []).sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp))
  return { data, isPending, error }
}
