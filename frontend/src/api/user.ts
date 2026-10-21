import { createApi } from './createApi'

export interface AvatarInfo {
  type: 'default' | 'custom' | 'none'
  id?: number
  url?: string
}

export interface UserProfile {
  id: string
  username: string
  email: string
  avatar: AvatarInfo
  trophies: number
  token_balance: number
  role: UserRole
  player_mode: PlayerMode
}

export type UserRole = 'user' | 'admin'

/** Difficulty mode: investigators see suspect backgrounds and chat tips, master sleuths go in blind. */
export type PlayerMode = 'investigator' | 'master_sleuth'

/** Either a custom image (max 1 MB) or the number (1-4) of a default avatar. */
export type SetAvatarInput = { file: File } | { avatar: number }

export const userApi = createApi({
  name: 'user',
  endpoints: (build) => ({
    getUserData: build.query<UserProfile>({
      query: () => '/api/user/profile',
      providesTags: ['User'],
      staleTime: 0,
    }),
    setAvatar: build.mutation<AvatarInfo, SetAvatarInput>({
      query: (input) => {
        if ('file' in input) {
          const body = new FormData()
          body.append('file', input.file)
          return { url: '/api/user/avatar', method: 'POST', body }
        }
        return { url: '/api/user/avatar', method: 'POST', body: { avatar: input.avatar } }
      },
      invalidatesTags: ['User'],
    }),
    setPlayerMode: build.mutation<{ player_mode: PlayerMode }, PlayerMode>({
      query: (mode) => ({ url: '/api/user/mode', method: 'PUT', body: { mode } }),
      invalidatesTags: ['User'],
    }),
    setUsername: build.mutation<{ username: string }, string>({
      query: (username) => ({ url: '/api/user/username', method: 'PUT', body: { username } }),
      invalidatesTags: ['User', 'Leaderboard'],
    }),
  }),
})

export const {
  useGetUserDataQuery,
  useSetAvatarMutation,
  useSetPlayerModeMutation,
  useSetUsernameMutation,
} = userApi
