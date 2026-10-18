import type { ReactNode } from 'react'
import { RiCoinsLine, RiSearchEyeLine, RiTrophyLine } from '@remixicon/react'
import { Skeleton } from '@/components/ui/skeleton'
import { STATS_POLL_MS, useGetStatsQuery } from '@/api/dashboard'
import { cn } from '@/lib/utils'

/** Live token balance and trophies, refetched every 5 minutes. Compact square boxes. */
export default function StatsPanel({ className }: { className?: string }) {
  const { data: stats } = useGetStatsQuery(undefined, { refetchInterval: STATS_POLL_MS })

  return (
    <div className={cn('flex gap-4 self-start lg:flex-col', className)} aria-live="polite">
      <Stat icon={<RiCoinsLine className="size-5" aria-hidden />} label="Tokens" value={stats?.tokens} />
      <Stat icon={<RiTrophyLine className="size-5" aria-hidden />} label="Trophies" value={stats?.trophies} />
      <Stat icon={<RiSearchEyeLine className="size-5" aria-hidden />} label="Solved" value={stats?.cases_solved} />
    </div>
  )
}

function Stat({ icon, label, value }: { icon: ReactNode; label: string; value?: number }) {
  return (
    <div className="frame flex aspect-square w-24 flex-col items-center justify-center gap-1 border bg-card p-2 text-center lg:w-full">
      <span className="text-gold">{icon}</span>
      <p className="text-xs text-muted-foreground">{label}</p>
      {value === undefined ? (
        <Skeleton className="h-6 w-8" />
      ) : (
        <p className="font-heading text-2xl leading-none text-gold">{value}</p>
      )}
    </div>
  )
}
