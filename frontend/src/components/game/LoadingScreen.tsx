import { RiSearchLine } from '@remixicon/react'
import { cn } from '@/lib/utils'

/** Full-area loading state: a magnifier sweeping over a case file. */
export default function LoadingScreen({ className, fullscreen = false }: { className?: string; fullscreen?: boolean }) {
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        'flex flex-col items-center justify-center gap-4 text-center',
        fullscreen ? 'min-h-screen bg-background' : 'min-h-[50vh]',
        className,
      )}
    >
      <div className="relative size-20 text-gold">
        <RiSearchLine className="magnifier-sweep absolute inset-0 size-20 drop-shadow-[0_0_8px_var(--gold)]" aria-hidden />
      </div>
      <p className="font-typewriter text-lg text-muted-foreground">
        Searching the archives<span className="loading-dots" aria-hidden />
      </p>
    </div>
  )
}
