import { Link, useParams } from 'react-router'
import { RiArrowLeftLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetAdminCaseQuery } from '@/api/admin'
import CaseBuilderForm from '@/components/admin/CaseBuilderForm'

/** /admin/cases/new creates a case; /admin/cases/:id edits one. */
export default function AdminCaseEditorPage() {
  const { id } = useParams()
  const isNew = !id || id === 'new'
  const { data, isPending, error, refetch, dataUpdatedAt } = useGetAdminCaseQuery(id ?? '', { enabled: !isNew })

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button asChild variant="ghost" size="sm">
          <Link to="/admin/dashboard">
            <RiArrowLeftLine className="size-4" aria-hidden />
            All cases
          </Link>
        </Button>
        <h2 className="text-2xl">{isNew ? 'New case' : (data?.title ?? 'Case')}</h2>
      </div>
      {isNew ? (
        <CaseBuilderForm />
      ) : isPending ? (
        <Skeleton className="h-96 w-full" />
      ) : error ? (
        <div className="flex items-center gap-3 text-sm text-destructive">
          {error.message}
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      ) : (
        // A new key after every save reloads the builder from what the server stored.
        <CaseBuilderForm key={dataUpdatedAt} initial={data} />
      )}
    </div>
  )
}
