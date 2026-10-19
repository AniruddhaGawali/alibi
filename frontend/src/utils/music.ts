import { assetUrl } from './assets'

/**
 * Background music. The files are static, in the asset bucket at music/<file>
 * (uploaded by hand, like the seeded art). main.ogg plays everywhere unless the
 * player is inside a case whose admin picked one of CASE_TRACKS. Keep the keys in
 * step with admin.MusicTracks on the backend.
 */
export const MAIN_TRACK_FILE = 'main.ogg'

export const CASE_TRACKS = [
  { key: 'detective-caution', label: 'Detective Caution', file: 'detective-caution.ogg' },
  { key: 'bloodmoney', label: 'Blood Money', file: 'bloodmoney.ogg' },
  { key: 'too-crazy', label: 'Too Crazy', file: 'too-crazy.ogg' },
  { key: 'solving-the-crime', label: 'Solving the Crime', file: 'solving-the-crime.ogg' },
] as const

export type CaseTrackKey = (typeof CASE_TRACKS)[number]['key']

/** URL of a case's track; '' or an unknown key (e.g. removed since) is the main theme. */
export function musicUrl(track: string | null | undefined): string {
  const file = CASE_TRACKS.find((t) => t.key === track)?.file ?? MAIN_TRACK_FILE
  return assetUrl(`music/${file}`)
}

/**
 * The tracks never change, so each one is kept in the browser's Cache Storage after its first
 * play and every later visit plays the local copy: no download, whatever caching headers the
 * bucket sends. If a track is ever replaced, change the cache name (v1 -> v2) so browsers
 * fetch it again.
 */
const MUSIC_CACHE = 'alibi-music-v1'
const blobUrls = new Map<string, string>() // network URL -> object URL of the cached copy
const saving = new Set<string>()

/**
 * What to play for a track: the cached copy when there is one, else the network URL, and a
 * copy is then saved in the background for next time. Any failure (no Cache Storage, a
 * private window, no CORS) just means playing from the network.
 */
async function trackSource(url: string): Promise<string> {
  const known = blobUrls.get(url)
  if (known) return known
  if (typeof caches === 'undefined') return url
  try {
    const cache = await caches.open(MUSIC_CACHE)
    const hit = await cache.match(url)
    if (hit) {
      const objectUrl = URL.createObjectURL(await hit.blob())
      blobUrls.set(url, objectUrl)
      return objectUrl
    }
    if (!saving.has(url)) {
      saving.add(url)
      void fetch(url, { mode: 'cors' })
        .then((res) => (res.ok ? cache.put(url, res) : undefined))
        .catch(() => undefined)
        .finally(() => saving.delete(url))
    }
  } catch {
    // fall back to the network
  }
  return url
}

const FADE_MS = 900
const FADE_STEPS = 18

/** Fades el's volume to `to`, then calls done. Returns a function that cancels it. */
function fade(el: HTMLAudioElement, to: number, done?: () => void): () => void {
  const from = el.volume
  let step = 0
  const id = window.setInterval(() => {
    step++
    el.volume = Math.min(1, Math.max(0, from + ((to - from) * step) / FADE_STEPS))
    if (step >= FADE_STEPS) {
      window.clearInterval(id)
      done?.()
    }
  }, FADE_MS / FADE_STEPS)
  return () => window.clearInterval(id)
}

/**
 * One looping track at a time, crossfaded on change. Browsers refuse to start audio
 * before the visitor has interacted with the page, so a refused start is retried on
 * the first click, tap or key press.
 */
class MusicController {
  private current?: HTMLAudioElement
  private url?: string
  private muted = false
  private volume = 0.35 // set from the player's setting by setVolume before anything plays
  private cancelFade?: () => void
  private waitingForGesture = false

  /** Switch to the track at url (a no-op when it is already the one playing). */
  play(url: string) {
    if (url === this.url) return
    this.url = url
    const old = this.current
    if (old) fade(old, 0, () => old.pause())

    const next = new Audio()
    next.loop = true
    next.preload = 'auto'
    next.volume = 0
    this.current = next
    // the source is looked up in the local cache first, so the track may start a moment later
    void trackSource(url).then((src) => {
      if (this.current !== next) return // another track was chosen meanwhile
      next.src = src
      if (!this.muted) this.start(next)
    })
  }

  /** Set the loudness (0-1) of the playing track and of those that follow. */
  setVolume(volume: number) {
    this.volume = Math.min(1, Math.max(0, volume))
    const el = this.current
    if (!el || this.muted) return
    // Follow the slider at once rather than fading, so dragging feels direct.
    this.cancelFade?.()
    el.volume = this.volume
  }

  setMuted(muted: boolean) {
    this.muted = muted
    const el = this.current
    if (!el) return
    this.cancelFade?.()
    if (muted) {
      this.cancelFade = fade(el, 0, () => el.pause())
    } else {
      this.start(el)
    }
  }

  private start(el: HTMLAudioElement) {
    el.play().then(
      () => {
        if (el !== this.current || this.muted) return
        this.cancelFade?.()
        this.cancelFade = fade(el, this.volume)
      },
      (err: unknown) => {
        // NotAllowedError: no user gesture yet. Anything else (missing file, network)
        // just leaves the game silent.
        if (err instanceof DOMException && err.name === 'NotAllowedError') this.retryOnGesture()
      },
    )
  }

  private retryOnGesture() {
    if (this.waitingForGesture) return
    this.waitingForGesture = true
    const resume = () => {
      this.waitingForGesture = false
      window.removeEventListener('pointerdown', resume)
      window.removeEventListener('keydown', resume)
      if (this.current && !this.muted) this.start(this.current)
    }
    window.addEventListener('pointerdown', resume)
    window.addEventListener('keydown', resume)
  }
}

export const music = new MusicController()
