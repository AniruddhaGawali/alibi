import { Link } from 'react-router'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { STATS_POLL_MS, useGetStatsQuery } from '@/api/dashboard'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { useLogout } from '@/hooks/useLogout'
import { avatarSrc } from '@/utils/avatar'

/** Profile button on the right of the navbar; opens a menu with the user's details. */
export default function UserMenu() {
  const { data: user, isPending } = useCurrentUser()
  const { logout, isPending: loggingOut } = useLogout()
  const { data: stats } = useGetStatsQuery(undefined, { refetchInterval: STATS_POLL_MS })

  if (isPending || !user) return <Skeleton className="h-9 w-32" />

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="gap-2" aria-label="Open profile menu">
          <Avatar size="sm">
            <AvatarImage src={avatarSrc(user.avatar)} alt={user.username} />
            <AvatarFallback>{user.username.slice(0, 2).toUpperCase()}</AvatarFallback>
          </Avatar>
          <span className="hidden sm:inline">{user.username}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <DropdownMenuLabel className="flex items-center gap-3">
          <Avatar size="lg">
            <AvatarImage src={avatarSrc(user.avatar)} alt={user.username} />
            <AvatarFallback>{user.username.slice(0, 2).toUpperCase()}</AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <p className="truncate font-heading text-base">{user.username}</p>
            <p className="truncate text-xs font-normal text-muted-foreground">{user.email}</p>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <div className="space-y-1 px-2 py-1.5 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Tokens</span>
            <span className="text-gold">{stats?.tokens ?? user.token_balance}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Trophies</span>
            <span className="text-gold">{stats?.trophies ?? user.trophies}</span>
          </div>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/onboarding/avatar">Change avatar</Link>
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" disabled={loggingOut} onSelect={logout}>
          {loggingOut ? 'Logging out…' : 'Log out'}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
