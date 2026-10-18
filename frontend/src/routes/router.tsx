import { createBrowserRouter, redirect } from 'react-router'
import Layout from '../components/layout/Layout'
import CasePage from '../pages/CasePage'
import DashboardPage from '../pages/DashboardPage'
import SignupPage from '../pages/SignupPage'
import LoginPage from '../pages/LoginPage'
import VerifyOtpPage from '../pages/VerifyOtpPage'
import AvatarOnboardingPage from '../pages/AvatarOnboardingPage'
import NotFoundPage from '../pages/NotFoundPage'
import { guestOnlyLoader, requireAuthLoader } from './guards'

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { path: '/', loader: () => redirect('/dashboard') },
      {
        loader: guestOnlyLoader,
        children: [
          { path: '/signup', element: <SignupPage /> },
          { path: '/login', element: <LoginPage /> },
          { path: '/verify-otp', element: <VerifyOtpPage /> },
        ],
      },
      {
        loader: requireAuthLoader,
        children: [
          { path: '/dashboard', element: <DashboardPage /> },
          { path: '/case/:id', element: <CasePage /> },
          { path: '/onboarding/avatar', element: <AvatarOnboardingPage /> },
        ],
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])
