import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { ensureAccessToken } from '@/api/apiClient'
import { useLogoutMutation } from '@/api/auth'
import { clearSession } from '@/utils/session'
import { getRefreshToken } from '@/utils/cookieHelpers'

/**
 * Returns `logout()` and a pending flag. The server revokes the tokens, and the
 * user is signed out locally even if that fails (they are told when it does).
 */
export function useLogout() {
  const navigate = useNavigate()
  const mutation = useLogoutMutation({
    onError: () => toast.error('You were signed out here, but the server could not revoke your session.'),
    onSettled: () => {
      clearSession()
      navigate('/login', { replace: true })
    },
  })

  const logout = async () => {
    // The server needs a live access token to revoke the session: renew it first when
    // it has expired, and only then read the (possibly rotated) refresh token.
    await ensureAccessToken()
    const refresh = getRefreshToken()
    if (refresh) {
      mutation.mutate({ refresh_token: refresh })
    } else {
      clearSession()
      navigate('/login', { replace: true })
    }
  }
  return { logout, isPending: mutation.isPending }
}
