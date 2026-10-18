import { RiDeleteBinLine } from '@remixicon/react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useDeleteNoteMutation, type CaseNoteItem } from '@/api/cases'
import type { ChatMessage } from '@/api/chat'
import { timeAgo } from '@/utils/relativeTime'

/** The detective's own note, shown in the log among the interrogation entries on a paper slip. */
/** `entry` is the interrogation log entry the note is tied to, when it has been loaded. */
export default function NoteCard({ note, caseId, aboutName, entry }: { note: CaseNoteItem; caseId: string; aboutName?: string; entry?: ChatMessage }) {
  const remove = useDeleteNoteMutation({ onError: (e) => toast.error(e.message) })

  return (
    <article className="case-sheet space-y-2 p-4 font-typewriter text-sm">
      <header className="flex flex-wrap items-center gap-2">
        <Badge>Your note</Badge>
        <span className="text-xs text-paper-ink/80">{aboutName ? `about ${aboutName}` : 'about the case'}</span>
        <time dateTime={note.created_at} className="ml-auto text-xs text-paper-ink/70">
          {timeAgo(note.created_at)}
        </time>
      </header>
      {note.chat_message_id && (
        <blockquote className="border-l-2 border-paper-rust/60 pl-2 text-xs text-paper-ink/80">
          {entry ? (
            <>
              on the question <span className="italic">“{entry.user_query}”</span>
            </>
          ) : (
            'on an interrogation entry'
          )}
        </blockquote>
      )}
      <p className="break-words whitespace-pre-wrap">{note.body}</p>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        disabled={remove.isPending}
        onClick={() => remove.mutate({ caseId, noteId: note.id })}
        aria-label="Delete note"
        className="-ml-2 text-paper-red hover:bg-paper-red/10 hover:text-paper-red"
      >
        <RiDeleteBinLine className="size-4" aria-hidden /> Delete
      </Button>
    </article>
  )
}
