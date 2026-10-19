import { useSessionStore } from '@/store/useSessionStore'
import { useHydrated } from './useHydrated'

/** Whether a user is signed in, hydration-safe (false in the prerendered HTML). */
export function useLoggedIn(): boolean {
  const hydrated = useHydrated()
  const loggedIn = useSessionStore((s) => s.loggedIn)
  return hydrated && loggedIn
}
