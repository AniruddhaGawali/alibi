import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'
import { RiDraftLine, RiSave3Line, RiSendPlaneLine } from '@remixicon/react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { AdminCaseDetail } from '@/api/admin'
import { showApiError } from '@/utils/formErrors'
import AdminSection from './AdminSection'
import { CaseBasicsSection, CaseStorySection } from './CaseDetailsSections'
import ScenesSection from './ScenesSection'
import SuspectsSection from './SuspectsSection'
import { caseSchema, toFormValues, type CaseFormValues } from './caseSchema'
import { useSaveCase } from './useSaveCase'

/** Fields the server can name in an error that sit directly on the case form. */
const CASE_FIELDS = ['title', 'synopsis', 'llm_model', 'main_plot', 'resolution'] as const

/**
 * The long-scroll case builder: basic info, story and hints, suspects with their AI
 * persona, scene images and publishing, saved together with one button. Mount it
 * with a `key` that changes when the server data does, so it starts from fresh values.
 */
export default function CaseBuilderForm({ initial }: { initial?: AdminCaseDetail }) {
  const navigate = useNavigate()
  const { control, handleSubmit, setValue, setError, formState, getValues } = useForm<CaseFormValues>({
    resolver: zodResolver(caseSchema),
    defaultValues: toFormValues(initial),
  })
  const { save, step, saving } = useSaveCase(initial, setValue)

  const live = !!initial?.is_published
  const onSubmit = handleSubmit(async (values) => {
    try {
      const id = await save(values)
      toast.success(
        values.is_published ? (live ? 'Case saved' : 'Case published') : live ? 'Case moved back to drafts' : 'Draft saved',
      )
      if (!initial) navigate(`/admin/cases/${id}`, { replace: true })
    } catch (e) {
      showApiError(e as Error, setError, CASE_FIELDS)
    }
  })
  // Both buttons submit the same form; they only differ in whether the case ends up published.
  // The flag is set before validation so the publish rules (suspects, one culprit) are checked.
  const submit = (publish: boolean) => {
    setValue('is_published', publish)
    void onSubmit()
  }
  const publishError = formState.errors.is_published?.message

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-8">
      <CaseBasicsSection control={control} disabled={saving} />
      <CaseStorySection control={control} disabled={saving} />
      <SuspectsSection control={control} setValue={setValue} disabled={saving} />
      <ScenesSection control={control} disabled={saving} />

      <AdminSection title="Save" className="sticky bottom-4 z-10">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="space-y-1">
            <p className="flex items-center gap-2 text-sm">
              Status: {live ? <Badge>Published</Badge> : <Badge variant="outline">{initial ? 'Draft' : 'New draft'}</Badge>}
            </p>
            <p className="text-sm text-muted-foreground">
              {live
                ? 'Live on every player’s dashboard. Move it back to drafts to rework it safely.'
                : 'Drafts are only visible here. Publishing needs at least 2 suspects and one culprit.'}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            {step && <span className="text-sm text-muted-foreground">{step}…</span>}
            {!step && publishError && <span className="text-sm text-destructive">{publishError}</span>}
            {!step && !publishError && formState.isSubmitted && !formState.isValid && (
              <span className="text-sm text-destructive">Fix the highlighted fields.</span>
            )}
            <Button type="button" variant="outline" disabled={saving} onClick={() => submit(false)}>
              <RiDraftLine className="size-4" aria-hidden />
              {saving && !getValues('is_published') ? 'Saving…' : live ? 'Move to draft' : 'Save as draft'}
            </Button>
            <Button type="button" disabled={saving} onClick={() => submit(true)}>
              {live ? <RiSave3Line className="size-4" aria-hidden /> : <RiSendPlaneLine className="size-4" aria-hidden />}
              {saving && getValues('is_published') ? 'Saving…' : live ? 'Save' : 'Save & publish'}
            </Button>
          </div>
        </div>
      </AdminSection>
    </form>
  )
}
