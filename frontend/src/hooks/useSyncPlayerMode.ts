import { useEffect } from 'react'
import { useGetUserDataQuery } from '@/api/user'
import { useGameStore } from '@/store/useGameStore'
import { useSessionStore } from '@/store/useSessionStore'

/** Mirrors the profile's difficulty mode into `useGameStore`. Mounted once, in the layout. */
export function useSyncPlayerMode() {
  const loggedIn = useSessionStore((s) => s.loggedIn)
  const { data } = useGetUserDataQuery(undefined, { enabled: loggedIn })
  const setPlayerMode = useGameStore((s) => s.setPlayerMode)
  const mode = data?.player_mode
  useEffect(() => {
    if (mode) setPlayerMode(mode)
  }, [mode, setPlayerMode])
}
