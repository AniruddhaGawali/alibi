import { useWatch, type Control, type UseFormSetValue } from 'react-hook-form'
import { RiDeleteBinLine, RiSpyLine } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { Separator } from '@/components/ui/separator'
import { FormImagePicker, FormInput, FormSwitch, FormTextarea } from '@/components/form'
import type { CaseFormValues } from './caseSchema'

interface SuspectCardProps {
  control: Control<CaseFormValues>
  setValue: UseFormSetValue<CaseFormValues>
  index: number
  /** How many suspects the case has, to switch the others off when this one becomes the culprit. */
  count: number
  onRemove: () => void
  disabled?: boolean
}

/** One suspect of the case builder: what players see, and the AI persona behind it. */
export default function SuspectCard({ control, setValue, index, count, onRemove, disabled }: SuspectCardProps) {
  const name = useWatch({ control, name: `suspects.${index}.name` })
  const culprit = useWatch({ control, name: `suspects.${index}.is_culprit` })
  const p = `suspects.${index}` as const

  // A case has one culprit: marking this one clears the flag on the others.
  const onCulprit = (checked: boolean) => {
    if (!checked) return
    for (let i = 0; i < count; i++) {
      if (i !== index) setValue(`suspects.${i}.is_culprit`, false, { shouldDirty: true })
    }
  }

  return (
    <article className={`space-y-5 border p-5 ${culprit ? 'border-destructive/60' : 'border-border'}`}>
      <header className="flex items-center justify-between gap-3">
        <h3 className="flex items-center gap-2 text-lg">
          <span className="text-muted-foreground">#{index + 1}</span>
          {name || 'New suspect'}
          {culprit && (
            <Badge variant="destructive">
              <RiSpyLine className="size-3.5" aria-hidden />
              Culprit
            </Badge>
          )}
        </h3>
        <Button type="button" variant="ghost" size="sm" onClick={onRemove} disabled={disabled}>
          <RiDeleteBinLine className="size-4" aria-hidden />
          Remove
        </Button>
      </header>

      <FieldGroup>
        <div className="grid gap-5 md:grid-cols-[auto_1fr]">
          <FormImagePicker control={control} name={`${p}.avatar`} label="Avatar" aspect="portrait" disabled={disabled} />
          <div className="grid gap-5 sm:grid-cols-2">
            <FormInput control={control} name={`${p}.name`} label="Name" placeholder="Lady Ashworth" />
            <FormInput control={control} name={`${p}.role`} label="Role" placeholder="the victim's widow" />
            <div className="sm:col-span-2">
              <FormInput
                control={control}
                name={`${p}.relationship_to_victim`}
                label="Relation to the victim"
                placeholder="his wife of twenty years"
              />
            </div>
          </div>
        </div>
        <FormTextarea
          control={control}
          name={`${p}.public_knowledge`}
          label="Public Knowledge / Suspect Background (shown only in Investigator mode)"
          description="What the town already knows about them. Every player's browser receives it, so never put a secret here."
          rows={3}
        />
        <FormTextarea
          control={control}
          name={`${p}.initial_testimony`}
          label="Initial testimony"
          description="Shown to the player in the briefing."
          rows={3}
        />
        <FormSwitch
          control={control}
          name={`${p}.is_culprit`}
          label="Is the culprit"
          description="Only one suspect per case. Never shown to players before the case is closed."
          onCheckedChange={onCulprit}
        />

        <Separator />
        <div>
          <h4 className="font-heading text-base text-gold">AI persona</h4>
          <p className="text-sm text-muted-foreground">
            Assembled into this suspect&apos;s system prompt. Other suspects only ever learn the name, role, relation and personality.
          </p>
        </div>
        <FormTextarea
          control={control}
          name={`${p}.persona`}
          label="Personality and manner of speaking"
          placeholder="Cold, precise, answers questions with questions."
          rows={2}
        />
        <FormTextarea
          control={control}
          name={`${p}.public_alibi`}
          label="Public alibi"
          description={culprit ? 'The lie they tell.' : 'The story they tell about where they were.'}
          rows={3}
        />
        <FormTextarea
          control={control}
          name={`${p}.hidden_truth`}
          label="Hidden truth"
          description={
            culprit
              ? 'What really happened. Required for the culprit; shown to players only after the case is closed.'
              : 'Optional: a secret this suspect keeps (an affair, a debt) that is not the murder.'
          }
          rows={4}
        />
        <FormTextarea
          control={control}
          name={`${p}.confession_triggers`}
          label="Confession triggers"
          description="What makes them crack, e.g. being shown the pharmacy receipt."
          rows={2}
        />
        <FormInput
          control={control}
          name={`${p}.confession_keywords`}
          label="Confession keywords"
          description="Comma-separated. The hidden truth and triggers reach the AI only once a question contains one: word starts, + for words that must appear together (e.g. poison, last+study)."
          placeholder="poison, last+study, receipt"
        />
      </FieldGroup>
    </article>
  )
}
