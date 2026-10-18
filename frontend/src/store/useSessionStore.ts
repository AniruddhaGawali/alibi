import { create } from 'zustand'
import { getJwt } from '@/utils/cookieHelpers'

interface SessionState {
  /** Mirrors "a JWT cookie exists" so the navbar re-renders on login / logout. */
  loggedIn: boolean
  signIn: () => void
  signOut: () => void
}

export const useSessionStore = create<SessionState>()((set) => ({
  loggedIn: getJwt() !== null,
  signIn: () => set({ loggedIn: true }),
  signOut: () => set({ loggedIn: false }),
}))
