import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { RiLightbulbLine, RiLockLine, RiSendPlane2Line } from '@remixicon/react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { FormTextarea } from '@/components/form'
import { Button } from '@/components/ui/button'
import { useGameStore } from '@/store/useGameStore'
import { cn } from '@/lib/utils'

/** Investigator-mode tips for cases whose admin wrote none; one is picked per conversation. */
const GENERIC_TIPS = [
  'Suspects lie. Cross-reference their timelines with what the others told you.',
  'Ask what they think of the other suspects.',
  'Ask about odd smells, sounds or visitors around the time of the crime.',
  'Press on details that changed between their first statement and now.',
  'Ask where they were, and who can confirm it.',
]

const schema = z.object({
  question: z.string().trim().min(1, 'Ask something first').max(500, 'Keep the question under 500 characters'),
})

/**
 * The question box. With no tokens left it locks: disabled, the placeholder tells the
 * player to make their final deduction, and the box turns red and greyed out.
 * Investigators get a detective tip under it; master sleuths do not.
 */
export default function ChatInput({
  suspectName,
  sending,
  onSend,
  tips,
}: {
  suspectName: string
  sending: boolean
  onSend: (question: string) => void
  /** The case's own tips (from the briefing); empty or missing = the generic ones. */
  tips?: string[]
}) {
  const tokens = useGameStore((s) => s.tokenBalance)
  const locked = tokens !== null && tokens <= 0
  const investigator = useGameStore((s) => s.playerMode === 'investigator')
  const [pick] = useState(Math.random)
  const pool = tips?.length ? tips : GENERIC_TIPS
  const tip = pool[Math.floor(pick * pool.length)]
  const { control, handleSubmit, reset } = useForm({ resolver: zodResolver(schema), defaultValues: { question: '' } })

  const submit = handleSubmit(({ question }) => {
    reset()
    onSend(question)
  })

  return (
    <form
      onSubmit={submit}
      aria-label={`Question ${suspectName}`}
      className={cn('space-y-2 border-t p-3', locked && 'cursor-not-allowed border-destructive bg-destructive/10')}
    >
      <div className="flex items-end gap-2">
        <div className="flex-1">
          <FormTextarea
            control={control}
            name="question"
            rows={2}
            maxLength={500}
            disabled={locked || sending || tokens === null}
            placeholder={locked ? 'Out of tokens. Make your final deduction.' : `Ask ${suspectName} a question…`}
            aria-label="Your question"
            className={cn('resize-none', locked && 'cursor-not-allowed border-destructive bg-muted text-muted-foreground placeholder:text-destructive')}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                void submit()
              }
            }}
          />
        </div>
        <Button type="submit" size="icon-lg" disabled={locked || sending || tokens === null} aria-label="Send question" className={cn(locked && 'cursor-not-allowed')}>
          <RiSendPlane2Line aria-hidden />
        </Button>
      </div>
      {locked ? (
        <p className="flex items-center gap-1.5 font-typewriter text-xs text-destructive">
          <RiLockLine className="size-4" aria-hidden /> No tokens left. The next ones arrive with the daily reset.
        </p>
      ) : (
        investigator && (
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground italic">
            <RiLightbulbLine className="size-4 shrink-0 text-gold" aria-hidden /> Detective tip: {tip}
          </p>
        )
      )}
    </form>
  )
}
