import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate } from 'react-router'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormInput } from '@/components/form'
import { useSignupMutation } from '@/api/auth'
import { savePendingOtpEmail } from '@/utils/cookieHelpers'
import { showApiError } from '@/utils/formErrors'
import AuthCard from './AuthCard'
import GoogleButton from './GoogleButton'
import OrDivider from './OrDivider'

const schema = z.object({
  email: z.email('Enter a valid email').max(254, 'Email is too long'),
  username: z.string().regex(/^[A-Za-z0-9_]{3,20}$/, 'Use 3-20 letters, digits or underscores'),
  password: z
    .string()
    .min(8, 'Password must be at least 8 characters')
    .max(72, 'Password must be at most 72 characters')
    .regex(/[A-Za-z]/, 'Password needs at least one letter')
    .regex(/[0-9]/, 'Password needs at least one digit'),
})
type Values = z.infer<typeof schema>

export default function SignupForm() {
  const navigate = useNavigate()
  const { control, handleSubmit, setError } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', username: '', password: '' },
  })
  const signup = useSignupMutation({
    onSuccess: (data) => {
      savePendingOtpEmail(data.email)
      navigate('/verify-otp')
    },
    onError: (e) => showApiError(e, setError, ['email', 'username', 'password']),
  })

  return (
    <AuthCard
      title="Create your account"
      description="Join the investigation. We will email you a verification code."
      footer={
        <span>
          Already a detective? <Link to="/login">Log in</Link>
        </span>
      }
    >
      <form onSubmit={handleSubmit((v) => signup.mutate(v))} noValidate>
        <FieldGroup>
          <FormInput control={control} name="email" label="Email" type="email" autoComplete="email" />
          <FormInput control={control} name="username" label="Username" autoComplete="username" />
          <FormInput control={control} name="password" label="Password" type="password" autoComplete="new-password" />
          <Button type="submit" disabled={signup.isPending}>
            {signup.isPending ? 'Sending code…' : 'Sign up'}
          </Button>
          <OrDivider />
          <GoogleButton />
        </FieldGroup>
      </form>
    </AuthCard>
  )
}
