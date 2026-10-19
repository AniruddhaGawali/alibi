import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { RiCloseLine, RiImageLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import FormFieldShell from './FormFieldShell'
import { IMAGE_TYPES, emptyImage, type ImageValue } from './imageValue'

interface FormImagePickerProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  label?: string
  description?: string
  /** Preview shape: wide for thumbnails and scenes, portrait for suspects. */
  aspect?: 'wide' | 'portrait'
  disabled?: boolean
}

function Preview({ value, aspect }: { value: ImageValue; aspect: 'wide' | 'portrait' }) {
  const src = value.preview ?? (value.url || undefined)
  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden border bg-muted/30 text-muted-foreground',
        aspect === 'wide' ? 'aspect-video w-40' : 'aspect-[3/4] w-24',
      )}
    >
      {src ? <img src={src} alt="" className="size-full object-cover" /> : <RiImageLine className="size-6" aria-hidden />}
    </div>
  )
}

export default function FormImagePicker<T extends FieldValues>({
  control,
  name,
  label,
  description,
  aspect = 'wide',
  disabled,
}: FormImagePickerProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => {
        const value = (field.value ?? emptyImage()) as ImageValue
        // The preview URL is made here, not in an effect, and the previous one is freed.
        const change = (next: ImageValue) => {
          if (value.preview) URL.revokeObjectURL(value.preview)
          field.onChange(next)
        }
        return (
          <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
            <div className="flex items-start gap-3">
              <Preview value={value} aspect={aspect} />
              <div className="flex min-w-0 flex-1 flex-col gap-2">
                <Input
                  id={name}
                  name={field.name}
                  ref={field.ref}
                  type="file"
                  accept={IMAGE_TYPES.join(',')}
                  disabled={disabled}
                  aria-invalid={fieldState.invalid}
                  onBlur={field.onBlur}
                  onChange={(e) => {
                    const file = e.target.files?.[0] ?? null
                    change({ url: value.url, file, preview: file ? URL.createObjectURL(file) : undefined })
                  }}
                />
                {value.file && <p className="truncate text-xs text-muted-foreground">Uploads when you save: {value.file.name}</p>}
                {(value.url || value.file) && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="self-start"
                    disabled={disabled}
                    onClick={() => change(emptyImage())}
                  >
                    <RiCloseLine className="size-4" aria-hidden />
                    Remove image
                  </Button>
                )}
              </div>
            </div>
          </FormFieldShell>
        )
      }}
    />
  )
}
