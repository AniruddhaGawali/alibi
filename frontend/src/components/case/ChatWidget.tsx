import { useEffect } from 'react'
import { RiChat3Line } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useGetSuspectsQuery } from '@/api/cases'
import { useChatStore } from '@/store/useChatStore'

/**
 * Floating chat button (bottom right) and the interrogation window it opens. The
 * open state and the selected suspect live in the chat store, so a suspect card's
 * Chat button opens this with that suspect already chosen. The message list itself
 * arrives in Phase 3.3.
 */
export default function ChatWidget({ caseId }: { caseId: string }) {
  const { data: suspects } = useGetSuspectsQuery(caseId)
  const { isChatOpen, activeChatSuspectId, openChat, closeChat, setActiveChatSuspect, reset } = useChatStore()

  // leaving the case page closes the chat and forgets the suspect
  useEffect(() => reset, [reset])

  const active = suspects?.find((s) => s.id === activeChatSuspectId) ?? suspects?.[0]

  return (
    <>
      {/* the wrapper is what is fixed: the theme gives buttons their own position */}
      <div className="fixed right-6 bottom-6 z-40">
        <Button size="icon-lg" onClick={() => openChat()} aria-label="Open chat" className="size-14 shadow-[var(--glow-gold-strong)]">
          <RiChat3Line className="size-7" aria-hidden />
        </Button>
      </div>

      <Sheet open={isChatOpen} onOpenChange={(open) => (open ? openChat() : closeChat())}>
        <SheetContent side="right" className="w-full gap-0 sm:max-w-md">
          <SheetHeader className="border-b">
            <SheetTitle className="font-heading text-xl">Interrogation</SheetTitle>
            <SheetDescription>Choose who to question.</SheetDescription>
          </SheetHeader>

          {suspects && active && (
            <div className="space-y-4 p-4">
              <Tabs value={active.id} onValueChange={setActiveChatSuspect}>
                <TabsList className="h-auto w-full flex-wrap">
                  {suspects.map((s) => (
                    <TabsTrigger key={s.id} value={s.id}>
                      {s.name}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>

              <div className="flex items-center gap-3">
                <span className="frame-portrait m-1">
                  <img src={active.avatar_url} alt="" className="size-24 object-cover" />
                </span>
                <div>
                  <p className="font-heading text-lg">{active.name}</p>
                  <p className="text-sm text-muted-foreground">{active.role}</p>
                </div>
              </div>
              <p className="border-l-2 border-primary/60 pl-3 font-typewriter text-sm text-muted-foreground">
                “{active.initial_testimony}”
              </p>
              <p className="border border-dashed p-4 text-center text-sm text-muted-foreground">
                The conversation with {active.name} will appear here.
              </p>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </>
  )
}
