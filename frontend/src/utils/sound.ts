import { useSoundStore } from '@/store/useSoundStore'

/**
 * Sound effects synthesised with the Web Audio API, so the game needs no audio
 * files. To use recorded sounds later, replace the bodies here and keep `play`.
 */
export type SoundName = 'click' | 'key' | 'paper' | 'stamp' | 'error'

let ctx: AudioContext | null = null

function context(): AudioContext | null {
  if (typeof window === 'undefined' || !('AudioContext' in window)) return null
  ctx ??= new AudioContext()
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

function noise(c: AudioContext, seconds: number): AudioBufferSourceNode {
  const buf = c.createBuffer(1, Math.ceil(c.sampleRate * seconds), c.sampleRate)
  const data = buf.getChannelData(0)
  for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1
  const src = c.createBufferSource()
  src.buffer = buf
  return src
}

function tone(c: AudioContext, type: OscillatorType, from: number, to: number, seconds: number, gain: number) {
  const t = c.currentTime
  const osc = c.createOscillator()
  const g = c.createGain()
  osc.type = type
  osc.frequency.setValueAtTime(from, t)
  osc.frequency.exponentialRampToValueAtTime(to, t + seconds)
  g.gain.setValueAtTime(gain, t)
  g.gain.exponentialRampToValueAtTime(0.0001, t + seconds)
  osc.connect(g).connect(c.destination)
  osc.start(t)
  osc.stop(t + seconds)
}

function burst(c: AudioContext, seconds: number, freq: number, q: number, gain: number) {
  const t = c.currentTime
  const src = noise(c, seconds)
  const filter = c.createBiquadFilter()
  filter.type = 'bandpass'
  filter.frequency.value = freq
  filter.Q.value = q
  const g = c.createGain()
  g.gain.setValueAtTime(gain, t)
  g.gain.exponentialRampToValueAtTime(0.0001, t + seconds)
  src.connect(filter).connect(g).connect(c.destination)
  src.start(t)
}

export function play(name: SoundName): void {
  if (useSoundStore.getState().muted) return
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches && name === 'key') return
  const c = context()
  if (!c) return
  switch (name) {
    case 'click':
      tone(c, 'square', 900, 300, 0.05, 0.05)
      break
    case 'key':
      burst(c, 0.03, 2500 + Math.random() * 800, 1.5, 0.12)
      tone(c, 'square', 180, 90, 0.03, 0.03)
      break
    case 'paper':
      burst(c, 0.28, 3500, 0.7, 0.18)
      break
    case 'stamp':
      tone(c, 'sine', 140, 40, 0.25, 0.5)
      burst(c, 0.08, 800, 0.8, 0.25)
      break
    case 'error':
      tone(c, 'sawtooth', 200, 110, 0.18, 0.06)
      break
  }
}
