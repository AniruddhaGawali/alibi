import { Link } from 'react-router'

export default function NotFoundPage() {
  return (
    <>
      <h1 className="mb-2 font-heading text-3xl">Page not found</h1>
      <Link to="/">Back to the home page</Link>
    </>
  )
}
