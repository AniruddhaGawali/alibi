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
      {/* Stats sit in the right column beside the leaderboard, below the welcome header. */}
      <div className="grid gap-x-8 gap-y-6 lg:grid-cols-[1fr_7rem]">
        <div className="lg:col-start-1">
          {user ? <h1 className="text-3xl text-primary">Welcome, {user.username}</h1> : <Skeleton className="h-9 w-64" />}
          <p className="text-muted-foreground">
            <Typewriter text="Pick a case and start interrogating." />
          </p>
        </div>
        <StatsPanel className="lg:col-start-2 lg:row-start-2" />
        <div className="lg:col-start-1">
          <Leaderboard />
        </div>
      </div>
      <StoryGrid />
    </div>
  )
}
