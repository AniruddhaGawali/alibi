import { cn } from '@/lib/utils'

/** Strip of masking tape across the top edge of a case file; parent must be `relative`. */
export default function Tape({ className }: { className?: string }) {
  return <span aria-hidden className={cn('tape', className)} />
}
