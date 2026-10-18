import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Link, useNavigate } from 'react-router'
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

export default function LoginForm() {
  const navigate = useNavigate()
  const startSession = useStartSession()
  const { control, handleSubmit } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', password: '' },
  })
  const login = useLoginMutation({
    onSuccess: async (session) => {
      try {
        await startSession(session)
        navigate('/dashboard')
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
