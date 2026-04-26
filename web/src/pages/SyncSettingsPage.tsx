import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import { useProjects } from '../api/queries'
import type { Secondary } from '../api/types'

export default function SyncSettingsPage() {
  const qc = useQueryClient()
  const { data: secondaries = [], isLoading: secsLoading, isError: secsError } = useQuery<Secondary[]>({
    queryKey: ['sync', 'secondaries'],
    queryFn: api.syncListSecondaries,
  })
  const { data: projects = [] } = useProjects()
  const [newId, setNewId] = useState('')
  const [newName, setNewName] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)
  const [selectedSec, setSelectedSec] = useState<string | null>(null)

  const selectedSecObj = secondaries.find(s => s.id === selectedSec)

  const addMut = useMutation({
    mutationFn: () => api.syncAddSecondary(newId, newName),
    onSuccess: (data) => {
      setNewToken(data.token)
      setNewId('')
      setNewName('')
      qc.invalidateQueries({ queryKey: ['sync', 'secondaries'] })
    },
  })

  const revokeMut = useMutation({
    mutationFn: (id: string) => api.syncRevokeSecondary(id),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: ['sync', 'secondaries'] })
      if (selectedSec === variables) setSelectedSec(null)
    },
  })

  const { data: grants = [] } = useQuery<string[]>({
    queryKey: ['sync', 'grants', selectedSec],
    queryFn: () => api.syncListGrants(selectedSec!),
    enabled: !!selectedSec,
  })

  const grantMut = useMutation({
    mutationFn: ({ sid, pid }: { sid: string; pid: string }) => api.syncGrantProject(sid, pid),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sync', 'grants', selectedSec] }),
  })
  const revokeGrantMut = useMutation({
    mutationFn: ({ sid, pid }: { sid: string; pid: string }) => api.syncRevokeGrant(sid, pid),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sync', 'grants', selectedSec] }),
  })

  const mutationPending = grantMut.isPending || revokeGrantMut.isPending

  return (
    <div className="p-6 max-w-3xl mx-auto space-y-8 overflow-y-auto">
      <h1 className="text-xl font-bold">Sync Settings</h1>

      {/* Register secondary */}
      <section className="space-y-3">
        <h2 className="font-semibold">Register Secondary Install</h2>
        <div className="flex gap-2">
          <input
            className="border rounded px-2 py-1 text-sm flex-1"
            placeholder="ID (e.g. matt-laptop)"
            value={newId}
            onChange={e => setNewId(e.target.value)}
          />
          <input
            className="border rounded px-2 py-1 text-sm flex-1"
            placeholder="Display name"
            value={newName}
            onChange={e => setNewName(e.target.value)}
          />
          <button
            className="bg-blue-600 text-white px-3 py-1 rounded text-sm disabled:opacity-50"
            onClick={() => addMut.mutate()}
            disabled={!newId || !newName || addMut.isPending}
          >
            {addMut.isPending ? 'Adding…' : 'Add'}
          </button>
        </div>
        {newToken && (
          <div className="bg-yellow-50 border border-yellow-300 rounded p-3 text-sm space-y-1">
            <p className="font-semibold text-yellow-800">Token (shown once — copy it now):</p>
            <code className="block break-all text-yellow-900">{newToken}</code>
            <button
              className="text-xs text-yellow-700 underline"
              onClick={() => setNewToken(null)}
            >
              I've copied it
            </button>
          </div>
        )}
        {addMut.isError && (
          <p className="text-red-600 text-xs">{String(addMut.error)}</p>
        )}
      </section>

      {/* Secondary list */}
      <section className="space-y-2">
        <h2 className="font-semibold">Registered Secondaries</h2>
        {secsLoading && (
          <p className="text-sm text-gray-500">Loading…</p>
        )}
        {secsError && (
          <p className="text-red-600 text-xs">Failed to load secondaries.</p>
        )}
        {!secsLoading && !secsError && secondaries.length === 0 && (
          <p className="text-sm text-gray-500">No secondaries registered yet.</p>
        )}
        {secondaries.map(sec => (
          <div
            key={sec.id}
            className={`border rounded p-3 flex items-start justify-between gap-2 cursor-pointer hover:bg-gray-50 ${
              selectedSec === sec.id ? 'ring-2 ring-blue-400' : ''
            }`}
            onClick={() => setSelectedSec(selectedSec === sec.id ? null : sec.id)}
          >
            <div>
              <p className="font-medium text-sm">{sec.display_name}</p>
              <p className="text-xs text-gray-500">
                {sec.id}
                {sec.last_seen_at && ` · last seen ${new Date(sec.last_seen_at).toLocaleString()}`}
              </p>
            </div>
            <button
              className="text-red-500 text-xs hover:underline shrink-0"
              onClick={e => { e.stopPropagation(); revokeMut.mutate(sec.id) }}
            >
              Revoke
            </button>
          </div>
        ))}
      </section>

      {/* Grant management for selected secondary */}
      {selectedSec && (
        <section className="space-y-2">
          <h2 className="font-semibold">
            Projects shared with <span className="font-mono text-sm">{selectedSecObj?.display_name ?? selectedSec}</span>
          </h2>
          {projects.length === 0 && (
            <p className="text-sm text-gray-500">No projects.</p>
          )}
          {revokeGrantMut.isError && (
            <p className="text-red-600 text-xs">Failed to revoke secondary.</p>
          )}
          {projects.map(p => {
            const granted = grants.includes(p.id)
            return (
              <label key={p.id} className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="checkbox"
                  checked={granted}
                  disabled={mutationPending}
                  onChange={() =>
                    granted
                      ? revokeGrantMut.mutate({ sid: selectedSec, pid: p.id })
                      : grantMut.mutate({ sid: selectedSec, pid: p.id })
                  }
                />
                {p.name}
                <span className="text-gray-400 font-mono text-xs">({p.id})</span>
              </label>
            )
          })}
        </section>
      )}
    </div>
  )
}
