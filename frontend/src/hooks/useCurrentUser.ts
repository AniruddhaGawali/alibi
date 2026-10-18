import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '@/api/apiClient'
import { useGetUserDataQuery } from '@/api/user'
import { useSessionStore } from '@/store/useSessionStore'
import { clearSession } from '@/utils/session'

/** The signed-in user's profile. An expired or revoked token signs the user out. */
export function useCurrentUser() {
  const loggedIn = useSessionStore((s) => s.loggedIn)
  const navigate = useNavigate()
  const query = useGetUserDataQuery(undefined, { enabled: loggedIn })

  const unauthorized = query.error instanceof ApiError && query.error.status === 401
  useEffect(() => {
    if (unauthorized) {
      clearSession()
      navigate('/login', { replace: true })
    }
  }, [unauthorized, navigate])

  return query
}
