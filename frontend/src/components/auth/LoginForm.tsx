import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { useEffect } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormInput } from '@/components/form'
import { useLoginMutation } from '@/api/auth'
import { useStartSession } from '@/hooks/useStartSession'
import AuthCard from './AuthCard'
import GoogleButton from './GoogleButton'
import OrDivider from './OrDivider'

const schema = z.object({
  email: z.email('Enter a valid email'),
  password: z.string().min(1, 'Enter your password').max(72, 'Password must be at most 72 characters'),
})
type Values = z.infer<typeof schema>

const GOOGLE_ERRORS: Record<string, string> = {
  google_disabled: 'Google sign-in is not available right now.',
  denied: 'Google sign-in was cancelled.',
  invalid_state: 'That Google sign-in link expired. Please try again.',
  rate_limited: 'Too many attempts. Please wait a bit and try again.',
  email_unverified: 'Your Google email is not verified.',
  account_conflict: 'This email is already linked to a different Google account.',
  failed: 'Google sign-in failed. Please try again.',
}

export default function LoginForm() {
  const navigate = useNavigate()
  // the backend sends failed Google sign-ins back here as ?google_error=<code>
  const [params, setParams] = useSearchParams()
  const googleError = params.get('google_error')
  useEffect(() => {
    if (!googleError) return
    toast.error(GOOGLE_ERRORS[googleError] ?? GOOGLE_ERRORS.failed)
    setParams({}, { replace: true })
  }, [googleError, setParams])
  const startSession = useStartSession()
  const { control, handleSubmit } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  })
  const login = useLoginMutation({
    onSuccess: async (session) => {
      try {
        const profile = await startSession(session)
        // Admins land in the admin panel; everyone else in the game.
        navigate(profile.role === 'admin' ? '/admin/dashboard' : '/dashboard')
      } catch (e) {
        toast.error(e instanceof Error ? e.message : 'Could not load your profile')
      }
    },
    onError: (e) => toast.error(e.message),
  })

  return (
    <AuthCard
      title="Welcome back"
      description="Log in to continue your case."
      footer={
        <span>
          New here? <Link to="/signup">Create an account</Link>
        </span>
      }
    >
      <form onSubmit={handleSubmit((v) => login.mutate(v))} noValidate>
        <FieldGroup>
          <FormInput control={control} name="email" label="Email" type="email" autoComplete="email" />
          <FormInput control={control} name="password" label="Password" type="password" autoComplete="current-password" />
          <Button type="submit" disabled={login.isPending}>
            {login.isPending ? 'Logging in…' : 'Log in'}
          </Button>
          <OrDivider />
          <GoogleButton />
        </FieldGroup>
      </form>
    </AuthCard>
  )
}
