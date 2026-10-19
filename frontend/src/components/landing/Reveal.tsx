import type { ReactNode } from 'react'
import { useReveal } from '@/hooks/useReveal'
import { cn } from '@/lib/utils'

/** Fades and lifts its children in when scrolled into view; `delay` (ms) staggers siblings. */
export default function Reveal({ children, delay = 0, className }: { children: ReactNode; delay?: number; className?: string }) {
  const ref = useReveal<HTMLDivElement>()
  return (
    <div ref={ref} className={cn('reveal', className)} style={{ transitionDelay: `${delay}ms` }}>
      {children}
    </div>
  )
}
