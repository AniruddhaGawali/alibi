import { Link } from 'react-router'
import { RiSearchEyeLine } from '@remixicon/react'
import { useLoggedIn } from '@/hooks/useLoggedIn'

export default function LandingFooter() {
  const loggedIn = useLoggedIn()
  return (
    <footer className="border-t bg-card/50">
      <div className="mx-auto grid max-w-6xl gap-8 px-6 py-10 sm:grid-cols-3">
        <div>
          <p className="flex items-center gap-2 font-heading text-xl text-primary">
            <RiSearchEyeLine className="size-5" aria-hidden />
            Alibi
          </p>
          <p className="mt-2 text-sm text-muted-foreground">An AI detective mystery game. Every suspect has a story.</p>
        </div>
        <nav aria-label="Footer" className="flex flex-col gap-1 text-sm">
          <Link to="/how-to-play">How to play</Link>
          <a href="#leaderboard">Leaderboard</a>
        </nav>
        <nav aria-label="Account" className="flex flex-col gap-1 text-sm">
          {loggedIn ? (
            <Link to="/dashboard">Dashboard</Link>
          ) : (
            <>
              <Link to="/signup">Sign up</Link>
              <Link to="/login">Log in</Link>
            </>
          )}
        </nav>
      </div>
      <p className="border-t py-4 text-center font-typewriter text-xs text-muted-foreground">
        © Alibi. All suspects are fictional.
      </p>
    </footer>
  )
}
