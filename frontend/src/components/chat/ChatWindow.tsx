import { useState } from 'react'
import { Skeleton } from '@/components/ui/skeleton'
import { ApiError } from '@/api/apiClient'
import { appendExchange, isFreshMessage, useInterrogateMutation, useSuspectChatHistory } from '@/api/chat'
import { invalidateTags } from '@/api/createApi'
import { queryClient } from '@/api/queryClient'
import { useGetBriefingQuery, type Suspect } from '@/api/cases'
import { useStickToBottom } from '@/hooks/useStickToBottom'
import { useGameStore } from '@/store/useGameStore'
import { timeAgo } from '@/utils/relativeTime'
import { play } from '@/utils/sound'
import ChatInput from './ChatInput'
import MessageBubble, { ThinkingBubble } from './MessageBubble'

/** A question that got no stored answer: blocked by moderation, or the model failed. Lives only in this window. */
interface Attempt {
  id: string
  at: number
  question: string
  /** blocked: the suspect's in-character brush-off; error: a notice, e.g. a timeout. */
  kind: 'blocked' | 'error'
  text: string
}

/**
 * The interrogation of one suspect: their stored conversation, a thinking indicator
 * while the model answers, and the question box. Sending a question takes a token at
 * once (the counter drops before the network call finishes) and gives it back if the
 * question is rejected, or the server says it refunded it. When the server kept the
 * token, or the outcome is unknown (network error), the balance is read again.
 */
export default function ChatWindow({ caseId, suspect }: { caseId: string; suspect: Suspect }) {
  const { data: history, isPending, error } = useSuspectChatHistory(caseId, suspect.id)
  const adjustTokens = useGameStore((s) => s.adjustTokenBalance)
  const setTokens = useGameStore((s) => s.setTokenBalance)
  const [attempts, setAttempts] = useState<Attempt[]>([])
  const { data: briefing } = useGetBriefingQuery(caseId) // already cached by the case page

  const send = useInterrogateMutation({
    onMutate: () => adjustTokens(-1), // optimistic: pay now, settle when the server answers
    onSuccess: (res, input) => {
      setTokens(res.tokens_remaining)
      appendExchange(input, suspect.name, res)
    },
    onError: (err, input) => {
      const api = err instanceof ApiError ? err : null
      const details = api?.details as { tokens?: string; token?: string } | undefined
      if (details?.tokens === 'out_of_tokens') setTokens(0)
      else if (details?.token === 'spent' || !api) void invalidateTags(queryClient, ['Stats']) // kept, or unknown: ask the server
      else adjustTokens(1) // the server took nothing, or refunded it
      play('error')
      const blocked = api?.status === 422 && (api.details as { question?: string } | undefined)?.question === 'off_topic'
      setAttempts((a) => [
        ...a,
        { id: crypto.randomUUID(), at: Date.now(), question: input.question, kind: blocked ? 'blocked' : 'error', text: err.message },
      ])
    },
  })

  type Line = { id: string; at: number; question: string; kind: 'ok' | 'blocked' | 'error'; text: string; fresh: boolean }
  const lines: Line[] = [
    ...(history ?? []).map((m): Line => ({ id: m.id, at: Date.parse(m.timestamp), question: m.user_query, kind: 'ok', text: m.ai_response, fresh: isFreshMessage(m.id) })),
    ...attempts.map((a): Line => ({ ...a, fresh: true })),
  ].sort((a, b) => a.at - b.at)

  const asking = send.isPending ? send.variables.question : null
  const { scrollRef, contentRef } = useStickToBottom<HTMLDivElement, HTMLDivElement>(lines.length + (asking ? 1 : 0))
  const portrait = suspect.avatar_url

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-4" aria-live="polite" aria-label={`Conversation with ${suspect.name}`}>
        <div ref={contentRef} className="space-y-4">
          <MessageBubble from="suspect" label={`${suspect.name} · first statement`} avatarUrl={portrait} text={suspect.initial_testimony} />

          {isPending ? (
            <Skeleton className="h-16 w-3/4" />
          ) : error ? (
            <MessageBubble from="notice" text={`Could not load the earlier conversation: ${error.message}`} />
          ) : null}

          {lines.map((l) => (
            <div key={l.id} className="space-y-4">
              <MessageBubble from="user" label="You" text={l.question} />
              {l.kind === 'error' ? (
                <MessageBubble from="notice" text={l.text} />
              ) : (
                <MessageBubble
                  from="suspect"
                  id={l.id}
                  label={`${suspect.name} · ${timeAgo(new Date(l.at).toISOString())}`}
                  avatarUrl={portrait}
                  text={l.text}
                  animate={l.fresh}
                  blocked={l.kind === 'blocked'}
                />
              )}
            </div>
          ))}

          {asking !== null && (
            <>
              <MessageBubble from="user" label="You" text={asking} />
              <ThinkingBubble name={suspect.name} avatarUrl={portrait} />
            </>
          )}
        </div>
      </div>

      <ChatInput
        suspectName={suspect.name}
        sending={send.isPending}
        onSend={(question) => send.mutate({ caseId, suspectId: suspect.id, question })}
        tips={briefing?.chat_tips}
      />
    </div>
  )
}
