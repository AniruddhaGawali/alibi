import { useEffect } from 'react'
import { play } from '@/utils/sound'
import { cn } from '@/lib/utils'

/** Tilted red ink stamp that thumps onto its parent (which must be `relative`). */
export default function Stamp({ children, className }: { children: string; className?: string }) {
  useEffect(() => {
    play('stamp')
  }, [])
  return (
    <span aria-hidden className={cn('stamp', className)}>
      {children}
    </span>
  )
}
