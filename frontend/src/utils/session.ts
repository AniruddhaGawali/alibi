import { queryClient } from '@/api/queryClient'
import { useSessionStore } from '@/store/useSessionStore'
import { deleteJwt, deleteRefreshToken, deleteUserData } from './cookieHelpers'

/** Forgets the signed-in user locally: cookies, cached server data and the session flag. */
export function clearSession(): void {
  deleteJwt()
  deleteRefreshToken()
  deleteUserData()
  queryClient.clear()
  useSessionStore.getState().signOut()
}
