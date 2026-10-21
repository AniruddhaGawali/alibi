import { useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormInput } from '@/components/form'
import { useSetUsernameMutation } from '@/api/user'
import { useCurrentUser } from '@/hooks/useCurrentUser'
import { showApiError } from '@/utils/formErrors'

const schema = z.object({
  username: z.string().trim().regex(/^[A-Za-z0-9_]{3,20}$/, 'Use 3-20 letters, digits or underscores'),
})
type Values = z.infer<typeof schema>

interface UsernameFormProps {
  submitLabel?: string
  /** Onboarding lets the player keep the suggested name; settings needs a new one. */
  allowUnchanged?: boolean
  onSaved?: () => void
}

/**
 * Pick a username. Availability is checked only on submit: the server answers 409
 * when the name is taken, and that message appears under the field.
 */
export default function UsernameForm({ submitLabel = 'Save', allowUnchanged = false, onSaved }: UsernameFormProps) {
  const { data: user } = useCurrentUser()
  const current = user?.username ?? ''
  const { control, handleSubmit, setError } = useForm<Values>({
    resolver: zodResolver(schema),
    values: { username: current }, // fills in once the profile has loaded
    resetOptions: { keepDirtyValues: true },
  })
  const unchanged = useWatch({ control, name: 'username' }).trim() === current

  const save = useSetUsernameMutation({
    onSuccess: () => onSaved?.(),
    onError: (e) => showApiError(e, setError, ['username']),
  })

  return (
    <form
      onSubmit={handleSubmit((v) => (v.username === current ? onSaved?.() : save.mutate(v.username)))}
      noValidate
    >
      <FieldGroup>
        <FormInput
          control={control}
          name="username"
          label="Username"
          autoComplete="username"
          maxLength={20}
          description="3-20 letters, digits or underscores. Shown on the leaderboard."
        />
        <Button type="submit" disabled={save.isPending || (unchanged && !allowUnchanged) || !user}>
          {save.isPending ? 'Saving…' : submitLabel}
        </Button>
      </FieldGroup>
    </form>
  )
}
