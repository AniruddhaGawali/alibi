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
}

/** Ids of exchanges added here before the server's copy exists. */
const LOCAL_PREFIX = 'local-'
export const isLocalMessage = (m: Pick<ChatMessage, 'id'>) => m.id.startsWith(LOCAL_PREFIX)

export const chatApi = createApi({
  name: 'chat',
  endpoints: (build) => ({
    /**
     * Ask a suspect a question. Costs one token. The backend answers 422 with an in-character
     * line when the question is off topic (no token taken), 403 when out of tokens, and
     * 502/504 when the model fails (token refunded). The roadmap says POST /api/chat; the
     * built route carries the case and suspect in the path.
     */
    interrogate: build.mutation<InterrogateReply, InterrogateInput>({
      query: ({ caseId, suspectId, question }) => ({
        url: `/api/cases/${caseId}/suspects/${suspectId}/interrogate`,
        method: 'POST',
        body: { question },
      }),
      invalidatesTags: ['Stats'], // the balance changed on the server
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
        (m) => isLocalMessage(m) && !server.some((s) => s.user_query === m.user_query && s.ai_response === m.ai_response),
      )
      return [...server, ...waiting]
    },
  }
}

export function useSuspectChatHistory(caseId: string, suspectId: string) {
  return useQuery(historyQuery(caseId, suspectId))
}

/** Puts a just-answered exchange at the end of the cached history, so the chat and the logs show it at once. */
export function appendExchange(input: InterrogateInput, suspectName: string, reply: string): ChatMessage {
  const message: ChatMessage = {
    id: `${LOCAL_PREFIX}${crypto.randomUUID()}`,
    case_id: input.caseId,
    suspect_id: input.suspectId,
    suspect_name: suspectName,
    user_query: input.question,
    ai_response: reply,
    timestamp: new Date().toISOString(),
  }
  const key = historyKey(input.caseId, input.suspectId)
  queryClient.setQueryData<ChatMessage[]>(key, (old) => [...(old ?? []), message])
  // A moment later the storage worker has saved it: refetch so the entry gets its real id
  // (notes can only be tied to stored entries).
  setTimeout(() => void queryClient.invalidateQueries({ queryKey: key }), 2500)
  return message
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
