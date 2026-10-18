import type { ComponentProps } from 'react'
import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Input } from '@/components/ui/input'
import FormFieldShell from './FormFieldShell'

interface FormFileProps<T extends FieldValues>
  extends Omit<ComponentProps<typeof Input>, 'name' | 'type' | 'value' | 'defaultValue' | 'onChange'> {
  control: Control<T>
  name: Path<T>
  label?: string
  description?: string
  /** Called after the form value changed (e.g. to clear a competing field). */
  onFileChange?: (file: File | undefined) => void
}

/** File picker; the form value is a `File` (or undefined). Size/type rules belong in the zod schema. */
export default function FormFile<T extends FieldValues>({
  control,
  name,
  label,
  description,
  onFileChange,
  ...props
}: FormFileProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
          <Input
            {...props}
            id={name}
            name={field.name}
            ref={field.ref}
            type="file"
            aria-invalid={fieldState.invalid}
            onBlur={field.onBlur}
            onChange={(e) => {
              const file = e.target.files?.[0]
              field.onChange(file)
              onFileChange?.(file)
            }}
          />
        </FormFieldShell>
      )}
    />
  )
}
