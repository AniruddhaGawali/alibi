import { useNavigate, useSearchParams } from 'react-router'
import AuthCard from '@/components/auth/AuthCard'
import AvatarForm from '@/components/auth/AvatarForm'

/** First onboarding step. A new player (?new=1) goes on to pick their difficulty mode. */
export default function AvatarOnboardingPage() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const isNew = params.get('new') === '1'
  return (
    <AuthCard title="Choose your avatar" description="Upload your own image or pick one of the defaults.">
      <AvatarForm submitLabel="Continue" onSaved={() => navigate(isNew ? '/onboarding/mode' : '/dashboard')} />
    </AuthCard>
  )
}
