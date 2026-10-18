import { ThemeProvider } from 'next-themes'
import { RouterProvider } from 'react-router'
import SoundEffects from './components/game/SoundEffects'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { setSessionExpiredHandler } from './api/apiClient'
import { router } from './routes/router'
import { clearSession } from './utils/session'

// When the refresh token is rejected too, sign out and go to the login page.
setSessionExpiredHandler(() => {
  clearSession()
  void router.navigate('/login', { replace: true })
})

export default function App() {
  // The app is dark-only; sonner reads its theme from next-themes.
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" forcedTheme="dark">
      <TooltipProvider>
        <RouterProvider router={router} />
        <Toaster />
        <SoundEffects />
      </TooltipProvider>
    </ThemeProvider>
  )
}
