import { createBrowserRouter, redirect } from 'react-router'
import Layout from '../components/layout/Layout'
import { guestOnlyLoader, requireAuthLoader } from './guards'

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { path: '/', loader: () => redirect('/dashboard') },
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
])
