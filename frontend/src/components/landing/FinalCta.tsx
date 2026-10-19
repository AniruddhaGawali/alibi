import { Link } from 'react-router'
import { Button } from '@/components/ui/button'
import { useLoggedIn } from '@/hooks/useLoggedIn'
import Reveal from './Reveal'

export default function FinalCta() {
  const loggedIn = useLoggedIn()
  return (
    <section className="mx-auto max-w-3xl px-6 py-24 text-center">
      <Reveal>
        <h2 className="text-4xl text-primary sm:text-5xl">The case is waiting, detective.</h2>
        <p className="mt-3 font-typewriter text-muted-foreground">Ten tokens. A room full of liars. Free to start.</p>
        <Button asChild size="lg" className="landing-cta mt-8">
          <Link to={loggedIn ? '/dashboard' : '/signup'}>{loggedIn ? 'Go to dashboard' : 'Take the case'}</Link>
        </Button>
      </Reveal>
    </section>
  )
}
