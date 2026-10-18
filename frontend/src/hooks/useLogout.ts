import { useNavigate } from 'react-router'
import { useLogoutMutation } from '@/api/auth'
import { clearSession } from '@/utils/session'
import { getRefreshToken } from '@/utils/cookieHelpers'

/**
 * Returns `logout()` and a pending flag. The server revokes the tokens, but the
 * user is signed out locally even if that call fails.
 */
export function useLogout() {
  const navigate = useNavigate()
  const mutation = useLogoutMutation({
    onSettled: () => {
      clearSession()
      navigate('/login', { replace: true })
    },
  })

  const logout = () => {
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
