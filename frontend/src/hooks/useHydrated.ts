import { useSyncExternalStore } from 'react'

const subscribe = () => () => {}

/**
 * false while the page is prerendered and during hydration, true afterwards. Anything that
 * depends on the browser (cookies, localStorage) uses it, so the first client render matches
 * the prerendered HTML exactly and the real value appears right after.
 */
export function useHydrated(): boolean {
  return useSyncExternalStore(subscribe, () => true, () => false)
}
