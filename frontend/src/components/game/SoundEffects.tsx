import { useEffect } from 'react'
import { play } from '@/utils/sound'

const CLICKABLE = 'button, a[href], [role="button"], [role="tab"], [role="menuitem"], [role="option"]'

/** Plays a click for every enabled button / link press. Mount once in App. */
export default function SoundEffects() {
  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      const el = (e.target as Element | null)?.closest(CLICKABLE)
      if (el && !el.hasAttribute('disabled') && el.getAttribute('aria-disabled') !== 'true') play('click')
    }
    document.addEventListener('pointerdown', onDown, true)
    return () => document.removeEventListener('pointerdown', onDown, true)
  }, [])
  return null
}
