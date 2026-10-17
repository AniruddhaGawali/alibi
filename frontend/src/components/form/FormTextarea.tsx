import type { ComponentProps } from 'react'
import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Textarea } from '@/components/ui/textarea'
import FormFieldShell from './FormFieldShell'

interface FormTextareaProps<T extends FieldValues>
  extends Omit<ComponentProps<typeof Textarea>, 'name' | 'defaultValue'> {
  control: Control<T>
  name: Path<T>
  label?: string
  description?: string
}

export default function FormTextarea<T extends FieldValues>({
  control,
  name,
  label,
  description,
  ...props
}: FormTextareaProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
          <Textarea {...props} {...field} id={name} aria-invalid={fieldState.invalid} />
        </FormFieldShell>
      )}
    />
  )
}
