import { renderToString } from 'react-dom/server'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createStaticHandler, createStaticRouter, StaticRouterProvider } from 'react-router'
import Providers from './components/layout/Providers'
import { routes } from './routes/routes'

/**
 * Build-time prerender (scripts/prerender.mjs): renders the page at `url` to HTML. It runs in
 * Node once per build, never per request, so the result is a plain static file.
 */
export async function render(url: string): Promise<string> {
  const { query, dataRoutes } = createStaticHandler(routes)
  const context = await query(new Request(new URL(url, 'http://localhost')))
  if (context instanceof Response) throw new Error(`prerender of ${url} redirected (${context.status})`)
  const router = createStaticRouter(dataRoutes, context)
  return renderToString(
    <QueryClientProvider client={new QueryClient()}>
      <Providers>
        <StaticRouterProvider router={router} context={context} />
      </Providers>
    </QueryClientProvider>,
  )
}
