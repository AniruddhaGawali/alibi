import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Checkbox } from '@/components/ui/checkbox'
import FormFieldShell from './FormFieldShell'

interface FormCheckboxProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  label: string
  description?: string
  disabled?: boolean
}

export default function FormCheckbox<T extends FieldValues>({
  control,
  name,
  label,
  description,
  disabled,
}: FormCheckboxProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormFieldShell
          name={name}
          label={label}
          description={description}
          fieldState={fieldState}
          orientation="horizontal"
        >
          <Checkbox
            id={name}
            name={field.name}
            checked={!!field.value}
            onCheckedChange={field.onChange}
            onBlur={field.onBlur}
            disabled={disabled}
            aria-invalid={fieldState.invalid}
          />
        </FormFieldShell>
      )}
    />
  )
}
