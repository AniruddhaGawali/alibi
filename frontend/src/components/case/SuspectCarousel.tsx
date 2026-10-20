import { useState } from 'react'
import { useKeenSlider, type KeenSliderInstance } from 'keen-slider/react'
import 'keen-slider/keen-slider.min.css'
import { RiArrowLeftSLine, RiArrowRightSLine } from '@remixicon/react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetSuspectsQuery, type Suspect } from '@/api/cases'
import { cn } from '@/lib/utils'
import SuspectCard from './SuspectCard'

/** Draggable keen-slider row of suspect cards, with arrow buttons and position dots. */
export default function SuspectCarousel({ caseId, chatLocked = false }: { caseId: string; chatLocked?: boolean }) {
  const { data: suspects, isPending, error } = useGetSuspectsQuery(caseId)

  return (
    <section className="space-y-4" aria-label="Suspects">
      {isPending ? (
        <>
          <h2 className="text-2xl">The Suspects</h2>
          <div className="flex gap-4">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-64 w-72 shrink-0" />
            ))}
          </div>
        </>
      ) : error ? (
        <>
          <h2 className="text-2xl">The Suspects</h2>
          <Alert variant="destructive">
            <AlertTitle>Could not load the suspects</AlertTitle>
            <AlertDescription>{error.message}</AlertDescription>
          </Alert>
        </>
      ) : (
        <SuspectSlider suspects={suspects} chatLocked={chatLocked} />
      )}
    </section>
  )
}

/** Mounted only once the suspects are loaded, so keen-slider measures real slides. */
function SuspectSlider({ suspects, chatLocked }: { suspects: Suspect[]; chatLocked: boolean }) {
  const [current, setCurrent] = useState(0)
  const [stops, setStops] = useState(suspects.length) // how many positions the slider can rest on
  const syncStops = (s: KeenSliderInstance) => {
    setStops(s.track.details.maxIdx + 1)
    setCurrent(s.track.details.rel)
  }
  const [sliderRef, slider] = useKeenSlider<HTMLDivElement>({
    slides: { perView: 1.15 },
    breakpoints: {
      '(min-width: 640px)': { slides: { perView: 2 } },
      '(min-width: 1024px)': { slides: { perView: 3 } },
      '(min-width: 1280px)': { slides: { perView: 4 } },
    },
    // perView changes with the breakpoint, and so does how far the slider can go
    created: syncStops,
    updated: syncStops,
    optionsChanged: syncStops,
    slideChanged: (s) => setCurrent(s.track.details.rel),
  })

  return (
    <>
      <div className="flex items-center justify-between">
        <h2 className="text-2xl">The Suspects</h2>
        {stops > 1 && (
          <div className="flex gap-2">
            <Button variant="outline" size="icon" onClick={() => slider.current?.prev()} aria-label="Previous suspects">
              <RiArrowLeftSLine aria-hidden />
            </Button>
            <Button variant="outline" size="icon" onClick={() => slider.current?.next()} aria-label="Next suspects">
              <RiArrowRightSLine aria-hidden />
            </Button>
          </div>
        )}
      </div>
      <div ref={sliderRef} className="keen-slider">
        {suspects.map((s) => (
          <div key={s.id} className="keen-slider__slide p-3">
            <SuspectCard suspect={s} chatLocked={chatLocked} />
          </div>
        ))}
      </div>
      {stops > 1 && (
        <div className="flex justify-center gap-2" role="group" aria-label="Suspect position">
          {Array.from({ length: stops }, (_, i) => (
            <button
              key={i}
              type="button"
              aria-label={`Go to position ${i + 1}`}
              aria-current={i === current}
              onClick={() => slider.current?.moveToIdx(i)}
              className={cn('h-1.5 w-6 bg-border transition', i === current && 'bg-primary shadow-[var(--glow-gold)]')}
            />
          ))}
        </div>
      )}
    </>
  )
}
