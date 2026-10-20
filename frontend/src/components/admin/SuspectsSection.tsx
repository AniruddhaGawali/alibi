import { useFieldArray, useFormState, type Control, type UseFormSetValue } from 'react-hook-form'
import { RiUserAddLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { FieldError } from '@/components/ui/field'
import AdminSection from './AdminSection'
import SuspectCard from './SuspectCard'
import SuspectOrder from './SuspectOrder'
import { MIN_SUSPECTS, emptySuspect, type CaseFormValues } from './caseSchema'

interface SuspectsSectionProps {
  control: Control<CaseFormValues>
  setValue: UseFormSetValue<CaseFormValues>
  disabled?: boolean
}

/** The dynamic list of suspects, in the order players see them. Removed suspects are deleted when the case is saved. */
export default function SuspectsSection({ control, setValue, disabled }: SuspectsSectionProps) {
  const { fields, append, remove, move } = useFieldArray({ control, name: 'suspects', keyName: 'key' })
  const { errors } = useFormState({ control, name: 'suspects' })
  const listError = errors.suspects?.root?.message ?? errors.suspects?.message

  return (
    <AdminSection
      title="Suspects"
      description={`A published case needs at least ${MIN_SUSPECTS} suspects and exactly one culprit.`}
      actions={
        <Button type="button" variant="outline" onClick={() => append(emptySuspect())} disabled={disabled || fields.length >= 12}>
          <RiUserAddLine className="size-4" aria-hidden />
          Add suspect
        </Button>
      }
    >
      {listError && <FieldError>{listError}</FieldError>}
      {fields.length === 0 ? (
        <p className="text-sm text-muted-foreground">No suspects yet.</p>
      ) : (
        <div className="space-y-6">
          {fields.length > 1 && <SuspectOrder control={control} onMove={move} disabled={disabled} />}
          {fields.map((f, i) => (
            <SuspectCard
              key={f.key}
              control={control}
              setValue={setValue}
              index={i}
              count={fields.length}
              onRemove={() => remove(i)}
              disabled={disabled}
            />
          ))}
        </div>
      )}
    </AdminSection>
  )
}
