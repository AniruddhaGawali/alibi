import { Separator } from '@/components/ui/separator'

export default function OrDivider() {
  return (
    <div className="flex items-center gap-3 text-xs uppercase text-muted-foreground">
      <Separator className="flex-1" />
      or
      <Separator className="flex-1" />
    </div>
  )
}
