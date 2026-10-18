import { create } from 'zustand'
import type { AccusationOutcome } from '@/api/cases'

interface GameState {
  /**
   * The player's question tokens, the one value every counter and the chat input read.
   * null until the server has told us. `useSyncTokenBalance` keeps it in step with the
   * server; the chat changes it at once when a question is sent (optimistic).
   */
  tokenBalance: number | null
  /** The player's trophies; null until known. Set from the server, and by a settled accusation. */
  trophies: number | null
  /** Settled accusations by case (story) id, so the case file can show its stamp. */
  outcomes: Record<string, AccusationOutcome>
  /** Hints revealed this session, by case id, in order (the briefing also carries them after a reload). */
  hints: Record<string, string[]>
  activeCaseId: string | null
  /** Suspect whose interrogation logs the case page shows; null = everyone's. */
  selectedSuspectFilter: string | null
  setTokenBalance: (balance: number | null) => void
  setTrophies: (trophies: number | null) => void
  setOutcome: (caseId: string, outcome: AccusationOutcome) => void
  /** Puts a revealed hint at its place (number is 1-based). */
  addHint: (caseId: string, number: number, hint: string) => void
  /** Add (or take away) tokens locally; never goes below zero. No-op while the balance is unknown. */
  adjustTokenBalance: (delta: number) => void
  setActiveCaseId: (caseId: string | null) => void
  setSuspectFilter: (suspectId: string | null) => void
}

export const useGameStore = create<GameState>()((set) => ({
  tokenBalance: null,
  trophies: null,
  outcomes: {},
  hints: {},
  activeCaseId: null,
  selectedSuspectFilter: null,
  setTokenBalance: (tokenBalance) => set({ tokenBalance }),
  setTrophies: (trophies) => set({ trophies }),
  addHint: (caseId, number, hint) =>
    set((s) => {
      const list = [...(s.hints[caseId] ?? [])]
      list[number - 1] = hint
      return { hints: { ...s.hints, [caseId]: list } }
    }),
  setOutcome: (caseId, outcome) => set((s) => ({ outcomes: { ...s.outcomes, [caseId]: outcome } })),
  adjustTokenBalance: (delta) =>
    set((s) => (s.tokenBalance === null ? s : { tokenBalance: Math.max(0, s.tokenBalance + delta) })),
  setActiveCaseId: (activeCaseId) => set({ activeCaseId }),
  setSuspectFilter: (selectedSuspectFilter) => set({ selectedSuspectFilter }),
}))
