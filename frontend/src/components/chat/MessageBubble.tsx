import { RiErrorWarningLine } from '@remixicon/react'
import Typewriter from '@/components/game/Typewriter'
import { cn } from '@/lib/utils'

/** Answers already typed out (or skipped) in this tab, so reopening the chat does not retype them. */
const revealed = new Set<string>()

interface BubbleProps {
  from: 'user' | 'suspect' | 'notice'
  text: string
  /** Small line above the bubble: who is talking, and when. */
  label?: string
  /** Suspect portrait, shown beside their bubbles. */
  avatarUrl?: string
  /** Suspect bubbles only: type the text out when it first appears. Needs `id`. */
  animate?: boolean
  id?: string
  /** A suspect line that came from the game (moderation block), drawn with a red edge. */
  blocked?: boolean
}

/**
 * One line of the interrogation. The detective is on the right in a clean, modern
 * bubble; the suspect is on the left in a dark gold-edged card in typewriter type; notices (a
 * timeout, an error) are centred.
 */
export default function MessageBubble({ from, text, label, avatarUrl, animate = false, id, blocked }: BubbleProps) {
  if (from === 'notice') {
    return (
      <p role="alert" className="mx-auto flex max-w-[90%] items-start gap-2 border border-destructive/60 bg-destructive/10 px-3 py-2 font-typewriter text-xs text-destructive">
        <RiErrorWarningLine className="mt-0.5 size-4 shrink-0" aria-hidden />
        {text}
      </p>
    )
  }

  if (from === 'user') {
    return (
      <div className="ml-auto flex max-w-[85%] flex-col items-end gap-1">
        {label && <span className="text-xs text-muted-foreground">{label}</span>}
        <p className="relative bg-primary px-3.5 py-2 text-sm leading-relaxed whitespace-pre-wrap text-primary-foreground shadow-[var(--glow-gold)] after:absolute after:top-2.5 after:-right-1.5 after:border-y-[6px] after:border-l-[8px] after:border-y-transparent after:border-l-primary">
          {text}
        </p>
      </div>
    )
  }

  const typeIt = animate && id !== undefined && !revealed.has(id)
  return (
    <div className="flex max-w-[92%] items-start gap-2">
      {avatarUrl && <img src={avatarUrl} alt="" className="mt-5 size-9 shrink-0 border border-primary object-cover" />}
      <div className="flex min-w-0 flex-col items-start gap-1">
        {label && <span className="text-xs text-muted-foreground">{label}</span>}
        <p
          className={cn(
            'relative border border-primary/30 bg-gradient-to-br from-card to-background px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap text-card-foreground shadow-[0_2px_10px_oklch(0_0_0/50%)] after:absolute after:top-2.5 after:-left-[13px] after:border-y-[6px] after:border-r-[8px] after:border-y-transparent after:border-r-primary/40',
            blocked && 'border-destructive/60 from-destructive/10 after:border-r-destructive/60',
          )}
        >
          <Typewriter text={text} animate={typeIt} speedMs={14} onDone={() => id !== undefined && revealed.add(id)} />
        </p>
      </div>
    </div>
  )
}

/** The suspect is working out an answer: shown the moment a question is sent. */
export function ThinkingBubble({ name, avatarUrl }: { name: string; avatarUrl?: string }) {
  return (
    <div className="flex max-w-[92%] items-start gap-2" role="status" aria-live="polite">
      {avatarUrl && <img src={avatarUrl} alt="" className="size-9 shrink-0 border border-primary object-cover" />}
      <p className="flex items-center gap-2 border border-primary/30 bg-gradient-to-br from-card to-background px-4 py-3 font-typewriter text-sm text-muted-foreground">
        {name} is thinking
        <span aria-hidden className="flex gap-1">
          {[0, 1, 2].map((i) => (
            <span
              key={i}
              className="size-1.5 animate-pulse bg-current motion-reduce:animate-none"
              style={{ animationDelay: `${i * 200}ms` }}
            />
          ))}
        </span>
      </p>
    </div>
  )
}
