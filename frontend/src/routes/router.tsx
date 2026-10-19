import { createBrowserRouter, matchRoutes } from 'react-router'
import { routes } from './routes'

// The landing page is prerendered (scripts/prerender.mjs). To hydrate it, the lazy route it
// matches must be loaded before the router starts, or the first render would show the
// loading screen in place of the prerendered page.
const hydrationData = window.__staticRouterHydrationData
if (hydrationData) {
  const lazyMatches = matchRoutes(routes, window.location)?.filter((m) => m.route.lazy) ?? []
  await Promise.all(
    lazyMatches.map(async (m) => {
      const loaded = await (m.route.lazy as () => Promise<object>)()
      Object.assign(m.route, { ...loaded, lazy: undefined })
    }),
  )
}

export const router = createBrowserRouter(routes, { hydrationData })
