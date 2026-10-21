import { useNavigate, useSearchParams } from 'react-router'
import AuthCard from '@/components/auth/AuthCard'
import UsernameForm from '@/components/settings/UsernameForm'

/**
 * Google sign-up starts here: the account got a name made from the Google profile,
 * which the player can keep or change. A new player (?new=1) goes on to the avatar.
 */
export default function UsernameOnboardingPage() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const isNew = params.get('new') === '1'
  return (
    <AuthCard title="Choose your name" description="This is how other detectives see you. You can change it later in Settings.">
      <UsernameForm
        submitLabel="Continue"
        allowUnchanged
        onSaved={() => navigate(isNew ? '/onboarding/avatar?new=1' : '/dashboard')}
      />
    </AuthCard>
  )
}
