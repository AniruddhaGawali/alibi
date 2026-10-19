export {}

declare global {
  interface Window {
    /** Written into the prerendered HTML by react-router's StaticRouterProvider. */
    __staticRouterHydrationData?: import('react-router').HydrationState
  }
}
