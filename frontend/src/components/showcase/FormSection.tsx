import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { FormCheckbox, FormInput, FormRadioGroup, FormSelect, FormSwitch, FormTextarea } from '@/components/form'
import Section from './Section'

const schema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters'),
  difficulty: z.string().min(1, 'Pick a difficulty'),
  role: z.string().min(1, 'Choose a role'),
  notes: z.string().max(80, 'Keep notes under 80 characters'),
  hints: z.boolean(),
  agree: z.boolean().refine((v) => v, 'You must accept the rules'),
})

type FormValues = z.infer<typeof schema>

export default function FormSection() {
  const { control, handleSubmit } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', difficulty: '', role: '', notes: '', hints: true, agree: false },
  })

  return (
    <Section title="Form (zod validation — submit empty to see errors)">
      <form className="w-full max-w-md" onSubmit={handleSubmit((v) => toast.success(`Welcome, ${v.name}`))}>
        <FieldGroup>
          <FormInput control={control} name="name" label="Detective name" placeholder="Name" />
          <FormSelect
            control={control}
            name="difficulty"
            label="Difficulty"
            placeholder="Select"
            options={[
              { value: 'easy', label: 'Easy' },
              { value: 'hard', label: 'Hard' },
            ]}
          />
          <FormRadioGroup
            control={control}
            name="role"
            label="Role"
            options={[
              { value: 'inspector', label: 'Inspector' },
              { value: 'journalist', label: 'Journalist' },
            ]}
          />
          <FormTextarea control={control} name="notes" label="Notes" description="Optional, max 80 characters" />
          <FormSwitch control={control} name="hints" label="Enable hints" />
          <FormCheckbox control={control} name="agree" label="I accept the rules" />
          <Button type="submit">Start</Button>
        </FieldGroup>
      </form>
    </Section>
  )
}
