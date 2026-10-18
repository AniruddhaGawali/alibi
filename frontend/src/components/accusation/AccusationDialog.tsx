import { useState } from 'react'
import { RiAttachment2, RiFingerprintLine } from '@remixicon/react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useAccuseMutation, useGetSuspectsQuery, type AccusationOutcome } from '@/api/cases'
import { cn } from '@/lib/utils'
import { useGameStore } from '@/store/useGameStore'

interface Props {
  caseId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called once the server has settled the accusation, after the stores are updated. */
  onResolved?: (outcome: AccusationOutcome) => void
}

/**
 * The final deduction: every suspect of the case on a manila folder, pick one, confirm.
 * A settled accusation puts the outcome and the new trophy balance in the game store
 * (the case file stamps itself, the counters follow) and closes the dialog.
 */
export default function AccusationDialog({ caseId, open, onOpenChange, onResolved }: Props) {
  const { data: suspects, isPending, error } = useGetSuspectsQuery(caseId, { enabled: open })
  const [selected, setSelected] = useState<string | null>(null)
  const setTrophies = useGameStore((s) => s.setTrophies)
  const setOutcome = useGameStore((s) => s.setOutcome)

  const accuse = useAccuseMutation({
    onSuccess: (outcome) => {
      setTrophies(outcome.trophies)
      setOutcome(caseId, outcome)
      onOpenChange(false)
      onResolved?.(outcome)
    },
    onError: (e) => toast.error(e.message),
  })
  const busy = accuse.isPending

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent
        className="max-h-[90vh] gap-5 overflow-y-auto border border-primary/30 bg-card p-6 shadow-[var(--glow-red)] ring-0 sm:max-w-3xl"
        onInteractOutside={(e) => busy && e.preventDefault()}
      >
        <DialogHeader>
          <p className="font-typewriter text-xs tracking-[0.3em] text-ember uppercase">Final deduction</p>
          <DialogTitle className="font-heading text-3xl text-primary">Who did it?</DialogTitle>
          <DialogDescription className="font-typewriter text-muted-foreground">
            Name the culprit. This is final: the case closes either way, and a wrong name costs you trophies.
          </DialogDescription>
        </DialogHeader>

        {isPending ? (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-44" />
            ))}
          </div>
        ) : error ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load the suspects</AlertTitle>
            <AlertDescription>{error.message}</AlertDescription>
          </Alert>
        ) : (
          <div role="radiogroup" aria-label="Suspects" className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            {suspects.map((s) => {
              const on = s.id === selected
              return (
                <button
                  key={s.id}
                  type="button"
                  role="radio"
                  aria-checked={on}
                  disabled={busy}
                  onClick={() => setSelected(s.id)}
                  className={cn(
                    'group relative flex flex-col items-center gap-2 border-2 bg-background/60 p-3 text-center transition-colors',
                    on ? 'border-destructive bg-destructive/10 shadow-[var(--glow-red)]' : 'border-border hover:border-primary hover:shadow-[var(--glow-gold)]',
                  )}
                >
                  {on && <RiAttachment2 className="absolute -top-3 right-2 size-8 rotate-12 text-destructive" aria-hidden />}
                  <img
                    src={s.avatar_url}
                    alt=""
                    className={cn(
                      'aspect-square w-full max-w-32 border object-cover transition-[filter]',
                      on ? 'border-destructive' : 'border-primary/40 grayscale group-hover:grayscale-0',
                    )}
                  />
                  <span className={cn('font-heading text-lg leading-tight', on ? 'text-destructive' : 'text-foreground')}>{s.name}</span>
                  <span className="font-typewriter text-xs text-muted-foreground">{s.role}</span>
                </button>
              )
            })}
          </div>
        )}

        <DialogFooter className="-mx-6 -mb-6 p-6">
          {selected && (
            <Button
              size="lg"
              variant="destructive"
              disabled={busy}
              onClick={() => accuse.mutate({ caseId, suspectId: selected })}
            >
              <RiFingerprintLine aria-hidden />
              {busy ? 'Evaluating Evidence...' : 'Submit Deduction'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
