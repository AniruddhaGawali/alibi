import { Outlet } from 'react-router'
import { useSyncTokenBalance } from '@/hooks/useSyncTokenBalance'
import Navbar from './Navbar'

export default function Layout() {
  useSyncTokenBalance()
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Navbar />
      <main className="mx-auto max-w-7xl p-6">
        <Outlet />
      </main>
    </div>
  )
}
