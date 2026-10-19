import { StrictMode } from 'react'
import { createRoot, hydrateRoot } from 'react-dom/client'
import { matchRoutes } from 'react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from './api/queryClient'
import './index.css'
import App from './App.tsx'
import { routes } from './routes/routes'

const root = document.getElementById('root')!
const app = (
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>
)

// Some pages ship as prerendered HTML (see scripts/prerender.mjs; data-prerendered says which).
// Hydrate it when it is the page being shown: the page is on screen before any JavaScript runs
// and React keeps the existing elements. The same HTML file can also be served as a fallback
// for other paths, in which case it is discarded and the app renders normally.
const path = window.location.pathname.replace(/\/+$/, '') || '/'
const marker = root.getAttribute('data-prerendered')
const isNotFound = matchRoutes(routes, path)?.at(-1)?.route.path === '*'
if (marker !== null && (marker === path || (marker === '404' && isNotFound))) {
  hydrateRoot(root, app)
} else {
  root.replaceChildren()
  createRoot(root).render(app)
}
