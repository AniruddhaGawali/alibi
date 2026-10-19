import { Link } from 'react-router'
import { Button } from '@/components/ui/button'
import type { AccusationOutcome } from '@/api/cases'
import PaperClip from '@/components/game/PaperClip'
import Stamp from '@/components/game/Stamp'
import Typewriter from '@/components/game/Typewriter'
import { cn } from '@/lib/utils'

/**
 * The closed case file: a verdict stamped onto the page, the truth of the crime, the
 * trophies it moved and the way back to the case files.
 */
export default function CaseOutcome({ outcome }: { outcome: AccusationOutcome }) {
  const won = outcome.correct
  const change = outcome.trophies_change
  return (
    <section className="case-folder mx-auto max-w-3xl" aria-label="Case outcome">
      <div className="case-sheet relative space-y-5 p-6 sm:p-8">
        <PaperClip />
        <div className="relative flex min-h-48 items-center justify-center">
          <Stamp className={cn('stamp-verdict', won ? 'stamp-verdict-won' : 'stamp-verdict-lost')}>
            {won ? 'Solved' : 'Wrong'}
          </Stamp>
        </div>
        <p className="sr-only">{won ? 'You named the culprit.' : 'You named the wrong suspect.'}</p>
        <h2 className="text-center font-heading text-2xl">
          {won ? `${outcome.culprit_name} is guilty` : `It was ${outcome.culprit_name}`}
        </h2>
        {outcome.resolution && (
          <p className="font-typewriter leading-relaxed whitespace-pre-line">
            <Typewriter text={outcome.resolution} />
          </p>
        )}
        <p className="text-center font-heading text-xl">
          {change === 0 ? 'Trophies unchanged' : `${change > 0 ? '+' : ''}${change} trophies`}
          <span className="font-typewriter text-sm text-paper-ink/70"> · you now hold {outcome.trophies}</span>
        </p>
        {outcome.hint_penalty > 0 && (
          <p className="text-center font-typewriter text-sm text-paper-ink/80">
            Hints cost you {outcome.hint_penalty} {outcome.hint_penalty === 1 ? 'trophy' : 'trophies'} of the reward.
          </p>
        )}
        <div className="flex justify-center">
          <Button asChild size="lg">
            <Link to="/dashboard">Return to Case Files</Link>
          </Button>
        </div>
      </div>
    </section>
  )
}
