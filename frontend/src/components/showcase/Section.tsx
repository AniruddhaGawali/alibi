import type { ReactNode } from 'react'
import { Separator } from '@/components/ui/separator'

export default function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-4">
      <h2 className="text-xl text-primary">{title}</h2>
      <Separator />
      <div className="flex flex-wrap items-start gap-4">{children}</div>
    </section>
  )
}
