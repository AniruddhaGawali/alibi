import type { RemixiconComponentType } from '@remixicon/react'
import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { FieldDescription, FieldError, FieldLegend, FieldSet } from '@/components/ui/field'
import { cn } from '@/lib/utils'

export interface ChoiceCard {
  value: string
  label: string
  description: string
  icon?: RemixiconComponentType
}

interface FormChoiceCardsProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  options: ChoiceCard[]
  label?: string
  description?: string
}

/** Pick one of a few options shown as cards with a title and an explanation (e.g. the difficulty mode). */
export default function FormChoiceCards<T extends FieldValues>({ control, name, options, label, description }: FormChoiceCardsProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FieldSet data-invalid={fieldState.invalid || undefined}>
          {label && <FieldLegend variant="label">{label}</FieldLegend>}
          {description && <FieldDescription>{description}</FieldDescription>}
          <div role="radiogroup" aria-label={label} className="grid gap-3 sm:grid-cols-2">
            {options.map((o) => {
              const selected = field.value === o.value
              const Icon = o.icon
              return (
                <button
                  key={o.value}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  onClick={() => field.onChange(o.value)}
                  className={cn(
                    'flex flex-col gap-2 border-2 border-border bg-card p-4 text-left transition-colors hover:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                    selected && 'border-primary shadow-[var(--glow-gold)]',
                  )}
                >
                  <span className={cn('flex items-center gap-2 font-heading text-lg', selected && 'text-gold')}>
                    {Icon && <Icon className="size-5" aria-hidden />}
                    {o.label}
                  </span>
                  <span className="text-sm text-muted-foreground">{o.description}</span>
                </button>
              )
            })}
          </div>
          {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
        </FieldSet>
      )}
    />
  )
}
