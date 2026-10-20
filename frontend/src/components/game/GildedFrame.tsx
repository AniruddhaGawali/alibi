import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import cornerTl from '@/assets/frame/gallery-corner-tl.svg'
import cornerTr from '@/assets/frame/gallery-corner-tr.svg'
import cornerBl from '@/assets/frame/gallery-corner-bl.svg'
import cornerBr from '@/assets/frame/gallery-corner-br.svg'
import crest from '@/assets/frame/gallery-crest.svg'
import base from '@/assets/frame/gallery-base.svg'
import { cn } from '@/lib/utils'

interface Pt {
  x: number
  y: number
}

const THICKNESS = 34 // width of the moulding at full size; the picture sits inside it
const FULL_WIDTH = 640 // box width at and above which the frame is drawn at full size
const MIN_SCALE = 0.55

/**
 * A point on the outline of a w x h box pulled in by `inset`, with a rococo
 * ripple of amplitude `amp` and a swelling at each corner of size `bump`.
 */
function outline(w: number, h: number, inset: number, amp: number, bump: number, phase: number, k: number): Pt[] {
  const out: Pt[] = []
  const step = 7
  const wave = (s: number, len: number) => {
    const g = (d: number) => Math.exp(-((d / (38 * k)) ** 2))
    return k * (amp * Math.sin((2 * Math.PI * s) / (64 * k) + phase) + bump * (g(s) + g(len - s)))
  }
  // top, right, bottom, left; every edge runs clockwise and `o` pushes outwards
  for (let s = 0; s < w; s += step) out.push({ x: s, y: inset - wave(s, w) })
  for (let s = 0; s < h; s += step) out.push({ x: w - inset + wave(s, h), y: s })
  for (let s = 0; s < w; s += step) out.push({ x: w - s, y: h - inset + wave(s, w) })
  for (let s = 0; s < h; s += step) out.push({ x: inset - wave(s, h), y: h - s })
  return out
}

/** Closed Catmull-Rom spline through the points as an SVG path. */
function closedPath(pts: Pt[]): string {
  const n = pts.length
  let d = `M ${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`
  for (let i = 0; i < n; i++) {
    const p0 = pts[(i - 1 + n) % n]
    const p1 = pts[i]
    const p2 = pts[(i + 1) % n]
    const p3 = pts[(i + 2) % n]
    d += ` C ${(p1.x + (p2.x - p0.x) / 6).toFixed(1)} ${(p1.y + (p2.y - p0.y) / 6).toFixed(1)} ${(p2.x - (p3.x - p1.x) / 6).toFixed(1)} ${(p2.y - (p3.y - p1.y) / 6).toFixed(1)} ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }
  return d + ' Z'
}

/** The picture opening: a rectangle with generously rounded corners. */
function opening(w: number, h: number, t: number, r: number): string {
  const x0 = t
  const y0 = t
  const x1 = w - t
  const y1 = h - t
  return (
    `M ${x0 + r} ${y0} L ${x1 - r} ${y0} Q ${x1 - r * 0.25} ${y0 + r * 0.25} ${x1} ${y0 + r} ` +
    `L ${x1} ${y1 - r} Q ${x1 - r * 0.25} ${y1 - r * 0.25} ${x1 - r} ${y1} ` +
    `L ${x0 + r} ${y1} Q ${x0 + r * 0.25} ${y1 - r * 0.25} ${x0} ${y1 - r} ` +
    `L ${x0} ${y0 + r} Q ${x0 + r * 0.25} ${y0 + r * 0.25} ${x0 + r} ${y0} Z`
  )
}

/**
 * A carved, gilded rococo picture frame around its children: a wavy moulding with
 * swollen corners, carved ripple lines, a rounded opening, shell corners and a crest.
 * Drawn as SVG at the size of the box, so it fits any picture. It overhangs the box
 * by about 30px: give it margin.
 */
export default function GildedFrame({ children, className }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState({ w: 0, h: 0 })
  const id = useId().replace(/:/g, '')

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const update = () => setSize({ w: el.offsetWidth, h: el.offsetHeight })
    update()
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const { w, h } = size
  // narrow boxes get a proportionally slimmer moulding and smaller ornaments
  const k = w > 0 ? Math.min(1, Math.max(MIN_SCALE, w / FULL_WIDTH)) : 1
  const t = THICKNESS * k
  const ready = w > 0 && h > 0
  const outer = ready ? closedPath(outline(w, h, 4 * k, 4, 16, 0, k)) : ''
  const carveA = ready ? closedPath(outline(w, h, 13 * k, 3, 10, 1.4, k)) : ''
  const carveB = ready ? closedPath(outline(w, h, 22 * k, 2.4, 5, 3, k)) : ''
  const hole = ready ? opening(w, h, t, 22 * k) : ''
  const holeLip = ready ? opening(w, h, t - 5 * k, 25 * k) : ''

  return (
    <div ref={ref} className={cn('relative', className)} style={{ padding: t }}>
      {children}
      {ready && (
        <svg
          className="pointer-events-none absolute inset-0 z-10 overflow-visible"
          width={w}
          height={h}
          viewBox={`0 0 ${w} ${h}`}
          aria-hidden
        >
          <defs>
            <linearGradient id={`${id}g`} x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stopColor="#fff0a0" />
              <stop offset=".22" stopColor="#e9b947" />
              <stop offset=".5" stopColor="#c4902e" />
              <stop offset=".78" stopColor="#f0cb64" />
              <stop offset="1" stopColor="#a8741f" />
            </linearGradient>
            <filter id={`${id}t`} x="0" y="0" width="100%" height="100%">
              {/* worn gilding: low-frequency mottling plus fine grain */}
              <feTurbulence type="fractalNoise" baseFrequency="0.035 0.05" numOctaves="3" seed="7" result="n" />
              <feColorMatrix in="n" type="matrix" values="0 0 0 0 .35  0 0 0 0 .2  0 0 0 0 .05  0 0 0 .9 -.28" result="stain" />
              <feComposite in="stain" in2="SourceGraphic" operator="in" />
            </filter>
            <filter id={`${id}s`} x="-10%" y="-10%" width="120%" height="120%">
              <feDropShadow dx="0" dy="6" stdDeviation="7" floodColor="#000" floodOpacity=".75" />
            </filter>
          </defs>

          <g filter={`url(#${id}s)`}>
            <path d={`${outer} ${hole}`} fillRule="evenodd" fill={`url(#${id}g)`} stroke="#3a2205" strokeWidth="1.6" />
          </g>
          {/* gilt wear */}
          <path d={`${outer} ${hole}`} fillRule="evenodd" fill="#000" filter={`url(#${id}t)`} opacity=".55" />
          {/* carved ripples, dark groove with a bright edge beside it */}
          {[carveA, carveB].map((d, i) => (
            <g key={i} fill="none">
              <path d={d} stroke="#3a2205" strokeOpacity=".75" strokeWidth="2.2" />
              <path d={d} stroke="#fff3b0" strokeOpacity=".65" strokeWidth="1" transform="translate(-1 -1)" />
            </g>
          ))}
          {/* raised lip around the opening */}
          <path d={holeLip} fill="none" stroke="#3a2205" strokeOpacity=".8" strokeWidth="2" />
          <path d={holeLip} fill="none" stroke="#fff0a0" strokeOpacity=".7" strokeWidth="1" transform="translate(-1 -1)" />
          <path d={hole} fill="none" stroke="#2a1704" strokeWidth="3" />
          <path d={hole} fill="none" stroke="#f0cb64" strokeWidth="1.2" strokeOpacity=".9" />

          {/* shell corners, crest and drop */}
          <g style={{ filter: 'drop-shadow(0 2px 3px rgb(0 0 0 / 65%))' }}>
            <image href={cornerTl} x={-30 * k} y={-30 * k} width={92 * k} height={92 * k} />
            <image href={cornerTr} x={w - 62 * k} y={-30 * k} width={92 * k} height={92 * k} />
            <image href={cornerBl} x={-30 * k} y={h - 62 * k} width={92 * k} height={92 * k} />
            <image href={cornerBr} x={w - 62 * k} y={h - 62 * k} width={92 * k} height={92 * k} />
            <image href={crest} x={w / 2 - 95 * k} y={-34 * k} width={190 * k} height={72 * k} />
            <image href={base} x={w / 2 - 55 * k} y={h - 12 * k} width={110 * k} height={37 * k} />
          </g>
        </svg>
      )}
    </div>
  )
}
