import { create } from 'zustand'

const KEY = 'alibi_muted'

function readMuted(): boolean {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

interface SoundState {
  muted: boolean
  toggleMuted: () => void
}

/** Sound on/off preference, remembered per browser. */
export const useSoundStore = create<SoundState>()((set, get) => ({
  muted: readMuted(),
  toggleMuted: () => {
    const muted = !get().muted
    try {
      localStorage.setItem(KEY, muted ? '1' : '0')
    } catch {
      /* private mode: the preference just won't persist */
    }
    set({ muted })
  },
}))
