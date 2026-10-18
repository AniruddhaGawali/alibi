import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Field, FieldDescription, FieldError, FieldLegend, FieldSet } from '@/components/ui/field'
import { cn } from '@/lib/utils'

export interface ImageChoice {
  value: number
  src: string
  alt: string
}

interface FormImageChoiceProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  options: ImageChoice[]
  label?: string
  description?: string
  /** Called after the form value changed (e.g. to clear a competing field). */
  onValueChange?: (value: number | null) => void
}

/** Pick one image; the form value is the option's number, or null. Clicking the chosen one again clears it. */
export default function FormImageChoice<T extends FieldValues>({
  control,
  name,
  options,
  label,
  description,
  onValueChange,
}: FormImageChoiceProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FieldSet data-invalid={fieldState.invalid || undefined}>
          {label && <FieldLegend variant="label">{label}</FieldLegend>}
          {description && <FieldDescription>{description}</FieldDescription>}
          <Field orientation="horizontal" className="flex-wrap">
            {options.map((o) => {
              const selected = field.value === o.value
              return (
                <button
                  key={o.value}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => {
                    const next = selected ? null : o.value
                    field.onChange(next)
                    onValueChange?.(next)
                  }}
                  className={cn(
                    'border-2 border-border p-1 transition-colors hover:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                    selected && 'border-primary shadow-[var(--glow-gold)]',
                  )}
                >
                  <img src={o.src} alt={o.alt} width={96} height={96} className="size-24 object-cover" />
                </button>
              )
            })}
          </Field>
          {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
        </FieldSet>
      )}
    />
  )
}
