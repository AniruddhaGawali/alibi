import { useState } from 'react'
import { RiScalesLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import type { AccusationOutcome } from '@/api/cases'
import { cn } from '@/lib/utils'
import AccusationDialog from './AccusationDialog'

/** The prominent "Submit Deduction" button and the dialog it opens, for the case the player is in. */
export default function SubmitDeductionButton({
  caseId,
  onResolved,
  className,
}: {
  caseId: string
  onResolved?: (outcome: AccusationOutcome) => void
  className?: string
}) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button size="lg" className={cn('shadow-[var(--glow-gold-strong)]', className)} onClick={() => setOpen(true)}>
        <RiScalesLine aria-hidden /> Submit Deduction
      </Button>
      <AccusationDialog caseId={caseId} open={open} onOpenChange={setOpen} onResolved={onResolved} />
    </>
  )
}
