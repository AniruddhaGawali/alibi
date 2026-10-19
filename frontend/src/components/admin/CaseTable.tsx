import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { RiDeleteBinLine, RiEditLine } from '@remixicon/react'
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
import { useDeleteAdminCaseMutation, useGetAdminCasesQuery, type AdminCaseSummary } from '@/api/admin'
import { MIN_SUSPECTS } from './caseSchema'

/** Every case, drafts included, with whether it can be published. */
export default function CaseTable() {
  const { data: cases, isPending, error, refetch } = useGetAdminCasesQuery()
  const [toDelete, setToDelete] = useState<AdminCaseSummary>()
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
      <Table>
        <TableHeader>
          <TableRow>
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
          {cases.map((c) => {
            const ready = c.suspect_count >= MIN_SUSPECTS && c.culprit_count === 1
            return (
              <TableRow key={c.id}>
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
