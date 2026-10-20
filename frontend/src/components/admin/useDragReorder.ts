import { useState, type DragEvent } from 'react'

/** Returns a copy of list with the item at from moved to index to. */
export function arrayMove<T>(list: T[], from: number, to: number): T[] {
  const next = [...list]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

/**
 * Native drag and drop for a list of items (no library). Spread `itemProps(i)` on each
 * item; `onMove(from, to)` is called on drop. `dragging` / `over` let the item style itself.
 */
export function useDragReorder(onMove: (from: number, to: number) => void) {
  const [dragging, setDragging] = useState<number | null>(null)
  const [over, setOver] = useState<number | null>(null)
  const reset = () => {
    setDragging(null)
    setOver(null)
  }

  const itemProps = (index: number) => ({
    draggable: true,
    onDragStart: (e: DragEvent<HTMLElement>) => {
      setDragging(index)
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/plain', String(index)) // Firefox only starts a drag with data set
    },
    onDragOver: (e: DragEvent<HTMLElement>) => {
      if (dragging === null) return // something else (a file, text) is being dragged
      e.preventDefault()
      e.dataTransfer.dropEffect = 'move'
      if (over !== index) setOver(index)
    },
    onDrop: (e: DragEvent<HTMLElement>) => {
      e.preventDefault()
      if (dragging !== null && dragging !== index) onMove(dragging, index)
      reset()
    },
    onDragEnd: reset,
  })

  return { itemProps, dragging, over }
}
