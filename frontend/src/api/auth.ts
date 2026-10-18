import { createApi } from './createApi'
import type { UserProfile } from './user'

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
  user: { id: string; username: string; email: string }
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
  useLogoutMutation,
} = authApi
