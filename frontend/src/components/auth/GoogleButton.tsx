import { RiGoogleFill } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { BASE_URL } from '@/api/apiClient'

/**
 * Starts Google sign-in. The whole flow runs on our backend: it redirects the browser
 * to Google, handles the callback and sends the browser back to /auth/google/callback.
 */
export default function GoogleButton({ label = 'Continue with Google' }: { label?: string }) {
  return (
    <Button
      type="button"
      variant="outline"
      className="w-full"
      onClick={() => window.location.assign(`${BASE_URL}/api/auth/google/start`)}
    >
      <RiGoogleFill className="size-4" aria-hidden />
      {label}
    </Button>
  )
}
