import { Link } from 'react-router'
import { RiSearchEyeLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import MusicToggle from '@/components/layout/MusicToggle'
import SoundToggle from '@/components/layout/SoundToggle'
import { useLoggedIn } from '@/hooks/useLoggedIn'

/** Slim bar over the hero: logo, sound controls and the sign-in buttons. */
export default function LandingTopBar() {
  const loggedIn = useLoggedIn()
  return (
    <header className="absolute inset-x-0 top-0 z-20">
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
        <Link to="/" className="flex items-center gap-2 font-heading text-2xl text-primary no-underline">
          <RiSearchEyeLine className="size-6" aria-hidden />
          Alibi
        </Link>
        <div className="flex items-center gap-2">
          <Button asChild variant="ghost" className="hidden sm:inline-flex">
            <Link to="/how-to-play">How to play</Link>
          </Button>
          <MusicToggle />
          <SoundToggle />
          {loggedIn ? (
            <Button asChild>
              <Link to="/dashboard">Dashboard</Link>
            </Button>
          ) : (
            <>
              <Button asChild variant="ghost">
                <Link to="/login">Log in</Link>
              </Button>
              <Button asChild>
                <Link to="/signup">Sign up</Link>
              </Button>
            </>
          )}
        </div>
      </nav>
    </header>
  )
}
