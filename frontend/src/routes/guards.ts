import { redirect } from 'react-router'
import { getJwt } from '@/utils/cookieHelpers'

/** Login, signup and OTP pages: a signed-in user goes straight to the dashboard. */
export function guestOnlyLoader() {
  if (getJwt()) throw redirect('/dashboard')
  return null
}

/** Pages that need a signed-in user. */
export function requireAuthLoader() {
  if (!getJwt()) throw redirect('/login')
  return null
}
