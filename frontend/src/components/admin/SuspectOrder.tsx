import { useWatch, type Control } from 'react-hook-form'
import { RiDraggable, RiUserLine } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { CaseFormValues } from './caseSchema'
import MoveButtons from './MoveButtons'
import { useDragReorder } from './useDragReorder'

/**
 * A compact list of the suspects' names to reorder them without dragging the big cards.
 * The order is the one players see, and is saved with the case.
 */
export default function SuspectOrder({
  control,
  onMove,
  disabled,
}: {
  control: Control<CaseFormValues>
  onMove: (from: number, to: number) => void
  disabled?: boolean
}) {
  const suspects = useWatch({ control, name: 'suspects' })
  const { itemProps, dragging, over } = useDragReorder(onMove)

  return (
    <div className="space-y-2 border border-dashed p-3">
      <p className="text-sm text-muted-foreground">
        Order shown to players: drag a name or use the arrows. Saved with the case.
      </p>
      <ol className="grid gap-1.5 sm:grid-cols-2 lg:grid-cols-3">
        {suspects.map((s, i) => {
          const src = s.avatar.preview || s.avatar.url
          return (
            <li
              key={s.id || i}
              {...(disabled ? {} : itemProps(i))}
              className={cn(
                'flex items-center gap-2 border bg-card px-2 py-1 text-sm',
                !disabled && 'cursor-grab active:cursor-grabbing',
                dragging === i && 'opacity-40',
                over === i && dragging !== null && dragging !== i && 'border-primary shadow-[var(--glow-gold)]',
              )}
            >
              <RiDraggable className="size-4 shrink-0 text-muted-foreground" aria-hidden />
              <span className="w-5 shrink-0 text-right text-muted-foreground">{i + 1}.</span>
              {src ? (
                <img src={src} alt="" className="size-7 shrink-0 border object-cover" />
              ) : (
                <RiUserLine className="size-7 shrink-0 border p-1 text-muted-foreground" aria-hidden />
              )}
              <span className="min-w-0 flex-1 truncate">{s.name || 'New suspect'}</span>
              {s.is_culprit && <Badge variant="destructive">Culprit</Badge>}
              <MoveButtons index={i} count={suspects.length} label={s.name || `suspect ${i + 1}`} onMove={onMove} disabled={disabled} />
            </li>
          )
        })}
      </ol>
    </div>
  )
}
