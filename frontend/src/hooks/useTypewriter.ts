import { useEffect, useRef, useState } from 'react'
import { play } from '@/utils/sound'

const prefersReducedMotion = () =>
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

interface Options {
  /** Milliseconds per character. */
  speedMs?: number
  /** false shows the whole text straight away (history, reduced motion). */
  animate?: boolean
  /** Called once, when the text is fully shown (typed out or skipped). */
  onDone?: () => void
}

/**
 * Reveals `text` one character at a time, like an old police typewriter, with a key
 * click for every other letter. `skip()` shows the rest at once. Changing `text`
 * starts over; users who prefer reduced motion get the full text immediately.
 */
export function useTypewriter(text: string, { speedMs = 28, animate = true, onDone }: Options = {}) {
  const instant = !animate || prefersReducedMotion()
  const [count, setCount] = useState(instant ? text.length : 0)
  const [shownFor, setShownFor] = useState(text)
  if (shownFor !== text) {
    setShownFor(text)
    setCount(instant ? text.length : 0)
  }

  const done = count >= text.length
  const typing = !instant && !done

  useEffect(() => {
    if (!typing) return
    const id = setInterval(() => setCount((c) => c + 1), speedMs)
    return () => clearInterval(id)
  }, [typing, speedMs])

  // a key click for every other letter typed (not for spaces, not when skipping ahead)
  const previous = useRef(count)
  useEffect(() => {
    if (typing && count - previous.current === 1 && count % 2 === 0 && text[count - 1]?.trim()) play('key')
    previous.current = count
  }, [count, typing, text])

  const onDoneRef = useRef(onDone)
  useEffect(() => {
    onDoneRef.current = onDone
  })
  useEffect(() => {
    if (done) onDoneRef.current?.()
  }, [done])

  return { shown: text.slice(0, count), typing, skip: () => setCount(text.length) }
}
