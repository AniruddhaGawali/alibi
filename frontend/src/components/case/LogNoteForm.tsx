import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import { FormTextarea } from '@/components/form'
import { useAddNoteMutation } from '@/api/cases'
import type { ChatMessage } from '@/api/chat'

const schema = z.object({
  text: z.string().trim().min(1, 'Write something first').max(1000, 'Keep a note under 1000 characters'),
})

/** A small form under a log entry: saves a note tied to that exact entry (and to its suspect). */
export default function LogNoteForm({ caseId, log, onDone }: { caseId: string; log: ChatMessage; onDone: () => void }) {
  const add = useAddNoteMutation({ onError: (e) => toast.error(e.message) })
  const { control, handleSubmit } = useForm({ resolver: zodResolver(schema), defaultValues: { text: '' } })

  const save = handleSubmit(({ text }) =>
    add.mutate({ caseId, body: text, suspectId: log.suspect_id, chatMessageId: log.id }, { onSuccess: onDone }),
  )

  return (
    <form onSubmit={save} className="space-y-2" aria-label={`Add a note to this exchange with ${log.suspect_name}`}>
      <FormTextarea
        control={control}
        name="text"
        rows={2}
        autoFocus
        aria-label="Note on this entry"
        placeholder="What stands out in this answer?"
        className="resize-none font-typewriter"
      />
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={add.isPending}>
          {add.isPending ? 'Saving…' : 'Save note'}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onDone}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
