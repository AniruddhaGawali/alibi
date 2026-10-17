import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import FormFieldShell from './FormFieldShell'

export interface SelectOption {
  value: string
  label: string
}

interface FormSelectProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  options: SelectOption[]
  label?: string
  description?: string
  placeholder?: string
  disabled?: boolean
}

export default function FormSelect<T extends FieldValues>({
  control,
  name,
  options,
  label,
  description,
  placeholder,
  disabled,
}: FormSelectProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
          <Select name={field.name} value={field.value ?? ''} onValueChange={field.onChange} disabled={disabled}>
            <SelectTrigger id={name} aria-invalid={fieldState.invalid} onBlur={field.onBlur}>
              <SelectValue placeholder={placeholder} />
            </SelectTrigger>
            <SelectContent>
              {options.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormFieldShell>
      )}
    />
  )
}
