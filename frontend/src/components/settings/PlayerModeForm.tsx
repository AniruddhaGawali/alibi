import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormChoiceCards } from '@/components/form'
import { useSetPlayerModeMutation } from '@/api/user'
import { useGameStore } from '@/store/useGameStore'
import { PLAYER_MODE_OPTIONS } from './playerModes'

const schema = z.object({ mode: z.enum(['investigator', 'master_sleuth'], 'Pick a mode') })
type Values = z.infer<typeof schema>

/** Choose the difficulty mode. The store changes at once and goes back if the server refuses. */
export default function PlayerModeForm({ submitLabel = 'Save', onSaved }: { submitLabel?: string; onSaved?: () => void }) {
  const current = useGameStore((s) => s.playerMode)
  const setPlayerMode = useGameStore((s) => s.setPlayerMode)
  const { control, handleSubmit } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { mode: current } })
  const save = useSetPlayerModeMutation({
    onMutate: (mode) => {
      const previous = useGameStore.getState().playerMode
      setPlayerMode(mode)
      return { previous }
    },
    onSuccess: () => onSaved?.(),
    onError: (e, _mode, ctx) => {
      if (ctx) setPlayerMode((ctx as { previous: Values['mode'] }).previous)
      toast.error(e.message)
    },
  })

  return (
    <form onSubmit={handleSubmit((v) => save.mutate(v.mode))} noValidate>
      <FieldGroup>
        <FormChoiceCards control={control} name="mode" label="Difficulty mode" options={PLAYER_MODE_OPTIONS} />
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? 'Saving…' : submitLabel}
        </Button>
      </FieldGroup>
    </form>
  )
}
