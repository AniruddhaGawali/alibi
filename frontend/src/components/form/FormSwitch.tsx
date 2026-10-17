import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { Switch } from '@/components/ui/switch'
import FormFieldShell from './FormFieldShell'

interface FormSwitchProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  label: string
  description?: string
  disabled?: boolean
}

export default function FormSwitch<T extends FieldValues>({
  control,
  name,
  label,
  description,
  disabled,
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
