import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { play } from '@/utils/sound'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { useStartCaseMutation } from '@/api/cases'
import { useGetActiveCaseQuery, useGetStoriesQuery } from '@/api/dashboard'
import RedString from '@/components/game/RedString'
import StoryCard from './StoryCard'

export default function StoryGrid() {
  const { data: fetched, isPending, error } = useGetStoriesQuery()
  const stories = withDemoCopies(fetched)
  const { data: active } = useGetActiveCaseQuery()
  const activeTitle = active?.story_title ?? null
  const navigate = useNavigate()
  // Starting a case you are already in is fine, so Start and Continue share this.
  const startCase = useStartCaseMutation({
    onSuccess: (lock) => navigate(`/case/${lock.story_id}`),
    onError: (e) => toast.error(e.message),
  })

  return (
    <section className="space-y-3">
      <h2 className="text-2xl">Cases</h2>
      {isPending ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-80" />
          ))}
        </div>
      ) : error ? (
        <Alert variant="destructive">
          <AlertTitle>Could not load cases</AlertTitle>
          <AlertDescription>{error.message}</AlertDescription>
        </Alert>
      ) : stories.length === 0 ? (
        <p className="text-muted-foreground">No cases yet. Check back soon.</p>
      ) : (
        <div className="relative pt-6">
          <RedString />
          <div className="relative grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
          {stories.map((story) => (
            <StoryCard
              key={story.id}
              story={story}
              busy={startCase.isPending}
              onOpen={(s) => {
                play('paper')
                startCase.mutate(s.id.replace(/-demo-\d+$/, ''))
              }}
              inProgress={story.title === activeTitle}
            />
          ))}
          </div>
        </div>
      )}
    </section>
  )
}

/**
 * Dev-only layout check: open /dashboard?demo=10 to repeat the cases up to 10 cards
 * (copies get fresh ids). Never active in a production build.
 */
function withDemoCopies<T extends { id: string }>(stories: T[] | undefined): T[] {
  if (!stories) return []
  if (!import.meta.env.DEV || stories.length === 0) return stories
  const n = Number(new URLSearchParams(window.location.search).get('demo'))
  if (!Number.isInteger(n) || n <= stories.length) return stories
  return Array.from({ length: Math.min(n, 30) }, (_, i) => {
    const base = stories[i % stories.length]
    return i < stories.length ? base : { ...base, id: `${base.id}-demo-${i}` }
  })
}
