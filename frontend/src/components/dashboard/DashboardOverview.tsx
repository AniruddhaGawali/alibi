import { Skeleton } from '@/components/ui/skeleton'
import { useGetUserDataQuery } from '@/api/user'
import Typewriter from '@/components/game/Typewriter'
import Leaderboard from './Leaderboard'
import StatsPanel from './StatsPanel'
import StoryGrid from './StoryGrid'

export default function DashboardOverview() {
  const { data: user } = useGetUserDataQuery()

  return (
    <div className="space-y-8">
      <div>
        {user ? <h1 className="text-3xl text-primary">Welcome, {user.username}</h1> : <Skeleton className="h-9 w-64" />}
        <p className="text-muted-foreground">
          <Typewriter text="Pick a case and start interrogating." />
        </p>
      </div>
      {/* The stats sit beside the framed leaderboard, as tall as the frame. */}
      <Leaderboard side={<StatsPanel />} />
      <StoryGrid />
    </div>
  )
}
