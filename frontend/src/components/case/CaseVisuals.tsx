import { useState } from 'react'
import { useKeenSlider, type KeenSliderInstance } from 'keen-slider/react'
import 'keen-slider/keen-slider.min.css'
import GildedFrame from '@/components/game/GildedFrame'
import { cn } from '@/lib/utils'
import type { Scene } from '@/api/cases'

const AUTOPLAY_MS = 4500

/** Looping keen-slider that moves to the next one on a timer; pauses while hovered or dragged. */
function autoplay(slider: KeenSliderInstance) {
  let timeout: ReturnType<typeof setTimeout>
  let paused = false
  const clear = () => clearTimeout(timeout)
  const next = () => {
    clear()
    if (paused) return
    timeout = setTimeout(() => slider.next(), AUTOPLAY_MS)
  }
  slider.on('created', () => {
    slider.container.addEventListener('mouseover', () => ((paused = true), clear()))
    slider.container.addEventListener('mouseout', () => ((paused = false), next()))
    next()
  })
  slider.on('dragStarted', clear)
  slider.on('animationEnded', next)
  slider.on('updated', next)
  slider.on('destroyed', clear)
}

/** Scene images: an auto-advancing looping slider in the gilded frame, with a thumbnail strip to jump to a scene. */
export default function CaseVisuals({ scenes }: { scenes: Scene[] }) {
  if (scenes.length === 0) {
    return (
      <GildedFrame className="m-5 sm:m-8">
        <div className="flex aspect-video items-center justify-center bg-muted text-muted-foreground">No scene photos yet</div>
      </GildedFrame>
    )
  }
  return <SceneSlider scenes={scenes} />
}

function SceneSlider({ scenes }: { scenes: Scene[] }) {
  const [selected, setSelected] = useState(0)
  const many = scenes.length > 1
  const [sliderRef, slider] = useKeenSlider<HTMLDivElement>(
    {
      loop: many,
      drag: many,
      defaultAnimation: { duration: 900 },
      slideChanged: (s) => setSelected(s.track.details.rel),
    },
    many ? [autoplay] : [],
  )

  return (
    <div className="min-w-0 space-y-3">
      <GildedFrame className="mx-5 mt-8 mb-7 min-w-0 sm:mx-8 sm:mt-10 sm:mb-9">
        <div ref={sliderRef} className="keen-slider w-full bg-black">
          {scenes.map((scene) => (
            <figure key={scene.id} className="keen-slider__slide relative">
              <img src={scene.image_url} alt={scene.scene_name} className="aspect-video w-full object-cover" />
              <figcaption className="absolute inset-x-0 bottom-0 bg-background/80 px-3 py-1.5 font-typewriter text-sm">
                {scene.scene_name}
              </figcaption>
            </figure>
          ))}
        </div>
      </GildedFrame>
      {many && (
        <ul className="grid grid-cols-3 gap-3">
          {scenes.map((scene, i) => (
            <li key={scene.id}>
              <button
                type="button"
                onClick={() => slider.current?.moveToIdx(i)}
                aria-label={`Show ${scene.scene_name}`}
                aria-pressed={i === selected}
                className={cn(
                  'block w-full border-2 border-transparent opacity-60 transition hover:opacity-100',
                  i === selected && 'border-primary opacity-100',
                )}
              >
                <img src={scene.image_url} alt="" loading="lazy" className="aspect-video w-full object-cover" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
