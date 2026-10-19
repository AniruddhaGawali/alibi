import { useEffect } from 'react'
import { useGetStatsQuery } from '@/api/dashboard'
import { useSessionStore } from '@/store/useSessionStore'
import { useSoundStore } from '@/store/useSoundStore'
import { music, musicUrl } from '@/utils/music'

/**
 * Plays the background music on every page (mounted once in Layout, so it keeps
 * playing across navigation): the active case's track while the player is inside a
 * case, the main theme everywhere else, including login and signup. Starting a case
 * or settling the accusation refreshes the stats, which switches the track.
 */
export default function MusicPlayer() {
  const loggedIn = useSessionStore((s) => s.loggedIn)
  const musicMuted = useSoundStore((s) => s.musicMuted)
  const volume = useSoundStore((s) => s.musicVolume)
  const { data: stats } = useGetStatsQuery(undefined, { enabled: loggedIn })
  const url = musicUrl(loggedIn ? stats?.active_music_track : null)

  // The volume is set first, so a track that starts right away fades up to the right level.
  useEffect(() => music.setVolume(volume), [volume])
  useEffect(() => music.play(url), [url])
  useEffect(() => music.setMuted(musicMuted), [musicMuted])
  return null
}
