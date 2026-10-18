import { useGetUserDataQuery, type UserProfile } from '@/api/user'
import type { AuthSession } from '@/api/auth'
import { useSessionStore } from '@/store/useSessionStore'
import { saveJwt, saveRefreshToken } from '@/utils/cookieHelpers'
import { clearSession } from '@/utils/session'

/**
 * Returns a function that finishes signing in: stores the JWT and refresh token
 * in cookies and loads the profile with the new token. If the profile cannot be
 * loaded the session is cleared again (so the guest-only pages do not bounce the
 * user to the dashboard with no way back) and the error is thrown.
 */
export function useStartSession() {
  const profile = useGetUserDataQuery(undefined, { enabled: false })

  return async (session: AuthSession): Promise<UserProfile> => {
    saveJwt(session.jwt_token, { maxAgeSeconds: session.expires_in })
    saveRefreshToken(session.refresh_token)

    const { data, error } = await profile.refetch()
    if (!data) {
      clearSession()
      throw error ?? new Error('Could not load your profile')
    }
    useSessionStore.getState().signIn()
    return data
  }
}
