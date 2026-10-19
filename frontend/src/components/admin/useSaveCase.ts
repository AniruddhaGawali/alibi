import { useState } from 'react'
import type { UseFormSetValue } from 'react-hook-form'
import {
  useCreateAdminCaseMutation,
  useCreateAdminSceneMutation,
  useCreateAdminSuspectMutation,
  useDeleteAdminSceneMutation,
  useDeleteAdminSuspectMutation,
  useUpdateAdminCaseMutation,
  useUpdateAdminSceneMutation,
  useUpdateAdminSuspectMutation,
  useUploadAdminImageMutation,
  type AdminCaseDetail,
  type ImageKind,
} from '@/api/admin'
import { ApiError } from '@/api/apiClient'
import { invalidateTags } from '@/api/createApi'
import { queryClient } from '@/api/queryClient'
import { emptyImage, type ImageValue } from '@/components/form'
import { caseFields, changedFields, sceneFields, suspectFields, type CaseFormValues } from './caseSchema'

/** A delete that finds nothing already happened (e.g. on a retry after a partial save). */
async function ignoreMissing(p: Promise<unknown>) {
  try {
    await p
  } catch (e) {
    if (!(e instanceof ApiError && e.status === 404)) throw e
  }
}

/**
 * Saves the whole case builder through the admin API, in an order that keeps a
 * published case playable at every step: unpublish first, then the case fields,
 * suspects (the culprit first, so the flag moves instead of leaving none), removed
 * suspects, scenes, and publish last. Picked images are uploaded on the way. Ids of
 * what gets created are written back into the form, so a retry after a failure
 * updates instead of creating duplicates. Returns the case id.
 */
export function useSaveCase(server: AdminCaseDetail | undefined, setValue: UseFormSetValue<CaseFormValues>) {
  const [caseId, setCaseId] = useState(server?.id)
  const [step, setStep] = useState<string>()
  const createCase = useCreateAdminCaseMutation()
  const updateCase = useUpdateAdminCaseMutation()
  const createSuspect = useCreateAdminSuspectMutation()
  const updateSuspect = useUpdateAdminSuspectMutation()
  const deleteSuspect = useDeleteAdminSuspectMutation()
  const createScene = useCreateAdminSceneMutation()
  const updateScene = useUpdateAdminSceneMutation()
  const deleteScene = useDeleteAdminSceneMutation()
  const upload = useUploadAdminImageMutation()

  const save = async (v: CaseFormValues): Promise<string> => {
    try {
      const imageUrl = async (img: ImageValue, kind: ImageKind, id: string) =>
        img.file ? (await upload.mutateAsync({ kind, caseId: id, file: img.file })).url : img.url

      // 1. Unpublish before anything that could make the live case unplayable.
      if (caseId && server?.is_published && !v.is_published) {
        setStep('Unpublishing')
        await updateCase.mutateAsync({ id: caseId, is_published: false })
      }

      // 2. The case itself. A new case is created as a draft.
      setStep('Saving the case file')
      const fields = caseFields(v)
      let id = caseId
      let prev = server
      if (!id) {
        const created = await createCase.mutateAsync({ ...fields, thumbnail_url: '' })
        id = created.id
        setCaseId(id)
        prev = { ...created, suspects: [], scenes: [] }
      }
      const thumbnail_url = await imageUrl(v.thumbnail, 'thumbnail', id)
      setValue('thumbnail', emptyImage(thumbnail_url))
      const patch = changedFields({ ...fields, thumbnail_url }, prev)
      if (Object.keys(patch).length > 0) await updateCase.mutateAsync({ id, ...patch })

      // 3. Suspects, the culprit first.
      const order = v.suspects.map((_, i) => i).sort((a, b) => Number(v.suspects[b].is_culprit) - Number(v.suspects[a].is_culprit))
      for (const i of order) {
        const s = v.suspects[i]
        setStep(`Saving suspect ${s.name}`)
        const avatar_url = await imageUrl(s.avatar, 'suspect', id)
        setValue(`suspects.${i}.avatar`, emptyImage(avatar_url))
        const body = suspectFields(s, avatar_url, i)
        if (!s.id) {
          const created = await createSuspect.mutateAsync({ caseId: id, ...body })
          setValue(`suspects.${i}.id`, created.id)
          continue
        }
        const changes = changedFields(body, prev?.suspects.find((p) => p.id === s.id))
        if (Object.keys(changes).length > 0) await updateSuspect.mutateAsync({ caseId: id, id: s.id, ...changes })
      }
      const keptSuspects = new Set(v.suspects.map((s) => s.id))
      for (const s of prev?.suspects ?? []) {
        if (keptSuspects.has(s.id)) continue
        setStep(`Removing suspect ${s.name}`)
        await ignoreMissing(deleteSuspect.mutateAsync({ caseId: id, id: s.id }))
      }

      // 4. Scenes: removals first, so their positions are free again.
      const keptScenes = new Set(v.scenes.map((s) => s.id))
      for (const s of prev?.scenes ?? []) {
        if (!keptScenes.has(s.id)) await ignoreMissing(deleteScene.mutateAsync({ caseId: id, id: s.id }))
      }
      for (const [i, s] of v.scenes.entries()) {
        setStep(`Saving scene ${s.scene_name}`)
        const image_url = await imageUrl(s.image, 'scene', id)
        setValue(`scenes.${i}.image`, emptyImage(image_url))
        const body = sceneFields(s, image_url)
        if (!s.id) {
          const created = await createScene.mutateAsync({ caseId: id, ...body })
          setValue(`scenes.${i}.id`, created.id)
          continue
        }
        const changes = changedFields(body, prev?.scenes.find((p) => p.id === s.id))
        if (Object.keys(changes).length > 0) await updateScene.mutateAsync({ caseId: id, id: s.id, ...changes })
      }

      // 5. Publish last, once the suspects it needs exist.
      if (v.is_published && !server?.is_published) {
        setStep('Publishing')
        await updateCase.mutateAsync({ id, is_published: true })
      }
      // Only after a full save: refetching reloads the builder from the server, which
      // after a failure would throw away the edits that were not saved yet.
      invalidateTags(queryClient, ['AdminCases', 'Stories'])
      return id
    } finally {
      setStep(undefined)
    }
  }

  return { save, step, saving: step !== undefined }
}
