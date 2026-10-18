import { useEffect, useRef } from 'react'

/**
 * Keeps a scrolling chat pinned to its newest message. Pass something that changes
 * whenever a message is added (a count): the window then jumps to the bottom. While
 * the content grows on its own (a reply being typed out) it follows only if the
 * reader is already at the bottom, so scrolling up to reread something still works.
 */
export function useStickToBottom<S extends HTMLElement, C extends HTMLElement>(trigger: unknown) {
  const scrollRef = useRef<S>(null)
  const contentRef = useRef<C>(null)
  const stuck = useRef(true)

  useEffect(() => {
    stuck.current = true
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [trigger])

  useEffect(() => {
    const el = scrollRef.current
    const content = contentRef.current
    if (!el || !content) return
    const follow = () => {
      if (stuck.current) el.scrollTop = el.scrollHeight
    }
    const onScroll = () => {
      stuck.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
    }
    const observer = new ResizeObserver(follow)
    observer.observe(content)
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      observer.disconnect()
      el.removeEventListener('scroll', onScroll)
    }
  }, [])

  return { scrollRef, contentRef }
}
