import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'

export function SyncStatus() {
  const { data } = useQuery({
    queryKey: ['sync', 'status'],
    queryFn: api.getSyncStatus,
    refetchInterval: 10_000,
  })

  if (!data || data.state === 'online' || data.state === 'standalone') return null

  return (
    <div className={`flex items-center gap-2 text-sm px-3 py-1 rounded ${
      data.state === 'offline'
        ? 'bg-red-50 text-red-700 border border-red-200'
        : 'text-yellow-700'
    }`}>
      <span className={`inline-block w-2 h-2 rounded-full ${
        data.state === 'offline' ? 'bg-red-500' : 'bg-yellow-400'
      }`} />
      {data.state === 'offline'
        ? 'Primary server unreachable — synced projects are read-only'
        : 'Reconnecting to primary...'}
    </div>
  )
}
