import type { AvatarInfo } from '@/api/user'
import { assetUrl } from './assets'

/** Default avatars 1-4 are stored at avatars/default/avatar-N.webp. */
export const defaultAvatarSrc = (id: number): string => assetUrl(`avatars/default/avatar-${id}.webp`)

/** Image URL for a user's avatar, or undefined when they have not chosen one. */
export function avatarSrc(avatar: AvatarInfo): string | undefined {
  // custom avatars come back from the API as a full public URL in the bucket
  if (avatar.type === 'custom' && avatar.url) return avatar.url
  if (avatar.type === 'default' && avatar.id) return defaultAvatarSrc(avatar.id)
  return undefined
}
