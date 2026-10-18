/** Stable pseudo-random number in [0, 1) from a string. */
function unit(seed: string, salt: number): number {
  let h = 2166136261 ^ salt
  for (let i = 0; i < seed.length; i++) h = Math.imul(h ^ seed.charCodeAt(i), 16777619)
  return ((h >>> 0) % 1000) / 1000
}

/**
 * Red push-pin on a case card; the red string ties to it. It sits at a random but
 * stable spot (derived from `seed`) on the top edge or on the left / right side,
 * always within the picture area at the top of the card so the yarn stays off the
 * text and the button. Parent must be `relative`; the card starts 8px below its top (`pt-2`).
 */
export default function Pin({ seed }: { seed: string }) {
  const edge = unit(seed, 3)
  const along = 0.12 + unit(seed, 1) * 0.76
  const style: React.CSSProperties =
    edge < 0.5
      ? { left: `${along * 100}%`, top: 8 + unit(seed, 2) * 6 } // top edge
      : edge < 0.75
        ? { left: 4, top: `${6 + unit(seed, 2) * 20}%` } // left side, upper part
        : { left: 'calc(100% - 4px)', top: `${6 + unit(seed, 2) * 20}%` } // right side, upper part
  return <span aria-hidden data-pin className="pin" style={style} />
}
