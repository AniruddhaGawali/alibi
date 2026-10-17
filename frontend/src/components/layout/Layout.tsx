import { Link, Outlet } from 'react-router'

export default function Layout() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="flex items-center border-b px-6 py-4">
        <Link to="/" className="font-heading text-2xl text-primary no-underline">
          Alibi
        </Link>
      </header>
      <main className="mx-auto max-w-6xl p-6">
        <Outlet />
      </main>
    </div>
  )
}
