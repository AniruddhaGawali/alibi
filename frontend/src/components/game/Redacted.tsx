import type { ReactNode } from 'react'

/** Text blacked out like a redacted document (still in the DOM for screen readers). */
export default function Redacted({ children }: { children: ReactNode }) {
  return <span className="redacted">{children}</span>
}
