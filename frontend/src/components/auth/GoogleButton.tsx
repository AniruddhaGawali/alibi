import { toast } from 'sonner'
import { Button } from '@/components/ui/button'

/** Placeholder until Google login exists on the backend (Phase 1.1). */
export default function GoogleButton() {
  return (
    <Button type="button" variant="outline" className="w-full" onClick={() => toast.info('Google login is coming soon')}>
      Login with Google
    </Button>
  )
}
