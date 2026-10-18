import { useEffect, useRef, useState } from 'react'

interface Pt {
  x: number
  y: number
  /** Index of the grid row the pin's card is in. */
  row: number
}

/**
 * Push-pin heads (`[data-pin]`) inside the board, relative to it, in the order the
 * yarn is strung: left to right along a row, then back right to left along the
 * next row, and so on, so the string drops to the card directly below the end of a row.
 */
function readPins(board: HTMLElement): Pt[] {
  const origin = board.getBoundingClientRect()
  const pins = Array.from(board.querySelectorAll<HTMLElement>('[data-pin]')).map((el) => {
    const card = el.parentElement!.getBoundingClientRect()
    const r = el.getBoundingClientRect()
    return {
      x: r.left - origin.left + r.width / 2,
      y: r.top - origin.top + r.height / 2,
      top: card.top - origin.top,
      left: card.left - origin.left,
    }
  })
  // cards in one grid row share a top (within a few pixels)
  const tops: number[] = []
  const withRow = pins.map((p) => {
    let row = tops.findIndex((t) => Math.abs(t - p.top) < 30)
    if (row < 0) row = tops.push(p.top) - 1
    return { ...p, row }
  })
  const order = tops.map((t, i) => [t, i]).sort((a, b) => a[0] - b[0]).map(([, i]) => i)
  return order.flatMap((rowId, i) => {
    const row = withRow.filter((p) => p.row === rowId).sort((a, b) => a.left - b.left)
    return (i % 2 === 0 ? row : row.reverse()).map(({ x, y }) => ({ x, y, row: i }))
  })
}

/**
 * Yarn between two pins. Within a row it hangs slack and a little messy; between
 * rows it swings out around the end of the row (right edge, then left edge,
 * alternating).
 */
function strand(a: Pt, b: Pt, width: number, i: number): string {
  if (a.row !== b.row) {
    // swing out past the board edge, run down the margin, then come back in at the
    // next pin's height, so the yarn stays off the lower part of the cards
    const right = (a.x + b.x) / 2 > width / 2
    const out = (right ? width + 14 : -14) + (right ? 1 : -1) * Math.sin(i * 7.7) * 5
    const run = Math.min(40, Math.abs(b.y - a.y) / 3)
    return (
      `M ${a.x} ${a.y} Q ${out} ${a.y} ${out} ${a.y + run} ` +
      `C ${out + (right ? 3 : -3)} ${(a.y + b.y) / 2} ${out - (right ? 3 : -3)} ${(a.y + b.y) / 2 + 20} ${out} ${b.y - run} ` +
      `Q ${out} ${b.y} ${b.x} ${b.y}`
    )
  }
  // a cubic whose control points are pushed down by a random amount and nudged
  // sideways, different for every strand (stable per index)
  const rnd = (k: number) => {
    const v = Math.sin((i + 1) * 12.9898 + k * 78.233) * 43758.5453
    return v - Math.floor(v)
  }
  const dx = b.x - a.x
  const dy = b.y - a.y
  const dist = Math.hypot(dx, dy) || 1
  const slack = 14 + rnd(1) * 30 + dist * 0.03
  const nx = -dy / dist
  const ny = dx / dist
  const w1 = (rnd(2) - 0.5) * 36
  const w2 = (rnd(3) - 0.5) * 36
  return (
    `M ${a.x} ${a.y} C ${a.x + dx * 0.28 + nx * w1} ${a.y + dy * 0.28 + ny * w1 + slack} ` +
    `${a.x + dx * 0.72 + nx * w2} ${a.y + dy * 0.72 + ny * w2 + slack * (0.5 + rnd(4))} ${b.x} ${b.y}`
  )
}

/**
 * Red yarn strung across the case board, tied to the pin of every case card in a
 * snaking order, drawn over the cards. Place it inside the (relative) board element
 * that holds the cards.
 */
export default function RedString() {
  const ref = useRef<SVGSVGElement>(null)
  const [geo, setGeo] = useState<{ w: number; h: number; pts: Pt[] } | null>(null)

  useEffect(() => {
    const board = ref.current?.parentElement
    if (!board) return
    const measure = () => {
      const { width, height } = board.getBoundingClientRect()
      setGeo({ w: width, h: height, pts: readPins(board) })
    }
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(board)
    return () => ro.disconnect()
  }, [])

  const d = geo ? geo.pts.slice(1).map((p, i) => strand(geo.pts[i], p, geo.w, i)) : []

  return (
    <svg
      ref={ref}
      className="red-string hidden sm:block"
      width={geo?.w ?? 0}
      height={geo?.h ?? 0}
      viewBox={`0 0 ${geo?.w ?? 0} ${geo?.h ?? 0}`}
      aria-hidden
    >
      {d.map((path, i) => (
        <g key={i}>
          <path d={path} fill="none" stroke="#000" strokeOpacity=".4" strokeWidth="5" strokeLinecap="round" transform="translate(2 5)" style={{ filter: 'blur(2px)' }} />
          <path d={path} fill="none" stroke="#4a0a0a" strokeWidth="5" strokeLinecap="round" />
          <path d={path} fill="none" stroke="#c4221f" strokeWidth="3.6" strokeLinecap="round" />
          <path d={path} fill="none" stroke="#ff8a78" strokeOpacity=".55" strokeWidth="3.6" strokeDasharray="1.2 3.4" />
          <path d={path} fill="none" stroke="#7a1010" strokeOpacity=".6" strokeWidth="3.6" strokeDasharray="1 3.6" strokeDashoffset="2.4" />
          <path d={path} fill="none" stroke="#ffd0c4" strokeOpacity=".35" strokeWidth="1" transform="translate(0 -1.2)" />
        </g>
      ))}
    </svg>
  )
}
