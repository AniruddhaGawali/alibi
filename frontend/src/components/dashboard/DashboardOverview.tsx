import { Link } from 'react-router'
import { RiBookOpenLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
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
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
        {user ? <h1 className="text-3xl text-primary">Welcome, {user.username}</h1> : <Skeleton className="h-9 w-64" />}
        <p className="text-muted-foreground">
          <Typewriter text="Pick a case and start interrogating." />
        </p>
        </div>
        <Button asChild variant="outline">
          <Link to="/how-to-play">
            <RiBookOpenLine className="size-4" aria-hidden />
            How to play
          </Link>
        </Button>
      </div>
      {/* The stats sit beside the framed leaderboard, as tall as the frame. */}
      <Leaderboard side={<StatsPanel />} />
      <StoryGrid />
    </div>
  )
}
