import { Outlet, useLocation } from 'react-router'
import { useSyncPlayerMode } from '@/hooks/useSyncPlayerMode'
import { useSyncTokenBalance } from '@/hooks/useSyncTokenBalance'
import MusicPlayer from '@/components/game/MusicPlayer'
import Navbar from './Navbar'
import RouteProgress from './RouteProgress'

export default function Layout() {
  useSyncTokenBalance()
  useSyncPlayerMode()
  const { pathname } = useLocation()
  // the landing page has its own top bar and full-width sections
  const landing = pathname === '/'
  return (
    <div className="min-h-screen bg-background text-foreground">
      <RouteProgress />
      <MusicPlayer />
      {!landing && <Navbar />}
      {/* keyed by path so every page change replays the fade-in */}
      <main key={pathname} className={landing ? 'page-enter' : 'page-enter mx-auto max-w-7xl p-6'}>
        <Outlet />
      </main>
    </div>
  )
}
