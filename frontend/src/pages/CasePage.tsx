import { Navigate, useParams } from 'react-router'
import CaseBriefing from '@/components/case/CaseBriefing'

export default function CasePage() {
  const { id } = useParams()
  if (!id) return <Navigate to="/dashboard" replace />
  return <CaseBriefing caseId={id} />
}
