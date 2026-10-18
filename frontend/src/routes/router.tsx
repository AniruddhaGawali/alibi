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
        ],
      },
      { path: '*', lazy: async () => ({ Component: (await import('../pages/NotFoundPage')).default }) },
    ],
  },
])
