import { rankFor } from '@/utils/rank'
import RankBadge from './RankBadge'

/** Rank title with a bar towards the next rank. */
export default function RankProgress({ trophies }: { trophies: number }) {
  const rank = rankFor(trophies)
  return (
    <div className="w-full space-y-1">
      <RankBadge trophies={trophies} />
      <div className="h-1.5 w-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(rank.progress * 100)}>
        <div className="h-full bg-gold transition-[width] duration-700" style={{ width: `${rank.progress * 100}%` }} />
      </div>
      <p className="text-[0.7rem] text-muted-foreground">
        {rank.toNext === null ? 'Highest rank' : `${rank.toNext} to next rank`}
      </p>
    </div>
  )
}
