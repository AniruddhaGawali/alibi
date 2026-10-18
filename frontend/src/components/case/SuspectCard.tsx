import { RiChat3Line } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import { useChatStore } from '@/store/useChatStore'
import { useGameStore } from '@/store/useGameStore'
import { CHAT_HISTORY_ID } from './ChatHistorySection'
import type { Suspect } from '@/api/cases'

/**
 * One suspect: portrait, name, relation to the victim, first statement and a Chat button.
 * Clicking the card itself filters the interrogation logs to this suspect and scrolls to them.
 */
export default function SuspectCard({ suspect }: { suspect: Suspect }) {
  const openChat = useChatStore((s) => s.openChat)
  const setSuspectFilter = useGameStore((s) => s.setSuspectFilter)

  const showLogs = () => {
    setSuspectFilter(suspect.id)
    document.getElementById(CHAT_HISTORY_ID)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  return (
    <article
      onClick={showLogs}
      className="frame flex w-72 shrink-0 cursor-pointer snap-start flex-col gap-4 bg-card p-5 sm:w-80"
    >
      <div className="flex items-center gap-4">
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
        <div className="min-w-0 space-y-1">
          <h3 className="truncate font-heading text-xl">{suspect.name}</h3>
          <Badge variant="secondary">{suspect.role}</Badge>
        </div>
      </div>
      <blockquote className="flex-1 border-l-2 border-primary/60 pl-3 font-typewriter text-sm leading-relaxed text-muted-foreground">
        “{suspect.initial_testimony}”
      </blockquote>
      <Button
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
