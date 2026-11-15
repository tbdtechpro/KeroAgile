import { useState } from 'react'
import { useUnfiledArtifacts } from '../api/queries'
import FileList from './FileList'

// UnfiledFiles shows the files the sweep kept for a project that are on no
// ticket yet. Filing them is done from Claude or the CLI.
export default function UnfiledFiles({
  projectId,
  onToast,
}: {
  projectId: string | null
  onToast: (msg: string, type: 'success' | 'error') => void
}) {
  const { data: files = [] } = useUnfiledArtifacts(projectId)
  const [open, setOpen] = useState(false)
  if (files.length === 0) return null
  return (
    <div className="relative mb-1">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="px-2 py-1 text-xs rounded transition-opacity opacity-70 hover:opacity-100"
        style={{ color: 'var(--ka-yellow)', background: 'var(--ka-surface)' }}
        aria-expanded={open}
      >
        Unfiled files ({files.length})
      </button>
      {open && (
        <div
          className="absolute left-0 mt-1 z-40 w-80 max-w-[calc(100vw-2rem)] max-h-96 overflow-y-auto rounded border p-3 shadow-lg"
          style={{ background: 'var(--ka-bg)', borderColor: '#1e293b' }}
        >
          <p className="text-xs mb-2" style={{ color: 'var(--ka-muted)' }}>
            Kept by the sweep, on no ticket. File them with <code>file_artifact</code> or{' '}
            <code>KeroAgile artifacts file</code>.
          </p>
          <FileList files={files} onToast={onToast} />
        </div>
      )}
    </div>
  )
}
