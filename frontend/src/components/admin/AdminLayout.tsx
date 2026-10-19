import { NavLink, Navigate, Outlet } from 'react-router'
import { RiGamepadLine, RiSettings3Line, RiStackLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { cn } from '@/lib/utils'

const LINKS = [
  { to: '/admin/dashboard', label: 'Cases', icon: RiStackLine },
  { to: '/admin/settings', label: 'Settings', icon: RiSettings3Line },
]

/**
 * Shell of every /admin page. Only admins get in; anyone else goes to the game
 * dashboard. This is for navigation only: the API refuses non-admins on its own.
 */
export default function AdminLayout() {
  const { data: user, isPending, isError } = useCurrentUser()

  if (isPending) return <Skeleton className="h-64 w-full" />
  if (isError || user.role !== 'admin') return <Navigate to="/dashboard" replace />

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p className="text-xs tracking-[0.3em] text-gold uppercase">Chief Inspector&apos;s office</p>
          <h1 className="text-3xl text-primary">Admin panel</h1>
        </div>
        <nav className="flex flex-wrap gap-2" aria-label="Admin">
          {LINKS.map(({ to, label, icon: Icon }) => (
            <Button key={to} asChild variant="ghost">
              <NavLink to={to} className={({ isActive }) => cn(isActive && 'text-gold underline underline-offset-8')}>
                <Icon className="size-4" aria-hidden />
                {label}
              </NavLink>
            </Button>
          ))}
          <Button asChild variant="outline">
            <NavLink to="/dashboard">
              <RiGamepadLine className="size-4" aria-hidden />
              Game
            </NavLink>
          </Button>
        </nav>
      </div>
      <Outlet />
    </div>
  )
}
