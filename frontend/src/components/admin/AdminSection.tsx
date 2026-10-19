import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface AdminSectionProps {
  title: string
  description?: string
  /** Buttons shown on the right of the heading. */
  actions?: ReactNode
  className?: string
  children: ReactNode
}

/** A framed panel of the admin panel with a heading. */
export default function AdminSection({ title, description, actions, className, children }: AdminSectionProps) {
  return (
    <section className={cn('frame space-y-5 border bg-card p-6', className)}>
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-xl text-primary">{title}</h2>
          {description && <p className="text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions}
      </header>
      {children}
    </section>
  )
}
