import type { ComponentType, ReactNode } from 'react'
import { RiCheckLine, RiCloseLine } from '@remixicon/react'
import Reveal from '@/components/landing/Reveal'

/** One rules chapter: a numbered, framed sheet with an icon and a title. */
export function RuleSection({
  id,
  icon: Icon,
  title,
  children,
}: {
  id: string
  icon: ComponentType<{ className?: string }>
  title: string
  children: ReactNode
}) {
  return (
    <Reveal>
      <section id={id} className="frame scroll-mt-8 space-y-4 border bg-card p-6 sm:p-8">
        <h2 className="flex items-center gap-3 text-2xl text-primary sm:text-3xl">
          <Icon className="size-7 shrink-0 text-gold" aria-hidden />
          {title}
        </h2>
        <div className="space-y-3 text-muted-foreground [&_strong]:text-foreground">{children}</div>
      </section>
    </Reveal>
  )
}

/** A do / don't list. */
export function Checklist({ items, kind }: { items: string[]; kind: 'do' | 'dont' }) {
  const Icon = kind === 'do' ? RiCheckLine : RiCloseLine
  return (
    <ul className="space-y-2">
      {items.map((t) => (
        <li key={t} className="flex gap-2">
          <Icon className={kind === 'do' ? 'mt-0.5 size-5 shrink-0 text-gold' : 'mt-0.5 size-5 shrink-0 text-destructive'} aria-hidden />
          <span>{t}</span>
        </li>
      ))}
    </ul>
  )
}
