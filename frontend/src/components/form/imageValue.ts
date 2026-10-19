import { z } from 'zod'

/** An image field: the saved URL, plus a picked file that is uploaded when the form is saved. */
export interface ImageValue {
  url: string
  file: File | null
  /** Object URL showing the picked file until it is uploaded. */
  preview?: string
}

export const emptyImage = (url = ''): ImageValue => ({ url, file: null })

const MAX_IMAGE_BYTES = 10 * 1024 * 1024
export const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']

/** Zod schema for an `ImageValue`; the server re-encodes to WebP and allows 10 MB. */
export const imageValueSchema = z.object({
  url: z.string(),
  preview: z.string().optional(),
  file: z
    .instanceof(File)
    .refine((f) => IMAGE_TYPES.includes(f.type), 'Pick a JPEG, PNG, GIF or WebP image')
    .refine((f) => f.size <= MAX_IMAGE_BYTES, 'Image must be at most 10 MB')
    .nullable(),
})
