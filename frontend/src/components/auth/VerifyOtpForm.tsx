import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Link, Navigate, useNavigate } from 'react-router'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormOtp } from '@/components/form'
import { useResendOtpMutation, useVerifyOtpMutation } from '@/api/auth'
import { useStartSession } from '@/hooks/useStartSession'
import { deletePendingOtpEmail, getPendingOtpEmail } from '@/utils/cookieHelpers'
import AuthCard from './AuthCard'

const schema = z.object({
  otp: z.string().regex(/^[0-9]{6}$/, 'Enter the 6-digit code'),
})
type Values = z.infer<typeof schema>

export default function VerifyOtpForm() {
  // Read once: the cookie is deleted on success, and a re-render before the
  // redirect lands must not bounce the user back to /signup.
  const [email] = useState(getPendingOtpEmail)
  const navigate = useNavigate()
  const startSession = useStartSession()
  const { control, handleSubmit, reset } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { otp: '' },
  })
  const verify = useVerifyOtpMutation({
    onSuccess: async (session) => {
      try {
        await startSession(session)
        deletePendingOtpEmail()
        navigate('/onboarding/avatar')
      } catch (e) {
        toast.error(e instanceof Error ? e.message : 'Could not load your profile')
      }
    },
    onError: (e) => {
      reset()
      toast.error(e.message)
    },
  })
  const resend = useResendOtpMutation({
    onSuccess: () => toast.success('A new code is on its way'),
    onError: (e) => toast.error(e.message),
  })

  // Nothing to verify without a pending signup.
  if (!email) return <Navigate to="/signup" replace />

  return (
    <AuthCard
      title="Verify your email"
      description={`Enter the 6-digit code we sent to ${email}.`}
      footer={
        <span>
          Wrong address? <Link to="/signup">Start over</Link>
        </span>
      }
    >
      <form onSubmit={handleSubmit((v) => verify.mutate({ email, otp: v.otp }))} noValidate>
        <FieldGroup>
          <FormOtp control={control} name="otp" label="Verification code" disabled={verify.isPending} />
          <Button type="submit" disabled={verify.isPending}>
            {verify.isPending ? 'Verifying…' : 'Verify'}
          </Button>
          <Button type="button" variant="ghost" disabled={resend.isPending} onClick={() => resend.mutate({ email })}>
            Resend code
          </Button>
        </FieldGroup>
      </form>
    </AuthCard>
  )
}
