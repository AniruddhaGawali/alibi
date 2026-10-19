import { RouterProvider } from 'react-router'
import Providers from './components/layout/Providers'
import { setSessionExpiredHandler } from './api/apiClient'
import { router } from './routes/router'
import { clearSession } from './utils/session'

// When the refresh token is rejected too, sign out and go to the login page.
setSessionExpiredHandler(() => {
  clearSession()
  void router.navigate('/login', { replace: true })
})

export default function App() {
  return (
    <Providers>
      <RouterProvider router={router} />
    </Providers>
  )
}
