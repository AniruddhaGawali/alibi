import { Link } from 'react-router'
import { Button } from '@/components/ui/button'
import { RiArrowRightLine, RiChat3Line, RiFileTextLine, RiScales3Line, RiSearchEyeLine } from '@remixicon/react'
import Reveal from './Reveal'

const STEPS = [
  { icon: RiFileTextLine, title: 'Read the case file', text: 'Study the crime, the scene photos and what each suspect told the police.' },
  { icon: RiChat3Line, title: 'Interrogate suspects', text: 'Ask anything. Each suspect is an AI with a secret, an alibi and a mood. Every question costs 1 token.' },
  { icon: RiSearchEyeLine, title: 'Catch the lie', text: 'Compare answers, keep detective notes, and press on the cracks. Evidence loosens tongues.' },
  { icon: RiScales3Line, title: 'Name the culprit', text: 'Make your final deduction. Be right to earn trophies. Be wrong and you lose some.' },
]

export default function HowToPlay() {
  return (
    <section id="how-to-play" className="mx-auto max-w-6xl scroll-mt-8 px-6 py-20">
      <Reveal className="mb-12 text-center">
        <h2 className="text-4xl">
          <Link to="/how-to-play" className="text-primary no-underline hover:underline">How to play</Link>
        </h2>
        <p className="mt-2 font-typewriter text-muted-foreground">Four steps from the crime scene to the verdict.</p>
      </Reveal>
      <ol className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
        {STEPS.map((s, i) => (
          <li key={s.title}>
            <Reveal delay={i * 120} className="h-full">
              <div className="frame group relative h-full border bg-card p-6 transition-transform duration-300 hover:-translate-y-2">
                <span className="absolute top-3 right-4 font-heading text-5xl text-primary/15">{i + 1}</span>
                <s.icon className="size-10 text-gold transition-transform duration-300 group-hover:scale-110 group-hover:rotate-6" aria-hidden />
                <h3 className="mt-4 text-xl">{s.title}</h3>
                <p className="mt-2 text-sm text-muted-foreground">{s.text}</p>
              </div>
            </Reveal>
          </li>
        ))}
      </ol>
      <Reveal className="mt-10 text-center">
        <Button asChild size="lg" variant="outline">
          <Link to="/how-to-play">
            Read the full rules
            <RiArrowRightLine className="size-5" aria-hidden />
          </Link>
        </Button>
      </Reveal>
    </section>
  )
}
