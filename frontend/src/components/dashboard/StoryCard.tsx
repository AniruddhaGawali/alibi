import { RiTrophyLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardFooter, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import Pin from '@/components/game/Pin'
import Redacted from '@/components/game/Redacted'
import Stamp from '@/components/game/Stamp'
import Tape from '@/components/game/Tape'
import type { Difficulty, Story } from '@/api/dashboard'

const DIFFICULTY: Record<Difficulty, { label: string; variant: 'secondary' | 'default' | 'destructive' }> = {
  easy: { label: 'Easy', variant: 'secondary' },
  medium: { label: 'Medium', variant: 'default' },
  hard: { label: 'Hard', variant: 'destructive' },
}

/**
 * A case card. The button says Continue for the case already in progress, Start otherwise, and
 * View case file for one the player has closed (stamped SOLVED or CLOSED; it is played once).
 * While another case is in progress the card is locked: redacted, stamped CLASSIFIED, no button.
 */
export default function StoryCard({
  story,
  inProgress = false,
  locked = false,
  onOpen,
  busy = false,
}: {
  story: Story
  inProgress?: boolean
  /** Another case is in progress, so this one cannot be started yet. */
  locked?: boolean
  /** Called by the Start / Continue button. */
  onOpen?: (story: Story) => void
  /** A start request is in flight; the button is disabled. */
  busy?: boolean
}) {
  const difficulty = DIFFICULTY[story.difficulty_level]

  return (
    <div className="relative z-10 pt-2">
      <Tape />
      <Pin seed={story.id} />
      <Card className="relative pt-0">
        {story.thumbnail_url ? (
          <img src={story.thumbnail_url} alt="" loading="lazy" className="aspect-video w-full object-cover" />
        ) : (
          <div className="aspect-video w-full bg-muted" aria-hidden />
        )}
        <CardHeader>
          <div className="flex items-start justify-between gap-2">
            <CardTitle className="font-heading text-xl">{story.title}</CardTitle>
            <Badge variant={difficulty.variant}>{difficulty.label}</Badge>
          </div>
          <CardDescription className="line-clamp-3 font-typewriter">
            {locked ? <Redacted>{story.synopsis}</Redacted> : story.synopsis}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex gap-4 text-sm">
          <span className="flex items-center gap-1 text-gold" title="Trophies for solving it">
            <RiTrophyLine className="size-4" aria-hidden />
            {story.reward_trophies}
          </span>
        </CardContent>
        <CardFooter>
          <Button className="w-full" disabled={busy || locked} onClick={() => onOpen?.(story)}>
            {story.outcome ? 'View case file' : locked ? 'Locked' : inProgress ? 'Continue' : 'Start'}
          </Button>
        </CardFooter>
        {locked && <Stamp>Classified</Stamp>}
        {story.outcome && <Stamp>{story.outcome === 'solved' ? 'Solved' : 'Closed'}</Stamp>}
      </Card>
    </div>
  )
}
