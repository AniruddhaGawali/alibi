import { Link } from 'react-router'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetBriefingQuery } from '@/api/cases'
import CasePlot from './CasePlot'
import CaseVisuals from './CaseVisuals'
import ChatHistorySection from './ChatHistorySection'
import ChatWidget from './ChatWidget'
import SuspectCarousel from './SuspectCarousel'

/** The case dossier page: title, pictures beside the plot, the suspects and the chat button. */
export default function CaseBriefing({ caseId }: { caseId: string }) {
  const { data: briefing, isPending, error } = useGetBriefingQuery(caseId)

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
      </header>

      <div className="grid gap-8 lg:grid-cols-[1fr_30rem]">
        <CaseVisuals scenes={briefing.scenes} />
        <CasePlot plot={briefing.main_plot} synopsis={briefing.synopsis} />
      </div>

      <SuspectCarousel caseId={caseId} />
      <ChatHistorySection caseId={caseId} />
      <ChatWidget caseId={caseId} />
    </div>
  )
}
