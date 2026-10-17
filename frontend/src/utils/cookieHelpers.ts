const JWT_COOKIE = 'alibi_jwt'
const DEFAULT_MAX_AGE_SECONDS = 60 * 60 * 24 // 1 day

export interface CookieOptions {
  maxAgeSeconds?: number
}

function buildCookie(name: string, value: string, maxAge: number): string {
  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  return `${name}=${encodeURIComponent(value)}; Path=/; Max-Age=${maxAge}; SameSite=Strict${secure}`
}

export function getJwt(): string | null {
  for (const part of document.cookie.split('; ')) {
    const idx = part.indexOf('=')
    if (idx !== -1 && part.slice(0, idx) === JWT_COOKIE) {
      return decodeURIComponent(part.slice(idx + 1))
    }
  }
  return null
}

export function saveJwt(token: string, options: CookieOptions = {}): void {
  document.cookie = buildCookie(JWT_COOKIE, token, options.maxAgeSeconds ?? DEFAULT_MAX_AGE_SECONDS)
}

export function updateJwt(token: string, options: CookieOptions = {}): void {
  saveJwt(token, options)
}

export function deleteJwt(): void {
  document.cookie = buildCookie(JWT_COOKIE, '', 0)
}
