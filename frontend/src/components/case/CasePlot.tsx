import PaperClip from '@/components/game/PaperClip'

/**
 * The main plot, as the top sheet of an old case file clipped to a few more pages.
 * The sheet is letter-shaped (portrait, 3:4) on desktop and simply as tall as its
 * text on mobile. A longer story scrolls inside the sheet without a visible scrollbar.
 */
export default function CasePlot({ plot, synopsis }: { plot: string; synopsis: string }) {
  const paragraphs = (plot || synopsis).split(/\n{2,}/).filter(Boolean)
  return (
    <div className="case-folder relative mx-auto my-2 w-full max-w-md lg:aspect-[3/4] lg:max-w-none lg:self-center">
      <PaperClip />
      <div className="case-sheet no-scrollbar space-y-3 p-7 pt-14 font-typewriter text-sm leading-relaxed lg:absolute lg:inset-0 lg:overflow-y-auto">
        <h2 className="border-b-2 border-dashed border-[#5c2c10]/50 pb-2 font-typewriter text-lg font-bold tracking-widest uppercase">
          Case file · Confidential
        </h2>
        {paragraphs.map((p, i) => (
          <p key={i}>{p}</p>
        ))}
      </div>
    </div>
  )
}
