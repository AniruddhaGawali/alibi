export type RankName = 'Rookie' | 'Detective' | 'Inspector' | 'Chief'

/** Trophies needed for each rank, lowest first. */
export const RANKS: { name: RankName; min: number }[] = [
  { name: 'Rookie', min: 0 },
  { name: 'Detective', min: 10 },
  { name: 'Inspector', min: 30 },
  { name: 'Chief', min: 60 },
]

export interface RankInfo {
  name: RankName
  /** 0-3, the position in RANKS (picks the icon). */
  tier: number
  /** Trophies still needed for the next rank; null at the top. */
  toNext: number | null
  /** 0-1 progress through the current rank; 1 at the top. */
  progress: number
}

export function rankFor(trophies: number): RankInfo {
  const t = Math.max(0, trophies)
  let tier = 0
  RANKS.forEach((r, i) => {
    if (t >= r.min) tier = i
  })
  const next = RANKS[tier + 1]
  if (!next) return { name: RANKS[tier].name, tier, toNext: null, progress: 1 }
  const { min } = RANKS[tier]
  return { name: RANKS[tier].name, tier, toNext: next.min - t, progress: (t - min) / (next.min - min) }
}
