import { Link } from 'react-router'
import { RiAddLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import AdminSection from '@/components/admin/AdminSection'
import CaseTable from '@/components/admin/CaseTable'

export default function AdminDashboardPage() {
  return (
    <AdminSection
      title="Cases"
      description="Drafts are only visible here. Publish a case once it has its suspects and one culprit."
      actions={
        <Button asChild>
          <Link to="/admin/cases/new">
            <RiAddLine className="size-4" aria-hidden />
            New case
          </Link>
        </Button>
      }
    >
      <CaseTable />
    </AdminSection>
  )
}
