import { PLAYER_MODE_OPTIONS } from '@/components/settings/playerModes'
import Reveal from './Reveal'

/** Landing section: the two difficulty modes. */
export default function ModesBand() {
  return (
    <section id="modes" className="mx-auto max-w-5xl scroll-mt-8 px-6 py-20">
      <Reveal className="mb-12 text-center">
        <h2 className="text-4xl text-primary">Choose your badge</h2>
        <p className="mt-2 font-typewriter text-muted-foreground">Same cases, same stakes. You decide how much help you get.</p>
      </Reveal>
      <div className="grid gap-6 sm:grid-cols-2">
        {PLAYER_MODE_OPTIONS.map((m, i) => (
          <Reveal key={m.value} delay={i * 120} className="h-full">
            <div className="frame group flex h-full flex-col gap-3 border bg-card p-6 transition-transform duration-300 hover:-translate-y-2">
              {m.icon && <m.icon className="size-10 text-gold transition-transform duration-300 group-hover:scale-110" aria-hidden />}
              <h3 className="text-2xl">{m.label}</h3>
              <p className="text-muted-foreground">{m.description}</p>
            </div>
          </Reveal>
        ))}
      </div>
      <Reveal className="mt-8 text-center">
        <p className="font-typewriter text-sm text-muted-foreground">
          Switch any time from Settings.{' '}
          {/* a plain link: a router Link would not scroll to the #modes anchor on the other page */}
          <a href="/how-to-play#modes" className="text-primary hover:underline">
            Compare the modes
          </a>
        </p>
      </Reveal>
    </section>
  )
}
