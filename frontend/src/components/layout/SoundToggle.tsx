import { RiVolumeMuteLine, RiVolumeUpLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { useSoundStore } from '@/store/useSoundStore'

export default function SoundToggle() {
  const muted = useSoundStore((s) => s.muted)
  const toggle = useSoundStore((s) => s.toggleMuted)
  return (
    <Button variant="ghost" size="icon" onClick={toggle} aria-label={muted ? 'Turn sound on' : 'Mute sound'} aria-pressed={muted}>
      {muted ? <RiVolumeMuteLine aria-hidden /> : <RiVolumeUpLine aria-hidden />}
    </Button>
  )
}
