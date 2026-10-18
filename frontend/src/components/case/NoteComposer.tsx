import { RiQuillPenLine } from '@remixicon/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import { FormSelect, FormTextarea } from '@/components/form'
import { useAddNoteMutation, useGetSuspectsQuery } from '@/api/cases'
import { useGameStore } from '@/store/useGameStore'

const GENERAL = 'general'

const schema = z.object({
  text: z.string().trim().min(1, 'Write something first').max(1000, 'Keep a note under 1000 characters'),
  about: z.string(),
})
type NoteForm = z.infer<typeof schema>

/**
 * Adds a note to the log. With a suspect selected in the filter the note is about
 * that suspect; under "All Logs" a dropdown picks the suspect or the case in general.
 */
export default function NoteComposer({ caseId }: { caseId: string }) {
  const { data: suspects } = useGetSuspectsQuery(caseId)
  const filter = useGameStore((s) => s.selectedSuspectFilter)
  const add = useAddNoteMutation({ onError: (e) => toast.error(e.message) })

  const { control, handleSubmit, reset, setValue } = useForm<NoteForm>({
    resolver: zodResolver(schema),
    defaultValues: { text: '', about: filter ?? GENERAL },
  })

  // switching the filter changes who a new note is about
  useEffect(() => setValue('about', filter ?? GENERAL), [filter, setValue])

  const suspect = suspects?.find((s) => s.id === filter)

  const save = handleSubmit(({ text, about }) =>
    add.mutate(
      { caseId, body: text, suspectId: filter ?? (about === GENERAL ? null : about) },
      { onSuccess: () => reset({ text: '', about }) },
    ),
  )

  return (
    <form onSubmit={save} className="frame space-y-3 bg-card p-4" aria-label="Add a note">
      <p className="flex items-center gap-2 font-heading text-lg">
        <RiQuillPenLine className="size-5 text-primary" aria-hidden />
        {suspect ? `Add a note about ${suspect.name}` : 'Add a note'}
      </p>
      <FormTextarea
        control={control}
        name="text"
        placeholder={suspect ? `What do you make of ${suspect.name}?` : 'Who lied? What does not add up?'}
        rows={2}
        className="resize-none font-typewriter"
      />
      <div className="flex flex-wrap items-end gap-3">
        {!filter && (
          <div className="w-full sm:w-64">
            <FormSelect
              control={control}
              name="about"
              label="About"
              options={[
                { value: GENERAL, label: 'The case in general' },
                ...(suspects ?? []).map((s) => ({ value: s.id, label: s.name })),
              ]}
            />
          </div>
        )}
        <Button type="submit" disabled={add.isPending}>
          {add.isPending ? 'Saving…' : 'Add note'}
        </Button>
      </div>
    </form>
  )
}
