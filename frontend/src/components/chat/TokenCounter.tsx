import { RiCoinsLine } from '@remixicon/react'
import { useGameStore } from '@/store/useGameStore'
import { cn } from '@/lib/utils'

/** The player's question tokens, read from the game store so it drops the moment a question is sent. */
export default function TokenCounter({ className }: { className?: string }) {
  const tokens = useGameStore((s) => s.tokenBalance)
  const empty = tokens !== null && tokens <= 0

  return (
    <div
      className={cn('flex items-center gap-1.5 border px-2.5 py-1 font-heading text-lg', empty ? 'border-destructive text-destructive' : 'border-primary/60 text-gold', className)}
      role="status"
      aria-label={tokens === null ? 'Question tokens' : `${tokens} question tokens left`}
      title="Each question costs 1 token"
    >
      <RiCoinsLine className="size-5" aria-hidden />
      <span aria-hidden>{tokens ?? '–'}</span>
    </div>
  )
}
