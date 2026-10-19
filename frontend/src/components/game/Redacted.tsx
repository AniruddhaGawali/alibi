import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/**
 * Text blacked out like a redacted document (still in the DOM for screen readers).
 * With `reveal` the bar lifts on hover, keyboard focus or a tap, showing the text underneath.
 */
export default function Redacted({ children, reveal = false }: { children: ReactNode; reveal?: boolean }) {
  return (
    <span className={cn('redacted', reveal && 'redacted-reveal')} {...(reveal && { tabIndex: 0 })}>
      {children}
    </span>
  )
}
