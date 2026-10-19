import { Link } from 'react-router'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import NotFound from '@/components/game/NotFound'
import { ApiError } from '@/api/apiClient'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetBriefingQuery, useGetResultQuery } from '@/api/cases'
import { useGetStatsQuery } from '@/api/dashboard'
import SubmitDeductionButton from '@/components/accusation/SubmitDeductionButton'
import { useGameStore } from '@/store/useGameStore'
import CaseOutcome from './CaseOutcome'
import CasePlot from './CasePlot'
import CaseVisuals from './CaseVisuals'
import HintWidget from './HintWidget'
import ChatHistorySection from './ChatHistorySection'
import ChatWidget from './ChatWidget'
import SuspectCarousel from './SuspectCarousel'

/** The case dossier page: title, pictures beside the plot, the suspects and the chat button. */
export default function CaseBriefing({ caseId }: { caseId: string }) {
  const { data: briefing, isPending, error } = useGetBriefingQuery(caseId)
  const { data: stats } = useGetStatsQuery()
  // The verdict: the one just settled in this session, else the stored one (a reload,
  // or coming back from the dashboard). The server answers 404 while the case is open.
  const fresh = useGameStore((s) => s.outcomes[caseId])
  const { data: stored, isPending: resultPending } = useGetResultQuery(caseId)
  const outcome = fresh ?? stored
  // Only the case the player is locked into can be questioned or accused. Once the
  // accusation is settled the server has dropped the lock, so the chat stays shut.
  const isActive = !resultPending && !outcome && stats?.active_story_id === caseId
  const chatLocked = !isActive

  if (isPending) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-14 w-2/3" />
        <div className="grid gap-8 lg:grid-cols-2">
          <Skeleton className="aspect-video" />
          <Skeleton className="h-80" />
        </div>
      </div>
    )
  }

  // An unknown or malformed case id gets the same page as an unknown URL.
  if (error instanceof ApiError && (error.status === 404 || error.status === 400)) {
    return <NotFound to="/dashboard" label="Back to the dashboard" />
  }

  if (error) {
    return (
      <Alert variant="destructive" className="mx-auto max-w-xl">
        <AlertTitle>Could not open this case</AlertTitle>
        <AlertDescription className="space-y-3">
          <p>{error.message}</p>
          <Button asChild variant="outline">
            <Link to="/dashboard">Back to the dashboard</Link>
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-10">
      <header className="space-y-2 text-center">
        <p className="font-typewriter text-sm tracking-[0.3em] text-muted-foreground uppercase">Case file</p>
        <h1 className="font-heading text-4xl tracking-wide text-primary sm:text-6xl">{briefing.title}</h1>
        {isActive && <SubmitDeductionButton caseId={caseId} className="mt-4" />}
      </header>

      {outcome && <CaseOutcome outcome={outcome} />}

      <div className="grid gap-8 lg:grid-cols-[1fr_30rem]">
        <CaseVisuals scenes={briefing.scenes} />
        <CasePlot plot={briefing.main_plot} synopsis={briefing.synopsis} />
      </div>

      <SuspectCarousel caseId={caseId} chatLocked={chatLocked} />
      <ChatHistorySection caseId={caseId} />
      {/* one fixed column, so the buttons can never overlap: hint on top, chat below */}
      <div className="fixed right-6 bottom-6 z-40 flex flex-col items-center gap-5">
        {isActive && <HintWidget caseId={caseId} />}
        <ChatWidget caseId={caseId} locked={chatLocked} />
      </div>
    </div>
  )
}
