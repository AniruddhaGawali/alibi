import { RiSearchEyeLine, RiSpyLine } from '@remixicon/react'
import type { ChoiceCard } from '@/components/form'
import type { PlayerMode } from '@/api/user'

export const PLAYER_MODE_LABELS: Record<PlayerMode, string> = {
  investigator: 'Investigator',
  master_sleuth: 'Master Sleuth',
}

export const PLAYER_MODE_OPTIONS: (ChoiceCard & { value: PlayerMode })[] = [
  {
    value: 'investigator',
    label: PLAYER_MODE_LABELS.investigator,
    icon: RiSearchEyeLine,
    description: 'See what the town already knows about each suspect, and get tips while you interrogate.',
  },
  {
    value: 'master_sleuth',
    label: PLAYER_MODE_LABELS.master_sleuth,
    icon: RiSpyLine,
    description: 'Go in blind: only the first statements, no backgrounds, no tips.',
  },
]
