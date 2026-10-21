import { RiChat3Line, RiSearchEyeLine } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import { useChatStore } from '@/store/useChatStore'
import { useGameStore } from '@/store/useGameStore'
import { CHAT_HISTORY_ID } from './ChatHistorySection'
import type { Suspect } from '@/api/cases'

/**
 * One suspect: portrait, name, relation to the victim, first statement and a Chat button.
 * Investigators also see the suspect's public background; master sleuths only the statement.
 * Clicking the card itself filters the interrogation logs to this suspect and scrolls to them.
 */
export default function SuspectCard({ suspect, chatLocked = false }: { suspect: Suspect; chatLocked?: boolean }) {
  const openChat = useChatStore((s) => s.openChat)
  const setSuspectFilter = useGameStore((s) => s.setSuspectFilter)
  const investigator = useGameStore((s) => s.playerMode === 'investigator')
  // ?? '': a suspect list cached before the field existed has no public_knowledge
  const background = investigator ? (suspect.public_knowledge ?? '').trim() : ''
  // first name on its own line, the rest of the name below it
  const [firstName, ...rest] = suspect.name.trim().split(/\s+/)
  const restName = rest.join(' ')

  const showLogs = () => {
    setSuspectFilter(suspect.id)
    document.getElementById(CHAT_HISTORY_ID)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  return (
    <article
      onClick={showLogs}
      className="frame flex h-full cursor-pointer flex-col gap-4 bg-card p-5"
    >
      <div className="flex items-start gap-4">
        <Tooltip>
          <TooltipTrigger asChild>
            <img
              src={suspect.avatar_url}
              alt={suspect.name}
              loading="lazy"
              className="size-20 shrink-0 border-2 border-primary object-cover shadow-[var(--glow-gold)]"
            />
          </TooltipTrigger>
          <TooltipContent side="bottom" className="max-w-xs font-typewriter text-sm leading-relaxed">
            “{suspect.initial_testimony}”
          </TooltipContent>
        </Tooltip>
        <div className="min-w-0 flex-1 space-y-2">
          <h3 className="font-heading text-xl leading-tight break-words" title={suspect.name}>
            <span className="block">{firstName}</span>
            {restName && <span className="block">{restName}</span>}
          </h3>
          <Badge variant="secondary" className="h-auto max-w-full text-left whitespace-normal break-words">
            {suspect.role}
          </Badge>
        </div>
      </div>
      <blockquote className="flex-1 border-l-2 border-primary/60 pl-3 font-typewriter text-sm leading-relaxed text-muted-foreground">
        “{suspect.initial_testimony}”
      </blockquote>
      {background && (
        <div className="space-y-1 border border-border bg-muted/40 p-3 text-sm">
          <p className="flex items-center gap-1.5 font-heading text-xs tracking-wide text-gold uppercase">
            <RiSearchEyeLine className="size-4" aria-hidden /> Public knowledge
          </p>
          <p className="whitespace-pre-line text-muted-foreground">{background}</p>
        </div>
      )}
      <Button
        disabled={chatLocked}
        title={chatLocked ? 'This case is closed' : undefined}
        onClick={(e) => {
          e.stopPropagation()
          openChat(suspect.id)
        }}
      >
        <RiChat3Line aria-hidden /> Chat
      </Button>
    </article>
  )
}
