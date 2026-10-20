import { useState } from 'react'
import { RiImageLine } from '@remixicon/react'
import { assetUrl } from '@/utils/assets'

/**
 * A screenshot for the handbook, from the asset bucket at guide/<file>. Until the file is
 * uploaded (or if it fails to load) a framed placeholder with the caption is shown instead.
 */
export default function GuideScreenshot({ file, alt, caption }: { file: string; alt: string; caption: string }) {
  const [missing, setMissing] = useState(false)
  return (
    <figure className="space-y-2">
      {missing ? (
        <div className="flex aspect-video w-full flex-col items-center justify-center gap-2 border border-dashed bg-background/40 p-4 text-center">
          <RiImageLine className="size-8 text-gold/60" aria-hidden />
          <span className="text-sm">{alt}</span>
        </div>
      ) : (
        <img
          src={assetUrl(`guide/${file}`)}
          alt={alt}
          loading="lazy"
          // a prerendered <img> can fail before React hydrates, when onError is not listening yet
          ref={(img) => {
            if (img?.complete && img.naturalWidth === 0) setMissing(true)
          }}
          onError={() => setMissing(true)}
          className="w-full border border-primary/40 shadow-[var(--glow-gold)]"
        />
      )}
      <figcaption className="font-typewriter text-xs text-muted-foreground">{caption}</figcaption>
    </figure>
  )
}
