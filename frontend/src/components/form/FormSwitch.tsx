import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Switch } from '@/components/ui/switch'
import FormFieldShell from './FormFieldShell'

interface FormSwitchProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  label: string
  description?: string
  disabled?: boolean
  /** Called after the form value changed (e.g. to switch off a competing field). */
  onCheckedChange?: (checked: boolean) => void
}

export default function FormSwitch<T extends FieldValues>({
  control,
  name,
  label,
  description,
  disabled,
  onCheckedChange,
}: FormSwitchProps<T>) {
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
          <Switch
            id={name}
            name={field.name}
            checked={!!field.value}
            onCheckedChange={(checked) => {
              field.onChange(checked)
              onCheckedChange?.(checked)
            }}
            onBlur={field.onBlur}
            disabled={disabled}
            aria-invalid={fieldState.invalid}
          />
        </FormFieldShell>
      )}
    />
  )
}
