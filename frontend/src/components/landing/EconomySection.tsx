import { RiCoinsLine, RiLightbulbLine, RiRefreshLine, RiTrophyLine } from '@remixicon/react'
import { useGameRules } from '@/api/game'
import Reveal from './Reveal'

export default function EconomySection() {
  // the admin-set numbers; the prerendered page shows the defaults until they load
  const rules = useGameRules()
  const RULES = [
    { icon: RiCoinsLine, big: String(rules.daily_tokens), title: 'Tokens a day', text: `Every question to a suspect costs 1 token. You can hold up to ${rules.max_tokens}. Spend them wisely.` },
    { icon: RiRefreshLine, big: rules.token_reset_time, title: 'Daily reset', text: `Tokens refill every day (${rules.token_reset_timezone}). Unused ones do not roll over.` },
    { icon: RiTrophyLine, big: '+/−', title: 'Trophies', text: 'A correct accusation wins the case reward. A wrong one costs trophies.' },
    { icon: RiLightbulbLine, big: '1·3·6', title: 'Hints', text: 'Stuck? Reveal up to 3 hints. Each one is taken off your reward only if you win.' },
  ]

  return (
    <section className="landing-band border-y py-20">
      <div className="mx-auto max-w-6xl px-6">
        <Reveal className="mb-12 text-center">
          <h2 className="text-4xl text-primary">Tokens, trophies and hints</h2>
          <p className="mt-2 font-typewriter text-muted-foreground">Limited questions. Real stakes.</p>
        </Reveal>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {RULES.map((r, i) => (
            <Reveal key={r.title} delay={i * 100}>
              <div className="frame flex h-full flex-col items-center gap-2 border bg-card p-6 text-center transition-shadow duration-300 hover:shadow-[var(--glow-gold)]">
                <r.icon className="size-8 text-gold" aria-hidden />
                <p className="font-heading text-4xl text-gold">{r.big}</p>
                <h3 className="text-lg">{r.title}</h3>
                <p className="text-sm text-muted-foreground">{r.text}</p>
              </div>
            </Reveal>
          ))}
        </div>
        <Reveal className="mt-8 text-center">
          <p className="font-typewriter text-sm text-muted-foreground">
            The token numbers are set by the game masters and may change from time to time; this page always shows the current ones.
          </p>
        </Reveal>
      </div>
    </section>
  )
}
