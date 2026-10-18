import { create } from 'zustand'

interface GameState {
  /**
   * The player's question tokens, the one value every counter and the chat input read.
   * null until the server has told us. `useSyncTokenBalance` keeps it in step with the
   * server; the chat changes it at once when a question is sent (optimistic).
   */
  tokenBalance: number | null
  activeCaseId: string | null
  /** Suspect whose interrogation logs the case page shows; null = everyone's. */
  selectedSuspectFilter: string | null
  setTokenBalance: (balance: number | null) => void
  /** Add (or take away) tokens locally; never goes below zero. No-op while the balance is unknown. */
  adjustTokenBalance: (delta: number) => void
  setActiveCaseId: (caseId: string | null) => void
  setSuspectFilter: (suspectId: string | null) => void
}

export const useGameStore = create<GameState>()((set) => ({
  tokenBalance: null,
  activeCaseId: null,
  selectedSuspectFilter: null,
  setTokenBalance: (tokenBalance) => set({ tokenBalance }),
  adjustTokenBalance: (delta) =>
    set((s) => (s.tokenBalance === null ? s : { tokenBalance: Math.max(0, s.tokenBalance + delta) })),
  setActiveCaseId: (activeCaseId) => set({ activeCaseId }),
  setSuspectFilter: (selectedSuspectFilter) => set({ selectedSuspectFilter }),
}))
