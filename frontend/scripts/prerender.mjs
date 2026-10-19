// Prerenders the public pages after `vite build` and the SSR build (see package.json):
//
//   dist/index.html              the landing page, served for "/"
//   dist/how-to-play/index.html  the rules page
//   dist/404.html                the "Case not found" page, for unknown paths
//   dist/200.html                the untouched app shell: the SPA fallback for the app routes
//
// Host config: serve dist as static files, use 404.html for unknown paths that are not app
// routes, and 200.html (not index.html) as the fallback for the app routes (/login,
// /dashboard, ...), e.g. a Netlify/Cloudflare "/* /200.html 200" rule after the static files.
//
// To prerender another page: add it to PAGES, and make main.tsx able to tell it applies
// (data-prerendered holds `marker`).
import { mkdir, readFile, writeFile, rm } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

const PAGES = [
  { url: '/', out: 'index.html', marker: '/' },
  { url: '/how-to-play', out: 'how-to-play/index.html', marker: '/how-to-play' },
  // any path no route matches renders the catch-all route
  { url: '/page-not-found', out: '404.html', marker: '404' },
]

const root = path.resolve(import.meta.dirname, '..')
const dist = path.join(root, 'dist')
const ssr = path.join(root, 'dist-ssr')

const { render } = await import(pathToFileURL(path.join(ssr, 'entry-server.js')).href)

const shell = await readFile(path.join(dist, 'index.html'), 'utf8')
const placeholder = '<div id="root"></div>'
if (!shell.includes(placeholder)) throw new Error(`prerender: ${placeholder} not found in dist/index.html`)
await writeFile(path.join(dist, '200.html'), shell)

for (const page of PAGES) {
  const html = await render(page.url)
  const file = path.join(dist, page.out)
  await mkdir(path.dirname(file), { recursive: true })
  await writeFile(file, shell.replace(placeholder, `<div id="root" data-prerendered="${page.marker}">${html}</div>`))
  console.log(`prerendered ${page.url} (${(html.length / 1024).toFixed(1)} kB of HTML) -> dist/${page.out}`)
}
await rm(ssr, { recursive: true, force: true })
