import { useEffect } from 'react'
import { useGetStatsQuery } from '@/api/dashboard'
import { useGameStore } from '@/store/useGameStore'

/**
 * Mirrors the server's token balance into `useGameStore`, so the navbar, the stats
 * panel and the chat all show one number. Mounted once, in the layout. The chat
 * changes the store itself the moment a question is sent; whenever the server's
 * figure changes (a refetch after a question, the daily reset) it wins again.
 */
export function useSyncTokenBalance() {
  const { data } = useGetStatsQuery()
  const setTokenBalance = useGameStore((s) => s.setTokenBalance)
  const tokens = data?.tokens
  useEffect(() => {
    if (tokens !== undefined) setTokenBalance(tokens)
  }, [tokens, setTokenBalance])
}
