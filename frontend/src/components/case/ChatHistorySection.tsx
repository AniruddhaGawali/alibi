import { useEffect } from 'react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetNotesQuery, useGetSuspectsQuery, type CaseNoteItem } from '@/api/cases'
import { useCaseChatHistory, type ChatMessage } from '@/api/chat'
import { useGameStore } from '@/store/useGameStore'
import { cn } from '@/lib/utils'
import ChatLogCard from './ChatLogCard'
import NoteCard from './NoteCard'
import NoteComposer from './NoteComposer'

/** Id of the section, so suspect cards can scroll to it. */
export const CHAT_HISTORY_ID = 'chat-history'

/**
 * The case log: every interrogation and the detective's own notes in one stream, newest
 * first, filterable by suspect. A note box at the top adds to it. The stream scrolls
 * inside a fixed-height box so a long log never stretches the page.
 */
export default function ChatHistorySection({ caseId }: { caseId: string }) {
  const { data: suspects } = useGetSuspectsQuery(caseId)
  const { data: logs, isPending, error } = useCaseChatHistory(caseId)
  const { data: notes } = useGetNotesQuery(caseId)
  const filter = useGameStore((s) => s.selectedSuspectFilter)
  const setFilter = useGameStore((s) => s.setSuspectFilter)

  // a new case page starts unfiltered
  useEffect(() => () => setFilter(null), [setFilter])

  const total = logs?.length ?? 0
  const noteCount = notes?.length ?? 0
  const nameOf = (id: string | null) => suspects?.find((s) => s.id === id)?.name

  // logs and notes merged into one stream, newest first
  type Entry = { kind: 'log'; at: number; log: ChatMessage } | { kind: 'note'; at: number; note: CaseNoteItem }
  const entries: Entry[] = [
    ...(logs ?? []).filter((l) => !filter || l.suspect_id === filter).map((log): Entry => ({ kind: 'log', at: Date.parse(log.timestamp), log })),
    ...(notes ?? []).filter((n) => !filter || n.suspect_id === filter).map((note): Entry => ({ kind: 'note', at: Date.parse(note.created_at), note })),
  ].sort((a, b) => b.at - a.at)

  return (
    <section id={CHAT_HISTORY_ID} className="scroll-mt-6 space-y-5" aria-label="Interrogation logs">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="text-2xl">Detective&apos;s Case Notes &amp; Interrogation Logs</h2>
        <Badge variant="secondary" className="font-typewriter" title="Recorded exchanges and your notes">
          {total} {total === 1 ? 'log' : 'logs'} · {noteCount} {noteCount === 1 ? 'note' : 'notes'}
        </Badge>
      </div>

      <div className="flex flex-wrap gap-2" role="group" aria-label="Filter logs by suspect">
        <Pill active={filter === null} onClick={() => setFilter(null)}>
          All Logs
        </Pill>
        {suspects?.map((s) => (
          <Pill key={s.id} active={filter === s.id} onClick={() => setFilter(s.id)}>
            <img src={s.avatar_url} alt="" className="size-5 object-cover" />
            {s.name}
          </Pill>
        ))}
      </div>

      <NoteComposer caseId={caseId} />

      <div className="max-h-[36rem] space-y-5 overflow-y-auto p-1 pr-3">
        {isPending ? (
          <div className="grid gap-4 md:grid-cols-2">
            <Skeleton className="h-44" />
            <Skeleton className="h-44" />
          </div>
        ) : error ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load the interrogation logs</AlertTitle>
            <AlertDescription>{error.message}</AlertDescription>
          </Alert>
        ) : entries.length === 0 ? (
          <EmptyFolder>
            {filter
              ? 'Nothing recorded for this suspect yet. Add a note above or open the chat to question them.'
              : 'No interrogation logs recorded. Select a suspect above or open the chat to begin questioning.'}
          </EmptyFolder>
        ) : (
          <div className="grid gap-5 md:grid-cols-2">
            {entries.map((e) =>
              e.kind === 'log' ? (
                <ChatLogCard key={`l${e.log.id}`} log={e.log} caseId={caseId} avatarUrl={suspects?.find((s) => s.id === e.log.suspect_id)?.avatar_url} />
              ) : (
                <NoteCard
                  key={`n${e.note.id}`}
                  note={e.note}
                  caseId={caseId}
                  aboutName={nameOf(e.note.suspect_id)}
                  entry={logs?.find((l) => l.id === e.note.chat_message_id)}
                />
              ),
            )}
          </div>
        )}
      </div>
    </section>
  )
}

function Pill({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <Button
      type="button"
      size="sm"
      variant={active ? 'default' : 'outline'}
      aria-pressed={active}
      onClick={onClick}
      className={cn('gap-2', !active && 'text-muted-foreground')}
    >
      {children}
    </Button>
  )
}

/** A blank case-report folder for when there is nothing to show. */
function EmptyFolder({ children }: { children: React.ReactNode }) {
  return (
    <div className="case-folder relative mx-auto my-3 max-w-xl">
      <div className="case-sheet px-8 py-10 text-center font-typewriter">
        <p className="mb-3 text-sm tracking-[0.3em] text-paper-rust uppercase">Case report · blank</p>
        <p className="leading-relaxed">{children}</p>
      </div>
    </div>
  )
}
