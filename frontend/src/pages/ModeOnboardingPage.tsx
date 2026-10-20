import { useNavigate } from 'react-router'
import AuthCard from '@/components/auth/AuthCard'
import PlayerModeForm from '@/components/settings/PlayerModeForm'

/** Second onboarding step: the difficulty mode. It can be changed later from the settings menu. */
export default function ModeOnboardingPage() {
  const navigate = useNavigate()
  return (
    <AuthCard
      title="Select your badge"
      description="How much help do you want on your cases? You can change this any time in Settings."
    >
      <PlayerModeForm submitLabel="Start investigating" onSaved={() => navigate('/dashboard')} />
    </AuthCard>
  )
}
