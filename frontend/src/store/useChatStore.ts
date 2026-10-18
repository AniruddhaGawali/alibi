import { create } from 'zustand'

interface ChatState {
  isChatOpen: boolean
  /** The suspect being interrogated; null until one is picked. */
  activeChatSuspectId: string | null
  /** Open the window, optionally with a suspect already selected (suspect cards pass theirs). */
  openChat: (suspectId?: string) => void
  closeChat: () => void
  setActiveChatSuspect: (suspectId: string) => void
  reset: () => void
}

export const useChatStore = create<ChatState>()((set) => ({
  isChatOpen: false,
  activeChatSuspectId: null,
  openChat: (suspectId) =>
    set((s) => ({ isChatOpen: true, activeChatSuspectId: suspectId ?? s.activeChatSuspectId })),
  closeChat: () => set({ isChatOpen: false }),
  setActiveChatSuspect: (activeChatSuspectId) => set({ activeChatSuspectId }),
  reset: () => set({ isChatOpen: false, activeChatSuspectId: null }),
}))
