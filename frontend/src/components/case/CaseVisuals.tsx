import { useState } from 'react'
import GildedFrame from '@/components/game/GildedFrame'
import { cn } from '@/lib/utils'
import type { Scene } from '@/api/cases'

/** Scene images: one large picture with a strip of thumbnails to switch between them. */
export default function CaseVisuals({ scenes }: { scenes: Scene[] }) {
  const [selected, setSelected] = useState(0)
  const current = scenes[selected] ?? scenes[0]

  if (!current) {
    return (
      <GildedFrame className="m-8">
        <div className="flex aspect-video items-center justify-center bg-muted text-muted-foreground">No scene photos yet</div>
      </GildedFrame>
    )
  }

  return (
    <div className="space-y-3">
      <GildedFrame className="mx-8 mt-10 mb-9">
        <figure className="relative bg-black">
          <img src={current.image_url} alt={current.scene_name} className="aspect-video w-full object-cover" />
          <figcaption className="absolute inset-x-0 bottom-0 bg-background/80 px-3 py-1.5 font-typewriter text-sm">
            {current.scene_name}
          </figcaption>
        </figure>
      </GildedFrame>
      {scenes.length > 1 && (
        <ul className="grid grid-cols-3 gap-3">
          {scenes.map((scene, i) => (
            <li key={scene.id}>
              <button
                type="button"
                onClick={() => setSelected(i)}
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
