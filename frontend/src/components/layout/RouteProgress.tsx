import { useNavigation } from 'react-router'

/** Thin gold bar at the top of the page while a route is loading. */
export default function RouteProgress() {
  const { state } = useNavigation()
  if (state === 'idle') return null
  return <div className="route-progress" role="progressbar" aria-label="Loading page" />
}
