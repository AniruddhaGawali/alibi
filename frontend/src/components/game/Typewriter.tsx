import { useTypewriter } from '@/hooks/useTypewriter'

interface TypewriterProps {
  text: string
  speedMs?: number
  /** false shows the text without typing it out. */
  animate?: boolean
  onDone?: () => void
  className?: string
}

/**
 * Types `text` out character by character, like a police report. While it types, a
 * click (or Enter / Space) skips to the full text.
 */
export default function Typewriter({ text, speedMs = 28, animate = true, onDone, className }: TypewriterProps) {
  const { shown, typing, skip } = useTypewriter(text, { speedMs, animate, onDone })

  return (
    <span
      className={`font-typewriter ${typing ? 'cursor-pointer' : ''} ${className ?? ''}`}
      aria-label={text}
      {...(typing && {
        role: 'button',
        tabIndex: 0,
        title: 'Click to skip',
        onClick: skip,
        onKeyDown: (e: React.KeyboardEvent) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            skip()
          }
        },
      })}
    >
      <span aria-hidden>{shown}</span>
      {typing && <span aria-hidden className="typewriter-caret" />}
    </span>
  )
}
