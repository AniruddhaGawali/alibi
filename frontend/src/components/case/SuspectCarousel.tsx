import { useRef } from 'react'
import { RiArrowLeftSLine, RiArrowRightSLine } from '@remixicon/react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetSuspectsQuery } from '@/api/cases'
import SuspectCard from './SuspectCard'

/** Horizontal, swipeable row of suspect cards with arrow buttons. */
export default function SuspectCarousel({ caseId }: { caseId: string }) {
  const { data: suspects, isPending, error } = useGetSuspectsQuery(caseId)
  const track = useRef<HTMLDivElement>(null)

  const scroll = (dir: 1 | -1) => {
    const el = track.current
    if (el) el.scrollBy({ left: dir * el.clientWidth * 0.8, behavior: 'smooth' })
  }

  return (
    <section className="space-y-4" aria-label="Suspects">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl">The Suspects</h2>
        {suspects && suspects.length > 1 && (
          <div className="flex gap-2">
            <Button variant="outline" size="icon" onClick={() => scroll(-1)} aria-label="Previous suspects">
              <RiArrowLeftSLine aria-hidden />
            </Button>
            <Button variant="outline" size="icon" onClick={() => scroll(1)} aria-label="Next suspects">
              <RiArrowRightSLine aria-hidden />
            </Button>
          </div>
        )}
      </div>
      {isPending ? (
        <div className="flex gap-4">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-64 w-72 shrink-0" />
          ))}
        </div>
      ) : error ? (
        <Alert variant="destructive">
          <AlertTitle>Could not load the suspects</AlertTitle>
          <AlertDescription>{error.message}</AlertDescription>
        </Alert>
      ) : (
        <div ref={track} className="flex snap-x snap-mandatory gap-5 overflow-x-auto scroll-smooth p-3 pb-5">
          {suspects.map((s) => (
            <SuspectCard key={s.id} suspect={s} />
          ))}
        </div>
      )}
    </section>
  )
}
