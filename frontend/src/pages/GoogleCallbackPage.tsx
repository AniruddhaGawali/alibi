import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import LoadingScreen from '@/components/game/LoadingScreen'
import { useGoogleExchangeMutation } from '@/api/auth'
import { useStartSession } from '@/hooks/useStartSession'

/**
 * Where the backend sends the browser after Google. The URL fragment holds a one-time
 * ticket (never the tokens): swap it for the session, then go to the right place.
 */
export default function GoogleCallbackPage() {
  const navigate = useNavigate()
  const startSession = useStartSession()
  const started = useRef(false) // the ticket works once, and StrictMode runs effects twice
  const exchange = useGoogleExchangeMutation()

  useEffect(() => {
    if (started.current) return
    started.current = true
    const ticket = new URLSearchParams(window.location.hash.slice(1)).get('ticket')
    window.history.replaceState(null, '', window.location.pathname) // keep the ticket out of the history
    const fail = (message: string) => {
      toast.error(message)
      navigate('/login', { replace: true })
    }
    if (!ticket) return fail('Google sign-in did not finish. Please try again.')

    exchange
      .mutateAsync({ ticket })
      .then(async (session) => {
        const profile = await startSession(session)
        navigate(session.is_new ? '/onboarding/avatar' : profile.role === 'admin' ? '/admin/dashboard' : '/dashboard', { replace: true })
      })
      .catch((e: unknown) => fail(e instanceof Error ? e.message : 'Google sign-in failed'))
    // eslint-disable-next-line react-hooks/exhaustive-deps -- runs once
  }, [])

  return <LoadingScreen />
}
