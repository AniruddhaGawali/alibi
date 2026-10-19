import { RANK_ICONS, RANK_TONES } from '@/components/game/rankIcons'
import { cn } from '@/lib/utils'
import { RANKS } from '@/utils/rank'
import Reveal from './Reveal'

const BLURBS = [
  'Where every detective starts.',
  'Your first cases are cracked.',
  'You read suspects like open books.',
  'The sharpest mind in the city.',
]

// each step stands taller than the last (from the sm breakpoint up), like a podium
const STEP_HEIGHT = ['sm:min-h-52', 'sm:min-h-60', 'sm:min-h-68', 'sm:min-h-76']

/** The four rank titles as rising steps, with the trophies each one needs. */
export default function RankLadder() {
  return (
    <section className="mx-auto max-w-5xl px-6 py-20 text-center">
      <Reveal>
        <h2 className="text-4xl text-primary">Climb the ranks</h2>
        <p className="mt-2 font-typewriter text-muted-foreground">Trophies turn a rookie into a legend.</p>
      </Reveal>
      <ol className="mt-12 grid grid-cols-2 gap-4 sm:grid-cols-4 sm:items-end">
        {RANKS.map((r, i) => {
          const Icon = RANK_ICONS[i]
          return (
            <li key={r.name} className="contents">
              <Reveal delay={i * 120} className="h-full sm:h-auto">
                <div
                  className={cn(
                    'frame group flex h-full flex-col items-center justify-center gap-3 border bg-card px-4 py-8',
                    'transition-transform duration-300 hover:-translate-y-2',
                    STEP_HEIGHT[i],
                  )}
                >
                  <span className="font-typewriter text-xs tracking-[0.3em] text-muted-foreground uppercase">Rank {i + 1}</span>
                  <Icon
                    className={cn('size-12 transition-transform duration-300 group-hover:scale-110', RANK_TONES[i])}
                    aria-hidden
                  />
                  <h3 className={cn('text-2xl', RANK_TONES[i])}>{r.name}</h3>
                  <p className="font-typewriter text-sm text-foreground">{r.min === 0 ? 'From the start' : `${r.min}+ trophies`}</p>
                  <p className="text-xs text-muted-foreground">{BLURBS[i]}</p>
                </div>
              </Reveal>
            </li>
          )
        })}
      </ol>
    </section>
  )
}
