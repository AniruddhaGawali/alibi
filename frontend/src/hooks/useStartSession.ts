import { useGetUserDataQuery, type UserProfile } from '@/api/user'
import type { AuthSession } from '@/api/auth'
import { useSessionStore } from '@/store/useSessionStore'
import { saveJwt, saveRefreshToken, saveUserData } from '@/utils/cookieHelpers'

/**
 * Returns a function that finishes signing in: stores the JWT and refresh token
 * in cookies, loads the profile with the new token and caches it in the
 * `user_data` cookie. It throws if the profile cannot be loaded.
 */
export function useStartSession() {
  const profile = useGetUserDataQuery(undefined, { enabled: false })

  return async (session: AuthSession): Promise<UserProfile> => {
    saveJwt(session.jwt_token)
    saveRefreshToken(session.refresh_token)

    const { data, error } = await profile.refetch()
    if (!data) throw error ?? new Error('Could not load your profile')
    saveUserData(data)
    useSessionStore.getState().signIn()
    return data
  }
}
