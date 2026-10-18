const JWT_COOKIE = 'alibi_jwt'
const REFRESH_COOKIE = 'alibi_refresh'
const PENDING_OTP_EMAIL_COOKIE = 'pending_otp_email'

const DEFAULT_JWT_MAX_AGE_SECONDS = 15 * 60 // the backend's default access token lifetime
const REFRESH_MAX_AGE_SECONDS = 60 * 60 * 24 * 7 // matches the backend's 7 day refresh token
const PENDING_OTP_MAX_AGE_SECONDS = 60 * 60 // the signup code is only valid for minutes

export interface CookieOptions {
  maxAgeSeconds?: number
}

function buildCookie(name: string, value: string, maxAge: number): string {
  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  return `${name}=${encodeURIComponent(value)}; Path=/; Max-Age=${maxAge}; SameSite=Strict${secure}`
}

function readCookie(name: string): string | null {
  for (const part of document.cookie.split('; ')) {
    const idx = part.indexOf('=')
    if (idx !== -1 && part.slice(0, idx) === name) {
      return decodeURIComponent(part.slice(idx + 1))
    }
  }
  return null
}

function writeCookie(name: string, value: string, maxAgeSeconds: number): void {
  document.cookie = buildCookie(name, value, maxAgeSeconds)
}

function removeCookie(name: string): void {
  document.cookie = buildCookie(name, '', 0)
}

// --- JWT access token (read by apiClient only) ---

export function getJwt(): string | null {
  return readCookie(JWT_COOKIE)
}

/** Pass the server's `expires_in` as `maxAgeSeconds` so the cookie goes away when the token expires. */
export function saveJwt(token: string, options: CookieOptions = {}): void {
  writeCookie(JWT_COOKIE, token, options.maxAgeSeconds ?? DEFAULT_JWT_MAX_AGE_SECONDS)
}

export function deleteJwt(): void {
  removeCookie(JWT_COOKIE)
}

// --- Refresh token ---

/** True while the user can still be signed in: an access token, or a refresh token to get a new one. */
export function hasSession(): boolean {
  return readCookie(JWT_COOKIE) !== null || readCookie(REFRESH_COOKIE) !== null
}

export function getRefreshToken(): string | null {
  return readCookie(REFRESH_COOKIE)
}

export function saveRefreshToken(token: string, options: CookieOptions = {}): void {
  writeCookie(REFRESH_COOKIE, token, options.maxAgeSeconds ?? REFRESH_MAX_AGE_SECONDS)
}

export function deleteRefreshToken(): void {
  removeCookie(REFRESH_COOKIE)
}

// --- Email waiting for OTP verification (set at signup, deleted after verify) ---

export function getPendingOtpEmail(): string | null {
  return readCookie(PENDING_OTP_EMAIL_COOKIE)
}

export function savePendingOtpEmail(email: string): void {
  writeCookie(PENDING_OTP_EMAIL_COOKIE, email, PENDING_OTP_MAX_AGE_SECONDS)
}

export function deletePendingOtpEmail(): void {
  removeCookie(PENDING_OTP_EMAIL_COOKIE)
}
