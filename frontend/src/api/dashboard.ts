import { createApi } from './createApi'

export interface DashboardData {
  tokenBalance: number
  trophies: number
  activeCaseId: string | null
}

// Placeholder endpoints: implemented on the backend in Phase 2.
export const dashboardApi = createApi({
  name: 'dashboard',
  endpoints: (build) => ({
    getDashboard: build.query<DashboardData>({
      query: () => '/api/dashboard',
      providesTags: ['Dashboard'],
      staleTime: 15,
    }),
  }),
})

export const { useGetDashboardQuery } = dashboardApi
