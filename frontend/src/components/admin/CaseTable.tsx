import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { RiArrowGoBackLine, RiDeleteBinLine, RiDraggable, RiEditLine, RiSave3Line } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useDeleteAdminCaseMutation, useGetAdminCasesQuery, useReorderAdminCasesMutation, type AdminCaseSummary } from '@/api/admin'
import { cn } from '@/lib/utils'
import { MIN_SUSPECTS } from './caseSchema'
import MoveButtons from './MoveButtons'
import { arrayMove, useDragReorder } from './useDragReorder'

/**
 * Every case, drafts included, with whether it can be published. Rows can be dragged (or
 * moved with the arrows) into a new order, which is only sent when "Save order" is pressed.
 */
export default function CaseTable() {
  const { data: cases, isPending, error, refetch } = useGetAdminCasesQuery()
  const [toDelete, setToDelete] = useState<AdminCaseSummary>()
  /** The unsaved order (case ids); null = the server's order. */
  const [order, setOrder] = useState<string[] | null>(null)
  const serverIds = (cases ?? []).map((c) => c.id)
  // Cases deleted meanwhile drop out of the draft order; new ones go to the top, as the server puts them.
  const ids = order ? [...serverIds.filter((id) => !order.includes(id)), ...order.filter((id) => serverIds.includes(id))] : serverIds
  const rows = ids.map((id) => cases!.find((c) => c.id === id)!)
  const dirty = order !== null && ids.some((id, i) => id !== serverIds[i])
  const move = (from: number, to: number) => setOrder(arrayMove(ids, from, to))
  const { itemProps, dragging, over } = useDragReorder(move)
  const saveOrder = useReorderAdminCasesMutation({
    onSuccess: () => {
      toast.success('Case order saved')
      setOrder(null)
    },
    onError: (e) => toast.error(e.message),
  })
  const del = useDeleteAdminCaseMutation({
    onSuccess: () => {
      toast.success('Case deleted')
      setToDelete(undefined)
    },
    onError: (e) => toast.error(e.message),
  })

  if (isPending) return <Skeleton className="h-48 w-full" />
  if (error) {
    return (
      <div className="flex items-center gap-3 text-sm text-destructive">
        {error.message}
        <Button variant="outline" size="sm" onClick={() => void refetch()}>
          Retry
        </Button>
      </div>
    )
  }
  if (cases.length === 0) return <p className="text-sm text-muted-foreground">No cases yet. Create the first one.</p>

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          Drag the rows (or use the arrows) to change the order. Players see the published cases in this order.
        </p>
        {dirty && (
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={saveOrder.isPending} onClick={() => setOrder(null)}>
              <RiArrowGoBackLine className="size-4" aria-hidden />
              Reset
            </Button>
            <Button size="sm" disabled={saveOrder.isPending} onClick={() => saveOrder.mutate(ids)}>
              <RiSave3Line className="size-4" aria-hidden />
              {saveOrder.isPending ? 'Saving…' : 'Save order'}
            </Button>
          </div>
        )}
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-16">
              <span className="sr-only">Order</span>
            </TableHead>
            <TableHead>Case</TableHead>
            <TableHead>Difficulty</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="text-right">Suspects</TableHead>
            <TableHead className="text-right">Trophies</TableHead>
            <TableHead className="text-right">Plays</TableHead>
            <TableHead className="sr-only">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((c, i) => {
            const ready = c.suspect_count >= MIN_SUSPECTS && c.culprit_count === 1
            return (
              <TableRow
                key={c.id}
                {...itemProps(i)}
                className={cn(
                  'cursor-grab active:cursor-grabbing',
                  dragging === i && 'opacity-40',
                  // the row lands after the target when moving down, before it when moving up
                  over === i && dragging !== null && dragging < i && 'border-b-2 border-b-primary',
                  over === i && dragging !== null && dragging > i && 'border-t-2 border-t-primary',
                )}
              >
                <TableCell>
                  <div className="flex items-center gap-1">
                    <RiDraggable className="size-4 text-muted-foreground" aria-hidden />
                    <MoveButtons index={i} count={rows.length} label={c.title} onMove={move} disabled={saveOrder.isPending} />
                  </div>
                </TableCell>
                <TableCell className="max-w-72">
                  <p className="truncate font-medium">{c.title}</p>
                  <p className="truncate text-xs text-muted-foreground">{c.synopsis}</p>
                </TableCell>
                <TableCell className="capitalize">{c.difficulty_level}</TableCell>
                <TableCell>
                  {c.is_published ? (
                    <Badge>Published</Badge>
                  ) : (
                    <Badge variant="outline" title={ready ? 'Ready to publish' : `Needs ${MIN_SUSPECTS}+ suspects and one culprit`}>
                      Draft{ready ? '' : ' · not ready'}
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-right">
                  {c.suspect_count}
                  {c.culprit_count !== 1 && <span className="ml-1 text-destructive" title="Needs exactly one culprit">!</span>}
                </TableCell>
                <TableCell className="text-right text-gold">
                  +{c.reward_trophies} / −{c.penalty_trophies}
                </TableCell>
                <TableCell className="text-right">{c.plays}</TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <Button asChild variant="ghost" size="sm">
                      <Link to={`/admin/cases/${c.id}`}>
                        <RiEditLine className="size-4" aria-hidden />
                        Edit
                      </Link>
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={c.plays > 0}
                      title={c.plays > 0 ? 'Played cases cannot be deleted; unpublish instead' : undefined}
                      onClick={() => setToDelete(c)}
                    >
                      <RiDeleteBinLine className="size-4" aria-hidden />
                      <span className="sr-only">Delete {c.title}</span>
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>

      <Dialog open={!!toDelete} onOpenChange={(open) => !open && setToDelete(undefined)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete “{toDelete?.title}”?</DialogTitle>
            <DialogDescription>
              Its suspects, scenes and uploaded images are deleted too. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Keep it</Button>
            </DialogClose>
            <Button variant="destructive" disabled={del.isPending} onClick={() => toDelete && del.mutate(toDelete.id)}>
              {del.isPending ? 'Deleting…' : 'Delete case'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
