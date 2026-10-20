import { redirect, type RouteObject } from 'react-router'
import LoadingScreen from '../components/game/LoadingScreen'
import Layout from '../components/layout/Layout'
import { guestOnlyLoader, requireAuthLoader } from './guards'

/** The route table, shared by the browser router and the build-time prerender. */
export const routes: RouteObject[] = [
  {
    element: <Layout />,
    HydrateFallback: () => <LoadingScreen fullscreen />,
    children: [
      { path: '/', lazy: async () => ({ Component: (await import('../pages/LandingPage')).default }) },
      { path: '/how-to-play', lazy: async () => ({ Component: (await import('../pages/HowToPlayPage')).default }) },
      { path: '/auth/google/callback', lazy: async () => ({ Component: (await import('../pages/GoogleCallbackPage')).default }) },
      {
        loader: guestOnlyLoader,
        children: [
          { path: '/signup', lazy: async () => ({ Component: (await import('../pages/SignupPage')).default }) },
          { path: '/login', lazy: async () => ({ Component: (await import('../pages/LoginPage')).default }) },
          { path: '/verify-otp', lazy: async () => ({ Component: (await import('../pages/VerifyOtpPage')).default }) },
        ],
      },
      {
        loader: requireAuthLoader,
        children: [
          { path: '/dashboard', lazy: async () => ({ Component: (await import('../pages/DashboardPage')).default }) },
          { path: '/case/:id', lazy: async () => ({ Component: (await import('../pages/CasePage')).default }) },
          { path: '/onboarding/avatar', lazy: async () => ({ Component: (await import('../pages/AvatarOnboardingPage')).default }) },
          { path: '/onboarding/mode', lazy: async () => ({ Component: (await import('../pages/ModeOnboardingPage')).default }) },
          {
            // Admin panel (P5.3). AdminLayout sends non-admins back to the game.
            path: '/admin',
            lazy: async () => ({ Component: (await import('../components/admin/AdminLayout')).default }),
            children: [
              { index: true, loader: () => redirect('/admin/dashboard') },
              { path: 'dashboard', lazy: async () => ({ Component: (await import('../pages/admin/AdminDashboardPage')).default }) },
              { path: 'settings', lazy: async () => ({ Component: (await import('../pages/admin/AdminSettingsPage')).default }) },
              { path: 'cases/:id', lazy: async () => ({ Component: (await import('../pages/admin/AdminCaseEditorPage')).default }) },
            ],
          },
        ],
      },
      { path: '*', lazy: async () => ({ Component: (await import('../pages/NotFoundPage')).default }) },
    ],
  },
]
