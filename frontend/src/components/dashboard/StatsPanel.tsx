import type { ReactNode } from 'react'
import { RiCoinsLine, RiSearchEyeLine, RiTrophyLine } from '@remixicon/react'
import { Skeleton } from '@/components/ui/skeleton'
import { STATS_POLL_MS, useGetStatsQuery } from '@/api/dashboard'
import RankProgress from '@/components/game/RankProgress'
import { useCountUp } from '@/hooks/useCountUp'
import { cn } from '@/lib/utils'
import { useGameStore } from '@/store/useGameStore'

/** Live token balance and trophies, refetched every 5 minutes. Compact square boxes. */
export default function StatsPanel({ className }: { className?: string }) {
  const { data: stats, isError } = useGetStatsQuery(undefined, { refetchInterval: STATS_POLL_MS })
  const tokens = useGameStore((s) => s.tokenBalance)
  const trophies = useGameStore((s) => s.trophies)

  return (
    <div className={cn('flex w-full gap-3 sm:gap-4 lg:flex-col', className)} aria-live="polite">
      <Stat icon={<RiCoinsLine className="size-9" aria-hidden />} label="Tokens" value={tokens ?? stats?.tokens} failed={isError} />
      <Stat icon={<RiTrophyLine className="size-9" aria-hidden />} label="Trophies" value={trophies ?? stats?.trophies} failed={isError} footer={(trophies ?? stats?.trophies) !== undefined ? <RankProgress trophies={(trophies ?? stats?.trophies)!} /> : undefined} />
      <Stat icon={<RiSearchEyeLine className="size-9" aria-hidden />} label="Solved" value={stats?.cases_solved} failed={isError} />
    </div>
  )
}

function Stat({ icon, label, value, failed, footer }: { icon: ReactNode; label: string; value?: number; failed?: boolean; footer?: ReactNode }) {
  const shown = useCountUp(value)
  return (
    <div className="frame flex min-w-0 flex-1 basis-0 flex-col items-center justify-center gap-2 border bg-card p-3 text-center max-sm:min-h-44 lg:w-full">
      <span className="text-gold">{icon}</span>
      <p className="text-base tracking-wide text-muted-foreground">{label}</p>
      {value === undefined && failed ? (
        <p className="font-heading text-5xl leading-none text-muted-foreground" title="Could not load">
          –
        </p>
      ) : value === undefined ? (
        <Skeleton className="h-10 w-14" />
      ) : (
        <p className="font-heading text-5xl leading-none text-gold">{shown}</p>
      )}
      {footer}
    </div>
  )
}
