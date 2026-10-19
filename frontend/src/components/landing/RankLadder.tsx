import RankBadge from '@/components/game/RankBadge'
import { RANKS } from '@/utils/rank'
import Reveal from './Reveal'

/** The four rank titles and the trophies each one needs. */
export default function RankLadder() {
  return (
    <section className="mx-auto max-w-4xl px-6 py-20 text-center">
      <Reveal>
        <h2 className="text-4xl text-primary">Climb the ranks</h2>
        <p className="mt-2 font-typewriter text-muted-foreground">Trophies turn a rookie into a legend.</p>
      </Reveal>
      <div className="mt-10 grid gap-4 sm:grid-cols-4">
        {RANKS.map((r, i) => (
          <Reveal key={r.name} delay={i * 120}>
            <div className="frame border bg-card p-5" style={{ marginTop: `${(3 - i) * 8}px` }}>
              <RankBadge trophies={r.min} className="text-base" />
              <p className="mt-2 font-typewriter text-sm text-muted-foreground">{r.min === 0 ? 'Where everyone starts' : `${r.min} trophies`}</p>
            </div>
          </Reveal>
        ))}
      </div>
    </section>
  )
}
