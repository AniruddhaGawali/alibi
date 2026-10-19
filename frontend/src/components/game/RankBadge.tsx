import { RiMedalLine, RiSearchEyeLine, RiShieldUserLine, RiVipCrownLine } from '@remixicon/react'
import { cn } from '@/lib/utils'
import { rankFor } from '@/utils/rank'

const ICONS = [RiShieldUserLine, RiSearchEyeLine, RiMedalLine, RiVipCrownLine]
const TONES = ['text-muted-foreground', 'text-gold', 'text-ember', 'text-primary']

/** Rank title for a trophy count: an icon alone (`compact`) or icon plus name. */
export default function RankBadge({
  trophies,
  compact = false,
  className,
}: {
  trophies: number
  compact?: boolean
  className?: string
}) {
  const rank = rankFor(trophies)
  const Icon = ICONS[rank.tier]
  return (
    <span
      className={cn('inline-flex items-center gap-1 font-heading text-xs tracking-wide', TONES[rank.tier], className)}
      title={`${rank.name}${rank.toNext !== null ? `: ${rank.toNext} more trophies for the next rank` : ''}`}
    >
      <Icon className="size-4" aria-hidden />
      {compact ? <span className="sr-only">{rank.name}</span> : rank.name}
    </span>
  )
}
