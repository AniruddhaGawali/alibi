import type { ReactNode } from 'react'
import type { ControllerFieldState } from 'react-hook-form'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/components/ui/field'

interface FormFieldShellProps {
  name: string
  label?: string
  description?: string
  fieldState: ControllerFieldState
  orientation?: 'vertical' | 'horizontal'
  children: ReactNode
}

/** Shared label / description / error layout for every form control. */
export default function FormFieldShell({
  name,
  label,
  description,
  fieldState,
  orientation = 'vertical',
  children,
}: FormFieldShellProps) {
  const horizontal = orientation === 'horizontal'
  return (
    <Field orientation={orientation} data-invalid={fieldState.invalid || undefined}>
      {!horizontal && label && <FieldLabel htmlFor={name}>{label}</FieldLabel>}
      {children}
      {horizontal && label && <FieldLabel htmlFor={name}>{label}</FieldLabel>}
      {description && <FieldDescription>{description}</FieldDescription>}
      {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
    </Field>
  )
}
