import type { ReactNode } from 'react'
import { RiTrophyLine } from '@remixicon/react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { LEADERBOARD_POLL_MS, useGetLeaderboardQuery } from '@/api/dashboard'
import { useGetUserDataQuery } from '@/api/user'
import { cn } from '@/lib/utils'

/**
 * Top players by trophies as a table, refetched every few seconds. `side` (the stats)
 * is placed to the right of the framed table, top and bottom aligned with the frame.
 */
export default function Leaderboard({ side }: { side?: ReactNode }) {
  const { data: entries, isPending, error } = useGetLeaderboardQuery(undefined, {
    refetchInterval: LEADERBOARD_POLL_MS,
  })
  const { data: me } = useGetUserDataQuery()

  return (
    <section className="space-y-3">
      <h2 className="text-2xl">Leaderboard</h2>
      <div className="grid gap-x-2 lg:grid-cols-[1fr_11.5rem]">
      <div className="min-w-0">
      {isPending ? (
        <div className="space-y-2">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-10" />
          ))}
        </div>
      ) : error && !entries ? (
        <Alert variant="destructive">
          <AlertTitle>Could not load the leaderboard</AlertTitle>
          <AlertDescription>{error.message}</AlertDescription>
        </Alert>
      ) : !entries || entries.length === 0 ? (
        <p className="text-muted-foreground">No ranked detectives yet. Solve a case to take the lead.</p>
      ) : (
        <div className="frame-vintage m-6 p-10">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-16">Rank</TableHead>
                <TableHead>Detective</TableHead>
                <TableHead className="text-right">
                  <span className="inline-flex items-center gap-1">
                    <RiTrophyLine className="size-4 text-gold" aria-hidden />
                    Trophies
                  </span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody aria-live="polite">
              {entries.map((e) => (
                <TableRow key={e.user_id} className={cn(e.user_id === me?.id && 'bg-primary/10 text-primary')}>
                  <TableCell className="font-heading">{e.rank}</TableCell>
                  <TableCell className="max-w-0 truncate">{e.username}</TableCell>
                  <TableCell className="text-right text-gold">{e.trophies}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      </div>
      {side && <div className="my-6 flex">{side}</div>}
      </div>
    </section>
  )
}
