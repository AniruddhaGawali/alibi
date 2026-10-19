import { queryClient } from '@/api/queryClient'
import { useGameStore } from '@/store/useGameStore'
import { useSessionStore } from '@/store/useSessionStore'
import { deleteJwt, deleteRefreshToken } from './cookieHelpers'

/** Forgets the signed-in user locally: cookies, cached server data and the session flag. */
export function clearSession(): void {
  deleteJwt()
  deleteRefreshToken()
  queryClient.clear()
  useGameStore.getState().setTokenBalance(null) // the next user must not see this balance
  useGameStore.setState({ trophies: null, outcomes: {}, hints: {} }) // nor these trophies or closed cases
  useSessionStore.getState().signOut()
}
