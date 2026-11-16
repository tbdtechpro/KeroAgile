import { useState } from 'react'
import { api } from '../api/client'
import type { Artifact } from '../api/types'
import { humanSize } from '../api/format'

const WHERE_LABEL: Record<Artifact['where'], string> = {
  here: 'on the server',
  remote: 'on the remote store',
  both: 'on the server and the remote store',
  unavailable: 'unavailable',
}

// FileList lists kept files with a download button each.
export default function FileList({
  files,
  onToast,
  renderExtra,
}: {
  files: Artifact[]
  onToast: (msg: string, type: 'success' | 'error') => void
  renderExtra?: (a: Artifact) => React.ReactNode
}) {
  const [busy, setBusy] = useState<number | null>(null)

  async function download(a: Artifact) {
    setBusy(a.id)
    try {
      const { url } = await api.artifactURL(a.id)
      const link = document.createElement('a')
      link.href = url + '&download=1'
      link.download = a.name
      document.body.appendChild(link)
      link.click()
      link.remove()
    } catch (e) {
      onToast(e instanceof Error ? e.message : 'Download failed', 'error')
    } finally {
      setBusy(null)
    }
  }

  return (
    <ul className="flex flex-col gap-2">
      {files.map(a => (
        <li key={a.id} className="text-sm">
          <div className="flex items-center gap-2">
            <span className="truncate flex-1" style={{ color: 'var(--ka-text)' }} title={a.name}>{a.name}</span>
            <span className="text-xs shrink-0" style={{ color: 'var(--ka-muted)' }}>{a.kind} · {humanSize(a.size)}</span>
            {a.where === 'unavailable' ? (
              <span className="text-xs shrink-0" style={{ color: 'var(--ka-red)' }} title={a.sha256}>
                unavailable {a.sha256.slice(0, 12)}
              </span>
            ) : (
              <button
                type="button"
                onClick={() => download(a)}
                disabled={busy === a.id}
                className="text-xs px-2 py-0.5 rounded shrink-0 disabled:opacity-40"
                style={{ background: '#1e293b', color: 'var(--ka-accent-lt)' }}
                title={WHERE_LABEL[a.where]}
                aria-label={`Download ${a.name}`}
              >
                {busy === a.id ? '…' : 'Download'}
              </button>
            )}
          </div>
          {a.note && <p className="text-xs" style={{ color: 'var(--ka-muted)' }}>{a.note}</p>}
          {renderExtra?.(a)}
        </li>
      ))}
    </ul>
  )
}
