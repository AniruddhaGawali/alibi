import { Link } from 'react-router'
import { RiFileSearchLine } from '@remixicon/react'
import { Button } from '@/components/ui/button'
import Stamp from '@/components/game/Stamp'

/** "Case not found" file with a MISSING stamp, for unknown pages and unknown case ids. */
export default function NotFound({ to = '/', label = 'Back to the office' }: { to?: string; label?: string }) {
  return (
    <div className="mx-auto flex max-w-md flex-col items-center gap-4 py-16 text-center">
      <div className="frame case-sheet relative w-full space-y-3 p-8">
        <RiFileSearchLine className="mx-auto size-12 text-ember" aria-hidden />
        <h1 className="font-heading text-3xl">Case not found</h1>
        <p className="font-typewriter">This file was never in the archives, or someone removed it.</p>
        <Stamp>Missing</Stamp>
      </div>
      <Button asChild>
        <Link to={to}>{label}</Link>
      </Button>
    </div>
  )
}
