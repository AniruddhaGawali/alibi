import { RiArrowDownSLine, RiArrowUpSLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'

/** Up / down buttons: the keyboard and touch way to reorder, next to drag and drop. */
export default function MoveButtons({
  index,
  count,
  label,
  onMove,
  disabled,
}: {
  index: number
  count: number
  /** What is moved, for screen readers ("The Crooked House Murders"). */
  label: string
  onMove: (from: number, to: number) => void
  disabled?: boolean
}) {
  return (
    <div className="flex flex-col">
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        disabled={disabled || index === 0}
        onClick={() => onMove(index, index - 1)}
        aria-label={`Move ${label} up`}
      >
        <RiArrowUpSLine aria-hidden />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        disabled={disabled || index === count - 1}
        onClick={() => onMove(index, index + 1)}
        aria-label={`Move ${label} down`}
      >
        <RiArrowDownSLine aria-hidden />
      </Button>
    </div>
  )
}
