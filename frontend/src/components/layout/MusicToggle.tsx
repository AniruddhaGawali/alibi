import { RiMusic2Fill, RiMusic2Line, RiVolumeDownLine, RiVolumeUpLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'
import { useSoundStore } from '@/store/useSoundStore'

/** Navbar music button: opens a panel with the on/off switch and the volume slider. */
export default function MusicToggle() {
  const muted = useSoundStore((s) => s.musicMuted)
  const toggle = useSoundStore((s) => s.toggleMusicMuted)
  const volume = useSoundStore((s) => s.musicVolume)
  const setVolume = useSoundStore((s) => s.setMusicVolume)
  const silent = muted || volume === 0

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Music volume" title={silent ? 'Music off' : `Music ${Math.round(volume * 100)}%`}>
          {silent ? <RiMusic2Line className="opacity-50" aria-hidden /> : <RiMusic2Fill className="text-gold" aria-hidden />}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-64 space-y-4">
        <div className="flex items-center justify-between gap-3">
          <p className="font-heading text-base">Music</p>
          <Button variant="outline" size="sm" onClick={toggle} aria-pressed={muted}>
            {muted ? 'Turn on' : 'Turn off'}
          </Button>
        </div>
        <div className="flex items-center gap-3">
          <RiVolumeDownLine className="size-5 shrink-0 text-muted-foreground" aria-hidden />
          <Slider
            aria-label="Music volume"
            min={0}
            max={100}
            step={1}
            value={[Math.round(volume * 100)]}
            onValueChange={([v]) => setVolume(v / 100)}
            disabled={muted}
          />
          <RiVolumeUpLine className="size-5 shrink-0 text-muted-foreground" aria-hidden />
        </div>
        <p className="text-right text-xs text-muted-foreground">{muted ? 'Off' : `${Math.round(volume * 100)}%`}</p>
      </PopoverContent>
    </Popover>
  )
}
