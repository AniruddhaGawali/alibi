/**
 * Images live in the object store (Supabase Storage today; S3/GCS later), at the
 * same paths as in frontend/public. VITE_ASSET_BASE_URL is the bucket's public
 * base, e.g. https://<project>.supabase.co/storage/v1/object/public/alibi-assets.
 * Unset, paths stay relative to the app's own origin, but frontend/public holds
 * no images any more, so set it (see frontend/.env.example) to see any art.
 */
export const ASSET_BASE_URL = (import.meta.env.VITE_ASSET_BASE_URL ?? '').replace(/\/+$/, '')

/** Public URL of an image by its object path ("avatars/default/avatar-1.webp"). */
export function assetUrl(path: string): string {
  if (/^https?:\/\//.test(path)) return path
  return `${ASSET_BASE_URL}/${path.replace(/^\/+/, '')}`
}
