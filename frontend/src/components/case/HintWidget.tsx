import { useState } from 'react'
import { RiLightbulbFlashLine, RiLightbulbLine } from '@remixicon/react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useGetHintsQuery, useRevealHintMutation } from '@/api/cases'
import Typewriter from '@/components/game/Typewriter'
import { useGameStore } from '@/store/useGameStore'

/** `confirm` asks before using a hint; `notes` shows the hints already revealed. */
type Step = 'confirm' | 'notes'

/**
 * Hint button, stacked just above the chat button in the floating column CaseBriefing renders. Clicking it asks whether to
 * use the next hint; "Yes" calls the hint API, counts the hint as used and shows it in the
 * "Detective Notes" view, where every hint revealed so far stays readable without paying again.
 * Nothing is charged now: hint n takes n trophies off the reward if the case is solved.
 */
export default function HintWidget({ caseId }: { caseId: string }) {
  const { data: hintState } = useGetHintsQuery(caseId)
  const local = useGameStore((s) => s.hints[caseId])
  const addHint = useGameStore((s) => s.addHint)
  const [open, setOpen] = useState(false)
  const [step, setStep] = useState<Step>('confirm')
  const [justRevealed, setJustRevealed] = useState<number | null>(null)
  // The server's list is the truth after a reload; this session's list can be a step ahead of a refetch.
  const server = hintState?.hints ?? []
  const unlocked = local && local.length > server.length ? local : server

  const reveal = useRevealHintMutation({
    onSuccess: (h) => {
      addHint(caseId, h.number, h.hint)
      setJustRevealed(h.number)
      setStep('notes')
    },
    onError: (e) => toast.error(e.message),
  })

  const total = hintState?.hints_total ?? 0
  const next = unlocked.length + 1
  const remaining = total - unlocked.length
  if (!hintState) return null

  const openModal = () => {
    setJustRevealed(null)
    // asking comes first whenever there is a hint to use; otherwise there is only the notes to read
    setStep(remaining > 0 ? 'confirm' : 'notes')
    setOpen(true)
  }

  return (
    <>
      {/* not fixed itself: CaseBriefing stacks this above the chat button in one column */}
      <div className="relative shrink-0">
        <Button size="icon-lg" onClick={openModal} aria-label="Hint" className="size-14 shadow-[var(--glow-gold-strong)]">
          <RiLightbulbLine className="size-7" aria-hidden />
        </Button>
        {unlocked.length > 0 && (
          <span className="pointer-events-none absolute -top-1 -right-1 flex size-5 items-center justify-center bg-destructive font-heading text-xs text-white">
            {unlocked.length}
          </span>
        )}
      </div>

      <Dialog open={open} onOpenChange={(next) => !reveal.isPending && setOpen(next)}>
        <DialogContent className="max-h-[90vh] gap-5 overflow-y-auto border border-primary/30 bg-card p-6 shadow-[var(--glow-gold)] ring-0 sm:max-w-xl">
          {step === 'confirm' && remaining > 0 ? (
            <>
              <DialogHeader>
                <DialogTitle className="font-heading text-3xl text-primary">Reveal Hint #{next}?</DialogTitle>
                <DialogDescription className="font-typewriter text-muted-foreground">
                  Do you want to use a hint? This will permanently deduct {next} {next === 1 ? 'trophy' : 'trophies'} from your final
                  reward upon solving the case.
                </DialogDescription>
              </DialogHeader>
              {unlocked.length > 0 && (
                <Button variant="link" className="justify-self-start px-0 text-primary" onClick={() => setStep('notes')}>
                  View the {unlocked.length} hint{unlocked.length === 1 ? '' : 's'} you already revealed (free)
                </Button>
              )}
              <DialogFooter className="-mx-6 -mb-6 p-6">
                <Button variant="outline" disabled={reveal.isPending} onClick={() => setOpen(false)}>
                  No
                </Button>
                <Button disabled={reveal.isPending} onClick={() => reveal.mutate(caseId)}>
                  <RiLightbulbFlashLine aria-hidden />
                  {reveal.isPending ? 'Revealing...' : 'Yes, use hint'}
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <DialogHeader>
                <DialogTitle className="font-heading text-3xl text-primary">Detective Notes</DialogTitle>
                <DialogDescription className="font-typewriter text-muted-foreground">
                  {total === 0
                    ? 'The investigator left no hints for this case.'
                    : `${unlocked.length} of ${total} hints revealed.${remaining === 0 ? ' No hints remaining.' : ''}`}
                </DialogDescription>
              </DialogHeader>
              {unlocked.length > 0 && (
                <ol className="space-y-3">
                  {unlocked.map((hint, i) => (
                    <li key={i} className="border border-primary/25 bg-background/60 p-3 font-typewriter leading-relaxed text-card-foreground">
                      <span className="mb-1 block font-heading text-sm tracking-wide text-ember uppercase">Hint #{i + 1}</span>
                      {/* only the one just revealed types itself out */}
                      <Typewriter text={hint} animate={justRevealed === i + 1} />
                    </li>
                  ))}
                </ol>
              )}
              <DialogFooter className="-mx-6 -mb-6 p-6">
                <Button variant="outline" onClick={() => setOpen(false)}>
                  Close
                </Button>
                {remaining > 0 && <Button onClick={() => setStep('confirm')}>Use another hint</Button>}
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
