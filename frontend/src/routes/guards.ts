import { redirect } from 'react-router'
import { hasSession } from '@/utils/cookieHelpers'

/** Login, signup and OTP pages: a signed-in user goes straight to the dashboard. */
export function guestOnlyLoader() {
  if (hasSession()) throw redirect('/dashboard')
  return null
}

/** Pages that need a signed-in user. An expired access token is fine: the API client refreshes it. */
export function requireAuthLoader() {
  if (!hasSession()) throw redirect('/login')
  return null
}
