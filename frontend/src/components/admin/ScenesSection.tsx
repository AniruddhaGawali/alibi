import { useFieldArray, type Control } from 'react-hook-form'
import { RiDeleteBinLine, RiImageAddLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { FormImagePicker, FormInput } from '@/components/form'
import AdminSection from './AdminSection'
import { emptyScene, type CaseFormValues } from './caseSchema'

interface ScenesSectionProps {
  control: Control<CaseFormValues>
  disabled?: boolean
}

/** Crime-scene pictures shown in the briefing, in order. */
export default function ScenesSection({ control, disabled }: ScenesSectionProps) {
  const { fields, append, remove } = useFieldArray({ control, name: 'scenes', keyName: 'key' })
  // Positions must be unique per case; a new scene goes after the highest one.
  const nextOrder = fields.reduce((m, s) => Math.max(m, s.display_order + 1), 0)

  return (
    <AdminSection
      title="Scene images"
      description="Shown in the case briefing, in this order."
      actions={
        <Button type="button" variant="outline" onClick={() => append(emptyScene(nextOrder))} disabled={disabled || fields.length >= 12}>
          <RiImageAddLine className="size-4" aria-hidden />
          Add scene
        </Button>
      }
    >
      {fields.length === 0 ? (
        <p className="text-sm text-muted-foreground">No scenes yet.</p>
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          {fields.map((f, i) => (
            <div key={f.key} className="space-y-4 border p-4">
              <FormInput control={control} name={`scenes.${i}.scene_name`} label="Scene" placeholder="The study" />
              <FormImagePicker control={control} name={`scenes.${i}.image`} label="Image" disabled={disabled} />
              <Button type="button" variant="ghost" size="sm" onClick={() => remove(i)} disabled={disabled}>
                <RiDeleteBinLine className="size-4" aria-hidden />
                Remove scene
              </Button>
            </div>
          ))}
        </div>
      )}
    </AdminSection>
  )
}
