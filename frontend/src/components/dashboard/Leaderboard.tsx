import type { ReactNode } from 'react'
import { RiMedalLine, RiTrophyLine } from '@remixicon/react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { LEADERBOARD_POLL_MS, useGetLeaderboardQuery } from '@/api/dashboard'
import RankBadge from '@/components/game/RankBadge'
import { cn } from '@/lib/utils'

/**
 * Top players by trophies as a table, refetched every few seconds. `side` (the stats)
 * is placed to the right of the framed table, top and bottom aligned with the frame.
 */
export default function Leaderboard({ side, centered = false }: { side?: ReactNode; centered?: boolean }) {
  const { data: entries, isPending, error } = useGetLeaderboardQuery(undefined, {
    refetchInterval: LEADERBOARD_POLL_MS,
  })

  return (
    <section className="space-y-3">
      <h2 className={cn(centered ? 'text-center text-4xl text-primary' : 'text-2xl')}>Leaderboard</h2>
      {/* the side column is only reserved when there is something to put in it */}
      <div className={cn('grid gap-x-2', side && 'lg:grid-cols-[1fr_11.5rem]')}>
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
                <TableHead className="hidden sm:table-cell">Title</TableHead>
                <TableHead className="text-right">
                  <span className="inline-flex items-center gap-1">
                    <RiTrophyLine className="size-4 text-gold" aria-hidden />
                    Trophies
                  </span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody aria-live="polite">
              {entries.map((e, i) => (
                <TableRow
                  key={e.rank}
                  style={{ animationDelay: `${i * 50}ms` }}
                  className={cn('row-in', e.is_me && 'bg-primary/10 text-primary')}
                >
                  <TableCell className={cn('font-heading', e.rank <= 3 && 'text-gold')}>
                    {e.rank <= 3 ? <RiMedalLine className="size-5" aria-label={`Rank ${e.rank}`} /> : e.rank}
                  </TableCell>
                  <TableCell className="max-w-0 truncate">{e.username}</TableCell>
                  <TableCell className="hidden sm:table-cell">
                    <RankBadge trophies={e.trophies} />
                  </TableCell>
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
