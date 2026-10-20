import { useState } from 'react'
import { Link } from 'react-router'
import { RiSettings3Line } from '@remixicon/react'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import RankBadge from '@/components/game/RankBadge'
import SettingsDialog, { type SettingsPanel } from '@/components/settings/SettingsDialog'
import { PLAYER_MODE_LABELS } from '@/components/settings/playerModes'
import { Skeleton } from '@/components/ui/skeleton'
import { STATS_POLL_MS, useGetStatsQuery } from '@/api/dashboard'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { useLogout } from '@/hooks/useLogout'
import { useGameStore } from '@/store/useGameStore'
import { avatarSrc } from '@/utils/avatar'

/** Profile button on the right of the navbar; opens a menu with the user's details. */
export default function UserMenu() {
  const { data: user, isPending, isError, refetch } = useCurrentUser()
  const { logout, isPending: loggingOut } = useLogout()
  const { data: stats } = useGetStatsQuery(undefined, { refetchInterval: STATS_POLL_MS })
  const tokens = useGameStore((s) => s.tokenBalance)
  const trophies = useGameStore((s) => s.trophies)
  const playerMode = useGameStore((s) => s.playerMode)
  const [panel, setPanel] = useState<SettingsPanel | null>(null)

  if (isError && !user) {
    // The profile failed to load: never leave the user without a way out.
    return (
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => void refetch()}>
          Retry
        </Button>
        <Button variant="ghost" size="sm" disabled={loggingOut} onClick={logout}>
          {loggingOut ? 'Logging out…' : 'Log out'}
        </Button>
      </div>
    )
  }
  if (isPending || !user) return <Skeleton className="h-9 w-32" />

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" className="gap-2" aria-label="Open profile menu">
            <Avatar size="sm">
              <AvatarImage src={avatarSrc(user.avatar)} alt={user.username} />
              <AvatarFallback>{user.username.slice(0, 2).toUpperCase()}</AvatarFallback>
            </Avatar>
            <span className="hidden sm:inline">{user.username}</span>
            <RankBadge trophies={trophies ?? stats?.trophies ?? user.trophies} compact className="hidden sm:inline-flex" />
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
              <RankBadge trophies={trophies ?? stats?.trophies ?? user.trophies} />
              <p className="truncate text-xs font-normal text-muted-foreground">{user.email}</p>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <div className="space-y-1 px-2 py-1.5 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Tokens</span>
              <span className="text-gold">{tokens ?? stats?.tokens ?? user.token_balance}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Trophies</span>
              <span className="text-gold">{trophies ?? stats?.trophies ?? user.trophies}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Mode</span>
              <span className="text-gold">{PLAYER_MODE_LABELS[playerMode]}</span>
            </div>
          </div>
          <DropdownMenuSeparator />
          {user.role === 'admin' && (
            <DropdownMenuItem asChild>
              <Link to="/admin/dashboard">Admin panel</Link>
            </DropdownMenuItem>
          )}
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <RiSettings3Line aria-hidden /> Settings
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              <DropdownMenuItem onSelect={() => setPanel('avatar')}>Change avatar</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setPanel('mode')}>Change difficulty mode</DropdownMenuItem>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuItem variant="destructive" disabled={loggingOut} onSelect={logout}>
            {loggingOut ? 'Logging out…' : 'Log out'}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <SettingsDialog panel={panel} onClose={() => setPanel(null)} />
    </>
  )
}
