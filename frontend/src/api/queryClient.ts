import { QueryClient } from '@tanstack/react-query'
import { isRetryableError } from './apiClient'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000, // data is fresh for 30s
      gcTime: 5 * 60 * 1000, // unused cache kept for 5 minutes
      retry: (failureCount, error) => failureCount < 2 && isRetryableError(error),
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false, // never auto-retry token-spending actions
    },
  },
})
