import { create } from 'zustand'

interface GameState {
  tokenBalance: number
  activeCaseId: string | null
  setTokenBalance: (balance: number) => void
  setActiveCaseId: (caseId: string | null) => void
}

export const useGameStore = create<GameState>()((set) => ({
  tokenBalance: 0,
  activeCaseId: null,
  setTokenBalance: (tokenBalance) => set({ tokenBalance }),
  setActiveCaseId: (activeCaseId) => set({ activeCaseId }),
}))
