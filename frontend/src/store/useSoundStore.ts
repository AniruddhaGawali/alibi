import { create } from 'zustand'

const KEY = 'alibi_muted'
const MUSIC_KEY = 'alibi_music_muted'
const VOLUME_KEY = 'alibi_music_volume'

/** Music volume until the player changes it (0-1). */
export const DEFAULT_MUSIC_VOLUME = 0.35

function readVolume(): number {
  try {
    const v = Number(localStorage.getItem(VOLUME_KEY))
    // Number(null) is 0: only a stored number counts.
    return localStorage.getItem(VOLUME_KEY) !== null && Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : DEFAULT_MUSIC_VOLUME
  } catch {
    return DEFAULT_MUSIC_VOLUME
  }
}

function readFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

function saveFlag(key: string, on: boolean) {
  try {
    localStorage.setItem(key, on ? '1' : '0')
  } catch {
    /* private mode: the preference just won't persist */
  }
}

interface SoundState {
  /** Sound effects off. */
  muted: boolean
  toggleMuted: () => void
  /** Background music off (separate, so effects can stay on without music). */
  musicMuted: boolean
  toggleMusicMuted: () => void
  /** Background music volume, 0-1. */
  musicVolume: number
  setMusicVolume: (volume: number) => void
}

/** Sound and music on/off preferences, remembered per browser. */
export const useSoundStore = create<SoundState>()((set, get) => ({
  muted: readFlag(KEY),
  toggleMuted: () => {
    const muted = !get().muted
    saveFlag(KEY, muted)
    set({ muted })
  },
  musicMuted: readFlag(MUSIC_KEY),
  toggleMusicMuted: () => {
    const musicMuted = !get().musicMuted
    saveFlag(MUSIC_KEY, musicMuted)
    set({ musicMuted })
  },
  musicVolume: readVolume(),
  setMusicVolume: (volume) => {
    const musicVolume = Math.min(1, Math.max(0, volume))
    try {
      localStorage.setItem(VOLUME_KEY, String(musicVolume))
    } catch {
      /* private mode: the preference just won't persist */
    }
    // Turning it up from zero is a request to hear it; zero is "off" without a second switch.
    set({ musicVolume, ...(musicVolume > 0 && get().musicMuted ? { musicMuted: false } : {}) })
    if (musicVolume > 0) saveFlag(MUSIC_KEY, false)
  },
}))
