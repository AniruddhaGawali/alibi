import type { ReactNode } from 'react'
import { ThemeProvider } from 'next-themes'
import SoundEffects from '@/components/game/SoundEffects'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'

/**
 * The app-wide providers around the router. Shared by the browser entry (App) and the
 * build-time prerender (entry-server), so both render the same tree.
 */
export default function Providers({ children }: { children: ReactNode }) {
  // The app is dark-only; sonner reads its theme from next-themes.
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" forcedTheme="dark">
      <TooltipProvider>
        {children}
        <Toaster />
        <SoundEffects />
      </TooltipProvider>
    </ThemeProvider>
  )
}
