import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormInput } from '@/components/form'
import { useUpdateAdminConfigMutation, type AdminConfig } from '@/api/admin'
import { showApiError } from '@/utils/formErrors'

/** Every zone this browser knows; the server checks the name again. */
const TIMEZONES: string[] = typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : ['UTC']

const schema = z.object({
  daily_tokens: z
    .string()
    .trim()
    .regex(/^\d{1,2}$/, 'Enter a whole number')
    .refine((v) => Number(v) >= 1 && Number(v) <= 10, 'Between 1 and 10: a player never holds more than 10'),
  token_reset_time: z.string().regex(/^([01]\d|2[0-3]):[0-5]\d$/, 'Pick a time'),
  token_reset_timezone: z.string().trim().min(1, 'Pick a time zone').max(64, 'Pick a time zone'),
})
type Values = z.infer<typeof schema>

const FIELDS = ['daily_tokens', 'token_reset_time', 'token_reset_timezone'] as const

/** Daily question tokens and when they are reset. */
export default function SettingsForm({ config }: { config: AdminConfig }) {
  const { control, handleSubmit, setError, formState, reset } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      daily_tokens: String(config.daily_tokens),
      token_reset_time: config.token_reset_time,
      token_reset_timezone: config.token_reset_timezone,
    },
  })
  const update = useUpdateAdminConfigMutation({
    onSuccess: (saved) => {
      toast.success('Settings saved')
      reset({ daily_tokens: String(saved.daily_tokens), token_reset_time: saved.token_reset_time, token_reset_timezone: saved.token_reset_timezone })
    },
    onError: (e) => showApiError(e, setError, FIELDS),
  })

  return (
    <form
      onSubmit={handleSubmit((v) => update.mutate({ ...v, daily_tokens: Number(v.daily_tokens) }))}
      noValidate
      className="max-w-xl"
    >
      <FieldGroup>
        <FormInput
          control={control}
          name="daily_tokens"
          label="Daily question tokens"
          description="Every balance is set to this at the reset (unused tokens do not roll over), and new accounts start with it. Takes effect at the next reset."
          inputMode="numeric"
        />
        <div className="grid gap-5 sm:grid-cols-2">
          <FormInput control={control} name="token_reset_time" label="Reset time" type="time" step={60} />
          <FormInput
            control={control}
            name="token_reset_timezone"
            label="Time zone"
            list="admin-timezones"
            autoComplete="off"
            placeholder="Asia/Kolkata"
          />
        </div>
        <datalist id="admin-timezones">
          {TIMEZONES.map((tz) => (
            <option key={tz} value={tz} />
          ))}
        </datalist>
        <p className="text-sm text-muted-foreground">
          The reset runs within a minute of this time. Moving it earlier than now on a day that has not been reset yet resets at once.
          {config.updated_at && <> Last changed {new Date(config.updated_at).toLocaleString()}.</>}
        </p>
        <Button type="submit" className="self-start" disabled={update.isPending || !formState.isDirty}>
          {update.isPending ? 'Saving…' : 'Save settings'}
        </Button>
      </FieldGroup>
    </form>
  )
}
