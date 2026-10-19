import { useWatch, type Control } from 'react-hook-form'
import { Button } from '@/components/ui/button'
import { FormCombobox, type ComboboxOption } from '@/components/form'
import { Skeleton } from '@/components/ui/skeleton'
import { useGetAdminModelsQuery, type AdminModel } from '@/api/admin'
import { DEFAULT_MODEL, type CaseFormValues } from './caseSchema'

function price(m: AdminModel): string {
  if (m.free) return 'Free'
  const fmt = (v: number) => `$${v < 1 ? v.toFixed(2) : v.toFixed(1)}`
  return `${fmt(m.prompt_price)} in / ${fmt(m.completion_price)} out per M tokens`
}

function context(tokens: number): string {
  if (!tokens) return ''
  return tokens >= 1000 ? ` · ${Math.round(tokens / 1000)}k context` : ` · ${tokens} context`
}

const toOption = (m: AdminModel): ComboboxOption => ({
  value: m.id,
  label: m.name || m.id,
  hint: `${m.id} · ${price(m)}${context(m.context_length)}`,
  keywords: [m.id, m.free ? 'free' : ''],
})

/**
 * The AI model of a case, picked from the OpenRouter models the backend allows (text
 * output only, within the price cap), so an id can never be mistyped.
 */
export default function ModelPicker({ control, disabled }: { control: Control<CaseFormValues>; disabled?: boolean }) {
  const { data, isPending, error, refetch } = useGetAdminModelsQuery()
  const current = useWatch({ control, name: 'llm_model' })

  if (isPending) return <Skeleton className="h-16 w-full" />

  const options = (data?.models ?? []).map(toOption)
  const fallback = data?.default ?? DEFAULT_MODEL
  // Keep the saved and default models selectable even if the list lacks them
  // (OpenRouter unreachable, or the model was retired since).
  for (const id of [current, fallback]) {
    if (id && !options.some((o) => o.value === id)) {
      options.unshift({ value: id, label: id, hint: error ? 'Saved model (list unavailable)' : 'No longer offered by OpenRouter' })
    }
  }

  return (
    <div className="space-y-2">
      <FormCombobox
        control={control}
        name="llm_model"
        label="AI model"
        description={`Plays every suspect of this case. Only text models within the price limit are listed; new cases get ${fallback}.`}
        options={options}
        placeholder="Pick a model"
        searchPlaceholder="Search by name, id or “free”…"
        emptyText="No model matches."
        disabled={disabled}
      />
      {error && (
        <p className="flex items-center gap-2 text-sm text-destructive">
          {error.message}
          <Button type="button" variant="outline" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </p>
      )}
    </div>
  )
}
