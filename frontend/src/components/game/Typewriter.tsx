import { useEffect, useState } from 'react'
import { play } from '@/utils/sound'

/** Types `text` out character by character, like a police report; instant for reduced-motion users. */
export default function Typewriter({ text, speedMs = 28 }: { text: string; speedMs?: number }) {
  const reduce = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const [count, setCount] = useState(0)

  useEffect(() => {
    if (reduce) return
    const id = setInterval(() => {
      setCount((c) => {
        if (c >= text.length) {
          clearInterval(id)
          return c
        }
        if (text[c] !== ' ') play('key')
        return c + 1
      })
    }, speedMs)
    return () => clearInterval(id)
  }, [text, speedMs, reduce])

  const shown = reduce ? text : text.slice(0, count)
  return (
    <span className="font-typewriter" aria-label={text}>
      <span aria-hidden>{shown}</span>
      {!reduce && count < text.length && <span aria-hidden className="typewriter-caret" />}
    </span>
  )
}
