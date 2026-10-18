import type { FieldValues, Path, UseFormSetError } from 'react-hook-form'
import { toast } from 'sonner'
import { ApiError } from '@/api/apiClient'

/**
 * Shows a failed request: field-level `details` from the backend
 * ({"email": "is already registered"}) go next to the matching fields, anything
 * else becomes a toast with the server's message.
 */
export function showApiError<T extends FieldValues>(
  error: Error,
  setError?: UseFormSetError<T>,
  fields: readonly Path<T>[] = [],
): void {
  if (setError && error instanceof ApiError && error.details && typeof error.details === 'object') {
    let placed = false
    for (const [key, msg] of Object.entries(error.details as Record<string, unknown>)) {
      if (typeof msg === 'string' && (fields as readonly string[]).includes(key)) {
        setError(key as Path<T>, { type: 'server', message: `${key.charAt(0).toUpperCase()}${key.slice(1)} ${msg}` })
        placed = true
      }
    }
    if (placed) return
  }
  toast.error(error.message)
}
