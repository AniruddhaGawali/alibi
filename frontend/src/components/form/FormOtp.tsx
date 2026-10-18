import { REGEXP_ONLY_DIGITS } from 'input-otp'
import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { InputOTP, InputOTPGroup, InputOTPSlot } from '@/components/ui/input-otp'
import FormFieldShell from './FormFieldShell'

interface FormOtpProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  label?: string
  description?: string
  length?: number
  disabled?: boolean
}

/** One-time-code field built on the shadcn InputOTP (digits only). */
export default function FormOtp<T extends FieldValues>({
  control,
  name,
  label,
  description,
  length = 6,
  disabled,
}: FormOtpProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
          <InputOTP
            id={name}
            maxLength={length}
            pattern={REGEXP_ONLY_DIGITS}
            inputMode="numeric"
            autoComplete="one-time-code"
            disabled={disabled}
            value={field.value ?? ''}
            onChange={field.onChange}
            onBlur={field.onBlur}
          >
            <InputOTPGroup>
              {Array.from({ length }, (_, i) => (
                <InputOTPSlot key={i} index={i} aria-invalid={fieldState.invalid} />
              ))}
            </InputOTPGroup>
          </InputOTP>
        </FormFieldShell>
      )}
    />
  )
}
