import { create } from 'zustand'
import { hasSession } from '@/utils/cookieHelpers'

interface SessionState {
  /** Mirrors "a session cookie exists" so the navbar re-renders on login / logout. */
  loggedIn: boolean
  signIn: () => void
  signOut: () => void
}

export const useSessionStore = create<SessionState>()((set) => ({
  loggedIn: hasSession(),
  signIn: () => set({ loggedIn: true }),
  signOut: () => set({ loggedIn: false }),
}))
