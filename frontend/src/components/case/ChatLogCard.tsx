import { useState } from 'react'
import { RiChat3Line, RiQuillPenLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { useChatStore } from '@/store/useChatStore'
import { timeAgo } from '@/utils/relativeTime'
import { isLocalMessage, type ChatMessage } from '@/api/chat'
import LogNoteForm from './LogNoteForm'

/** One interrogation exchange: who was asked, the question, the typed answer, and a way back into the chat. */
export default function ChatLogCard({ log, caseId, avatarUrl }: { log: ChatMessage; caseId: string; avatarUrl?: string }) {
  const openChat = useChatStore((s) => s.openChat)
  const [noting, setNoting] = useState(false)
  // an entry answered a moment ago has no stored id yet; a note needs the stored one
  const saved = !isLocalMessage(log)

  return (
    <article className="frame space-y-3 bg-card p-5">
      <header className="flex items-center gap-3">
        {avatarUrl && <img src={avatarUrl} alt="" loading="lazy" className="size-10 shrink-0 border border-primary object-cover" />}
        <div className="min-w-0 flex-1">
          <h3 className="truncate font-heading text-lg">{log.suspect_name}</h3>
        </div>
        <time dateTime={log.timestamp} className="shrink-0 font-typewriter text-xs text-muted-foreground">
          {timeAgo(log.timestamp)}
        </time>
      </header>

      <blockquote className="border-l-2 border-primary/60 pl-3 text-sm">
        <span className="font-heading text-primary">Q:</span> “{log.user_query}”
      </blockquote>

      <p className="bg-background/60 p-3 font-typewriter text-sm leading-relaxed text-muted-foreground">{log.ai_response}</p>

      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" onClick={() => openChat(log.suspect_id)}>
          <RiChat3Line aria-hidden /> Resume Interrogation
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!saved || noting}
          title={saved ? undefined : 'Available in a moment, once this answer is saved'}
          onClick={() => setNoting(true)}
        >
          <RiQuillPenLine aria-hidden /> Add note
        </Button>
      </div>
      {noting && <LogNoteForm caseId={caseId} log={log} onDone={() => setNoting(false)} />}
    </article>
  )
}
