import { queryClient } from '@/api/queryClient'
import { useGameStore } from '@/store/useGameStore'
import { useSessionStore } from '@/store/useSessionStore'
import { deleteJwt, deleteRefreshToken, getJwt, getRefreshToken } from './cookieHelpers'

/** Forgets the signed-in user locally: cookies, cached server data and the session flag. */
export function clearSession(): void {
  deleteJwt()
  deleteRefreshToken()
  queryClient.clear()
  useGameStore.getState().setTokenBalance(null) // the next user must not see this balance
  useGameStore.setState({ trophies: null, outcomes: {}, hints: {} }) // nor these trophies or closed cases
  useSessionStore.getState().signOut()
}

/** The signed-in user's id, read from the token's `sub` claim (not secret; the server never trusts this). */
export function currentUserId(): string {
  for (const token of [getJwt(), getRefreshToken()]) {
    try {
      const payload = token?.split('.')[1]
      if (payload) {
        const { sub } = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/'))) as { sub?: string }
        if (sub) return sub
      }
    } catch {
      // a malformed token: try the next one
    }
  }
  return 'anon'
}
