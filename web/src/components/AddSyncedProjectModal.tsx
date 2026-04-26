import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'

interface Props { onClose: () => void }

export function AddSyncedProjectModal({ onClose }: Props) {
  const [url, setUrl] = useState('')
  const [token, setToken] = useState('')
  const [projectId, setProjectId] = useState('')
  const qc = useQueryClient()

  const mut = useMutation({
    mutationFn: () => api.addSyncedProject(url, token, projectId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['projects'] })
      onClose()
    },
  })

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl shadow-xl p-6 w-full max-w-md space-y-4">
        <h2 className="text-lg font-semibold">Add Synced Project</h2>
        <p className="text-sm text-gray-600">
          Connect to a primary KeroAgile install and sync one of its shared projects to this device.
        </p>
        <input className="w-full border rounded px-2 py-1 text-sm" placeholder="Primary URL (https://...)"
          value={url} onChange={e => setUrl(e.target.value)} />
        <input className="w-full border rounded px-2 py-1 text-sm" placeholder="API token"
          value={token} onChange={e => setToken(e.target.value)} type="password" />
        <input className="w-full border rounded px-2 py-1 text-sm" placeholder="Project ID (e.g. KA)"
          value={projectId} onChange={e => setProjectId(e.target.value)} />
        {mut.isError && <p className="text-red-600 text-xs">{String(mut.error)}</p>}
        <div className="flex justify-end gap-2">
          <button className="px-3 py-1 text-sm rounded border" onClick={onClose}>Cancel</button>
          <button className="px-3 py-1 text-sm rounded bg-blue-600 text-white"
            onClick={() => mut.mutate()}
            disabled={!url || !token || !projectId || mut.isPending}>
            {mut.isPending ? 'Connecting...' : 'Connect'}
          </button>
        </div>
      </div>
    </div>
  )
}
