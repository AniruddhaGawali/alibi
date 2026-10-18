import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'
import { Button } from '@/components/ui/button'
import { FieldGroup, FieldSeparator } from '@/components/ui/field'
import { FormFile, FormImageChoice, type ImageChoice } from '@/components/form'
import { useSetAvatarMutation } from '@/api/user'
import { defaultAvatarSrc } from '@/utils/avatar'
import AuthCard from './AuthCard'

const MAX_BYTES = 1024 * 1024 // matches the backend's 1 MB limit
const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif']

const DEFAULT_AVATARS: ImageChoice[] = [1, 2, 3, 4].map((id) => ({
  value: id,
  src: defaultAvatarSrc(id),
  alt: `Default Avatar ${id}`,
}))

const schema = z
  .object({
    file: z.instanceof(File).optional(),
    defaultId: z.number().int().min(1).max(4).nullable(),
  })
  .superRefine((v, ctx) => {
    if (v.file) {
      if (v.file.size > MAX_BYTES) {
        ctx.addIssue({ code: 'custom', path: ['file'], message: 'Image must be 1 MB or smaller' })
      } else if (!IMAGE_TYPES.includes(v.file.type)) {
        ctx.addIssue({ code: 'custom', path: ['file'], message: 'Use a JPEG, PNG or GIF image' })
      }
    } else if (v.defaultId === null) {
      ctx.addIssue({ code: 'custom', path: ['defaultId'], message: 'Upload an image or choose a default avatar' })
    }
  })
type Values = z.infer<typeof schema>

export default function AvatarForm() {
  const navigate = useNavigate()
  const { control, handleSubmit, setValue, resetField } = useForm<Values>({
    resolver: zodResolver(schema),
    mode: 'onChange', // reject an oversized file as soon as it is picked
    defaultValues: { file: undefined, defaultId: null },
  })
  const setAvatar = useSetAvatarMutation({
    onSuccess: () => navigate('/dashboard'),
    onError: (e) => toast.error(e.message),
  })

  return (
    <AuthCard title="Choose your avatar" description="Upload your own image or pick one of the defaults.">
      <form
        onSubmit={handleSubmit((v) => setAvatar.mutate(v.file ? { file: v.file } : { avatar: v.defaultId! }))}
        noValidate
      >
        <FieldGroup>
          <FormFile
            control={control}
            name="file"
            label="Upload a custom avatar"
            description="JPEG, PNG or GIF, up to 1 MB."
            accept={IMAGE_TYPES.join(',')}
            onFileChange={(file) => {
              if (file) setValue('defaultId', null)
            }}
          />
          <FieldSeparator>or</FieldSeparator>
          <FormImageChoice
            control={control}
            name="defaultId"
            label="Choose default"
            options={DEFAULT_AVATARS}
            onValueChange={(id) => {
              if (id !== null) resetField('file')
            }}
          />
          <Button type="submit" disabled={setAvatar.isPending}>
            {setAvatar.isPending ? 'Saving…' : 'Continue'}
          </Button>
        </FieldGroup>
      </form>
    </AuthCard>
  )
}
