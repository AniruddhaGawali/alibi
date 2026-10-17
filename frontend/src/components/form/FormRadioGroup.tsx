import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Field, FieldDescription, FieldError, FieldLabel, FieldLegend, FieldSet } from '@/components/ui/field'
import type { SelectOption } from './FormSelect'

interface FormRadioGroupProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  options: SelectOption[]
  label?: string
  description?: string
}

export default function FormRadioGroup<T extends FieldValues>({
  control,
  name,
  options,
  label,
  description,
}: FormRadioGroupProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FieldSet data-invalid={fieldState.invalid || undefined}>
          {label && <FieldLegend variant="label">{label}</FieldLegend>}
          {description && <FieldDescription>{description}</FieldDescription>}
          <RadioGroup name={field.name} value={field.value ?? ''} onValueChange={field.onChange}>
            {options.map((o) => (
              <Field key={o.value} orientation="horizontal" data-invalid={fieldState.invalid || undefined}>
                <RadioGroupItem value={o.value} id={`${name}-${o.value}`} aria-invalid={fieldState.invalid} />
                <FieldLabel htmlFor={`${name}-${o.value}`}>{o.label}</FieldLabel>
              </Field>
            ))}
          </RadioGroup>
          {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
        </FieldSet>
      )}
    />
  )
}
