import { getJwt } from '../utils/cookieHelpers'

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

export async function apiClient<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  // FormData sets its own multipart boundary header; everything else is JSON.
  if (init.body !== undefined && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  headers.set('Accept', 'application/json')

  const token = getJwt()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${BASE_URL}${path}`, { ...init, headers })

  const body: Envelope<T> | null = await res.json().catch(() => null)

  if (!res.ok || body?.status === 'error') {
    throw new ApiError(res.status, body?.message ?? res.statusText, body?.details)
  }
  return (body?.data ?? null) as T
}
