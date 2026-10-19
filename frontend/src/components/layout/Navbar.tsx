import { Link } from 'react-router'
import { Button } from '@/components/ui/button'
import { useLoggedIn } from '@/hooks/useLoggedIn'
import MusicToggle from './MusicToggle'
import SoundToggle from './SoundToggle'
import UserMenu from './UserMenu'

export default function Navbar() {
  const loggedIn = useLoggedIn()

  return (
    <header className="border-b">
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-6 py-3">
        <Link to={loggedIn ? '/dashboard' : '/'} className="font-heading text-2xl text-primary no-underline">
          Alibi
        </Link>
        <div className="flex items-center gap-2">
        <MusicToggle />
        <SoundToggle />
        {loggedIn ? (
          <UserMenu />
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
