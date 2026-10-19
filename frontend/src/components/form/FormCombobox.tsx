import { useState, type ReactNode } from 'react'
import { Controller, type FieldValues, type Path, type Control } from 'react-hook-form'
import { RiArrowDownSLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import FormFieldShell from './FormFieldShell'

export interface ComboboxOption {
  value: string
  label: string
  /** Second line under the label (e.g. an id or a price). */
  hint?: ReactNode
  /** Extra words the search matches besides the label and value. */
  keywords?: string[]
}

interface FormComboboxProps<T extends FieldValues> {
  control: Control<T>
  name: Path<T>
  options: ComboboxOption[]
  label?: string
  description?: string
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  disabled?: boolean
}

/**
 * A searchable dropdown for long lists: the value can only be one of `options`, so
 * nothing can be mistyped. A value that is not among the options (e.g. one saved
 * before it was removed) is still shown on the button.
 */
export default function FormCombobox<T extends FieldValues>({
  control,
  name,
  options,
  label,
  description,
  placeholder = 'Select…',
  searchPlaceholder = 'Search…',
  emptyText = 'Nothing found.',
  disabled,
}: FormComboboxProps<T>) {
  const [open, setOpen] = useState(false)

  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => {
        const selected = options.find((o) => o.value === field.value)
        return (
          <FormFieldShell name={name} label={label} description={description} fieldState={fieldState}>
            <Popover open={open} onOpenChange={setOpen}>
              <PopoverTrigger asChild>
                <Button
                  id={name}
                  ref={field.ref}
                  type="button"
                  variant="outline"
                  role="combobox"
                  aria-expanded={open}
                  aria-invalid={fieldState.invalid}
                  disabled={disabled}
                  onBlur={field.onBlur}
                  className="w-full justify-between font-normal"
                >
                  <span className={cn('truncate', !field.value && 'text-muted-foreground')}>
                    {selected?.label ?? (field.value || placeholder)}
                  </span>
                  <RiArrowDownSLine className="size-4 opacity-60" aria-hidden />
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-(--radix-popover-trigger-width) min-w-80 p-0" align="start">
                <Command>
                  <CommandInput placeholder={searchPlaceholder} />
                  <CommandList>
                    <CommandEmpty>{emptyText}</CommandEmpty>
                    {options.map((o) => (
                      <CommandItem
                        key={o.value}
                        value={o.value}
                        keywords={[o.label, ...(o.keywords ?? [])]}
                        data-checked={o.value === field.value}
                        onSelect={() => {
                          field.onChange(o.value)
                          setOpen(false)
                        }}
                      >
                        <div className="min-w-0">
                          <p className="truncate">{o.label}</p>
                          {o.hint && <p className="truncate text-xs text-muted-foreground">{o.hint}</p>}
                        </div>
                      </CommandItem>
                    ))}
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </FormFieldShell>
        )
      }}
    />
  )
}
