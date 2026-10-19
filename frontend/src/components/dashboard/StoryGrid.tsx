import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { play } from '@/utils/sound'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { useStartCaseMutation } from '@/api/cases'
import { useGetStatsQuery, useGetStoriesQuery } from '@/api/dashboard'
import RedString from '@/components/game/RedString'
import StoryCard from './StoryCard'

export default function StoryGrid() {
  const { data: stories = [], isPending, error } = useGetStoriesQuery()
  // The stats already say which story is in progress (the stats panel shares this query).
  const { data: stats } = useGetStatsQuery()
  const activeId = stats?.active_story_id ?? null
  const outcomes = stats?.case_outcomes ?? {}
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
              outcome={outcomes[story.id] ?? null}
              busy={startCase.isPending}
              onOpen={(s) => {
                play('paper')
                // a closed case is only read, never started again
                if (outcomes[s.id]) navigate(`/case/${s.id}`)
                else startCase.mutate(s.id)
              }}
              inProgress={story.id === activeId}
              locked={activeId !== null && story.id !== activeId && !outcomes[story.id]}
            />
          ))}
          </div>
        </div>
      )}
    </section>
  )
}
