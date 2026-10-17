import type { ComponentProps } from 'react'
import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Input } from '@/components/ui/input'
import FormFieldShell from './FormFieldShell'

interface FormInputProps<T extends FieldValues>
  extends Omit<ComponentProps<typeof Input>, 'name' | 'defaultValue'> {
  control: Control<T>
  name: Path<T>
  label?: string
  description?: string
}

export default function FormInput<T extends FieldValues>({
  control,
  name,
  label,
  description,
  ...props
}: FormInputProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
          <Input {...props} {...field} id={name} aria-invalid={fieldState.invalid} />
        </FormFieldShell>
      )}
    />
  )
}
