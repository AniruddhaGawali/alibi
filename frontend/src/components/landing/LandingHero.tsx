import { useRef } from 'react'
import { Link } from 'react-router'
import { RiArrowDownSLine, RiDoorOpenLine, RiPlayFill } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import Redacted from '@/components/game/Redacted'
import Typewriter from '@/components/game/Typewriter'
import { useLoggedIn } from '@/hooks/useLoggedIn'
import { play } from '@/utils/sound'

const TITLE = 'ALIBI'

/** Big title with a flashlight that follows the pointer, letters that lift on hover, and the main call to action. */
export default function LandingHero() {
  const loggedIn = useLoggedIn()
  const ref = useRef<HTMLElement>(null)

  // The pointer position goes straight into CSS variables: no re-render per mouse move.
  const move = (e: React.PointerEvent) => {
    const r = ref.current?.getBoundingClientRect()
    if (!r) return
    ref.current?.style.setProperty('--mx', `${e.clientX - r.left}px`)
    ref.current?.style.setProperty('--my', `${e.clientY - r.top}px`)
  }

  return (
    <section ref={ref} onPointerMove={move} className="landing-hero relative isolate overflow-hidden px-6 pt-32 pb-24 text-center">
      <div className="landing-spotlight" aria-hidden />
      <div className="landing-dust" aria-hidden />

      <p className="font-typewriter text-sm tracking-[0.4em] text-muted-foreground uppercase">
        <Redacted reveal>Case file</Redacted> · Eyes only
      </p>

      <h1 className="landing-title mt-4 font-heading text-7xl tracking-[0.12em] text-primary sm:text-9xl" aria-label={TITLE}>
        {TITLE.split('').map((ch, i) => (
          <span key={i} aria-hidden className="landing-letter" style={{ animationDelay: `${300 + i * 120}ms` }}>
            {ch}
          </span>
        ))}
      </h1>

      <p className="mx-auto mt-6 max-w-2xl text-xl text-foreground sm:text-2xl">
        <Typewriter text="Everyone has a story. One of them is lying." speedMs={45} />
      </p>
      <p className="mx-auto mt-3 max-w-xl text-muted-foreground">
        Interrogate AI suspects, catch them in a lie, and name the culprit before your questions run out.
      </p>

      <div className="mt-10 flex flex-wrap items-center justify-center gap-4">
        {loggedIn ? (
          <Button asChild size="lg" onClick={() => play('click')}>
            <Link to="/dashboard">
              <RiDoorOpenLine className="size-5" aria-hidden />
              Go to dashboard
            </Link>
          </Button>
        ) : (
          <>
            <Button asChild size="lg" className="landing-cta">
              <Link to="/signup">
                <RiPlayFill className="size-5" aria-hidden />
                Play now
              </Link>
            </Button>
            <Button asChild size="lg" variant="outline">
              <Link to="/login">Log in</Link>
            </Button>
          </>
        )}
      </div>

      <a href="#how-to-play" className="mt-16 inline-flex flex-col items-center text-sm text-muted-foreground no-underline hover:text-primary">
        How it works
        <RiArrowDownSLine className="landing-bounce size-6" aria-hidden />
      </a>
    </section>
  )
}
