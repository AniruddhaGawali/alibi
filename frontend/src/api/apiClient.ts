import { getJwt, getRefreshToken, saveJwt, saveRefreshToken } from '../utils/cookieHelpers'

export const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export class ApiError extends Error {
  status: number
  details?: unknown

  constructor(status: number, message: string, details?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.details = details
  }
}

/** Server errors and network failures are worth retrying; 4xx are not. */
export function isRetryableError(error: unknown): boolean {
  if (error instanceof ApiError) return error.status >= 500 || error.status === 429
  return true
}

interface Envelope<T> {
  status: 'success' | 'error'
  data?: T
  message?: string
  details?: unknown
}

/** Auth endpoints answer 401 for reasons a token refresh cannot fix (wrong password, bad code). */
const NO_REFRESH_PATHS = ['/api/auth/login', '/api/auth/signup', '/api/auth/verify-otp', '/api/auth/resend-otp', '/api/auth/refresh']

let onSessionExpired: (() => void) | null = null

/** Registered once at startup: called when the session cannot be renewed, to sign out and show the login page. */
export function setSessionExpiredHandler(handler: () => void): void {
  onSessionExpired = handler
}

interface RefreshedTokens {
  jwt_token: string
  refresh_token: string
  expires_in: number
}

/** The renewal already in flight. Refresh tokens work once, so concurrent 401s must share a single call. */
let refreshing: Promise<boolean> | null = null

/**
 * Runs fn while holding a lock shared by every tab of the browser, so two tabs never
 * rotate the refresh token at the same moment. Without the Web Locks API it just runs.
 */
function withTabLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== 'undefined' ? navigator.locks : undefined
  return locks ? locks.request('alibi-refresh', fn) : fn()
}

const REJECTED = [400, 401, 403, 422]

/**
 * Trades the refresh token for a new pair and stores it. Resolves true when the
 * session is renewed. When the server rejects the refresh token it signs the user out
 * (once, however many requests were waiting) and resolves false. A network failure or
 * server error also resolves false but leaves the session alone.
 *
 * `stale` is the access token the caller just found expired (null when its cookie is
 * gone). The cookies are shared by all tabs, so by the time this tab holds the lock
 * another tab may already have renewed: a different access token then means the work
 * is done. A rejection is only believed when the refresh cookie is still the one that
 * was sent; if it changed, another tab rotated it and this tab retries with the new one.
 */
function renew(stale: string | null): Promise<boolean> {
  refreshing ??= withTabLock(async () => {
    const fresh = getJwt()
    if (fresh && fresh !== stale) return true // another tab renewed while this one waited
    try {
      for (let attempt = 0; attempt < 2; attempt++) {
        const refresh = getRefreshToken()
        if (!refresh) return false
        const res = await fetch(`${BASE_URL}/api/auth/refresh`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify({ refresh_token: refresh }),
        })
        if (REJECTED.includes(res.status)) {
          const current = getRefreshToken()
          if (current && current !== refresh) continue // rotated by another tab: try its token
          onSessionExpired?.()
          return false
        }
        const body: Envelope<RefreshedTokens> | null = await res.json().catch(() => null)
        if (!res.ok || body?.status !== 'success' || !body.data) return false
        saveJwt(body.data.jwt_token, { maxAgeSeconds: body.data.expires_in })
        saveRefreshToken(body.data.refresh_token)
        return true
      }
      return false
    } catch {
      return false
    }
  }).finally(() => {
    refreshing = null
  })
  return refreshing
}

/**
 * Renews the access token when its cookie has expired but the refresh token is still
 * there. Call it before reading the refresh token for a request body (logout), since
 * renewing rotates the refresh token. Resolves false when there is nothing to renew
 * or the renewal failed.
 */
export async function ensureAccessToken(): Promise<boolean> {
  if (getJwt()) return true
  return getRefreshToken() ? renew(null) : false
}

function send(path: string, init: RequestInit): Promise<Response> {
  const headers = new Headers(init.headers)
  // FormData sets its own multipart boundary header; everything else is JSON.
  if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  headers.set('Accept', 'application/json')
  const token = getJwt()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return fetch(`${BASE_URL}${path}`, { ...init, headers })
}

/**
 * Calls the API. If the access token has expired (401), it renews the session with
 * the refresh token and repeats the request once, so the user is not sent back to
 * the login page every few minutes. Only a rejected refresh token ends the session.
 */
export async function apiClient<T>(path: string, init: RequestInit = {}): Promise<T> {
  const canRefresh = !NO_REFRESH_PATHS.includes(path)

  // The access cookie is gone but the refresh token is not: renew before even trying.
  if (canRefresh && !getJwt() && getRefreshToken() && !(await renew(null))) {
    throw new ApiError(401, 'Your session has expired. Please log in again.')
  }
  const tokenUsed = getJwt()

  let res = await send(path, init)
  if (res.status === 401 && canRefresh) {
    // Another tab may have renewed the session already: its new token is in the cookie.
    const current = getJwt()
    if ((current && current !== tokenUsed) || (await renew(tokenUsed))) {
      res = await send(path, init)
    }
  }

  const body: Envelope<T> | null = await res.json().catch(() => null)

  if (!res.ok || body?.status === 'error') {
    throw new ApiError(res.status, body?.message ?? res.statusText, body?.details)
  }
  return (body?.data ?? null) as T
}
