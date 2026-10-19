import { createApi } from './createApi'
import type { UserProfile, UserRole } from './user'

export interface SignupInput {
  username: string
  email: string
  password: string
}

export interface OtpSent {
  email: string
  otp_expires_in: number
  resend_after_sec: number
}

export interface VerifyOtpInput {
  email: string
  otp: string
}

export interface LoginInput {
  email: string
  password: string
}

/** Returned by verify-otp and login. */
export interface AuthSession {
  jwt_token: string
  refresh_token: string
  expires_in: number
  user: { id: string; username: string; email: string; role: UserRole }
}

/** Returned by google/exchange: a session, plus whether the account was just created. */
export interface GoogleSession extends AuthSession {
  is_new: boolean
}

export type { UserProfile }

export const authApi = createApi({
  name: 'auth',
  endpoints: (build) => ({
    signup: build.mutation<OtpSent, SignupInput>({
      query: (body) => ({ url: '/api/auth/signup', method: 'POST', body }),
    }),
    resendOtp: build.mutation<OtpSent, { email: string }>({
      query: (body) => ({ url: '/api/auth/resend-otp', method: 'POST', body }),
    }),
    verifyOtp: build.mutation<AuthSession, VerifyOtpInput>({
      query: (body) => ({ url: '/api/auth/verify-otp', method: 'POST', body }),
    }),
    login: build.mutation<AuthSession, LoginInput>({
      query: (body) => ({ url: '/api/auth/login', method: 'POST', body }),
    }),
    // swaps the one-time ticket from the Google redirect for a session
    googleExchange: build.mutation<GoogleSession, { ticket: string }>({
      query: (body) => ({ url: '/api/auth/google/exchange', method: 'POST', body }),
    }),
    logout: build.mutation<{ message: string }, { refresh_token: string }>({
      query: (body) => ({ url: '/api/auth/logout', method: 'POST', body }),
    }),
  }),
})

export const {
  useSignupMutation,
  useResendOtpMutation,
  useVerifyOtpMutation,
  useLoginMutation,
  useGoogleExchangeMutation,
  useLogoutMutation,
} = authApi
