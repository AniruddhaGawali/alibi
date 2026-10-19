import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetAdminConfigQuery } from '@/api/admin'
import AdminSection from '@/components/admin/AdminSection'
import GrantTokensPanel from '@/components/admin/GrantTokensPanel'
import SettingsForm from '@/components/admin/SettingsForm'

export default function AdminSettingsPage() {
  const { data, isPending, error, refetch, dataUpdatedAt } = useGetAdminConfigQuery()

  return (
    <div className="space-y-8">
      <AdminSection title="Global settings" description="The question-token economy for every player.">
        {isPending ? (
          <Skeleton className="h-56 w-full max-w-xl" />
        ) : error ? (
          <div className="flex items-center gap-3 text-sm text-destructive">
            {error.message}
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Retry
            </Button>
          </div>
        ) : (
          <SettingsForm key={dataUpdatedAt} config={data} />
        )}
      </AdminSection>
      <AdminSection
        title="Give tokens to a player"
        description="For support cases or rewards: adds question tokens to one player right away."
      >
        <GrantTokensPanel />
      </AdminSection>
    </div>
  )
}
