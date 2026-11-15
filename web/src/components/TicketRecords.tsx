import { useState } from 'react'
import { useAddNote, useTask } from '../api/queries'
import FileList from './FileList'

// TicketRecords shows what has accumulated on a ticket: its notes, links and
// files. Listings leave these out, so it reads the ticket in full.
export default function TicketRecords({
  taskId,
  onToast,
}: {
  taskId: string
  onToast: (msg: string, type: 'success' | 'error') => void
}) {
  const { data: task } = useTask(taskId)
  const addNote = useAddNote()
  const [draft, setDraft] = useState('')

  async function submitNote() {
    const body = draft.trim()
    if (!body) return
    try {
      await addNote.mutateAsync({ taskId, body })
      setDraft('')
    } catch (e) {
      onToast(e instanceof Error ? e.message : 'Failed to add note', 'error')
    }
  }

  const notes = task?.notes ?? []
  const files = task?.artifacts ?? []

  return (
    <div className="flex flex-col gap-4">
      {files.length > 0 && (
        <div>
          <p className="text-xs mb-1" style={{ color: 'var(--ka-muted)' }}>Files ({files.length})</p>
          <FileList files={files} onToast={onToast} />
        </div>
      )}
      <div>
        <p className="text-xs mb-1" style={{ color: 'var(--ka-muted)' }}>
          Notes{notes.length > 0 ? ` (${notes.length})` : ''}
        </p>
        <ol className="flex flex-col gap-2">
          {notes.map(n => (
            <li key={n.id} className="text-sm">
              <p className="text-xs" style={{ color: 'var(--ka-muted)' }}>
                {new Date(n.created_at).toLocaleString()}
                {n.author_id && <span style={{ color: 'var(--ka-accent-lt)' }}> · @{n.author_id}</span>}
              </p>
              <p className="whitespace-pre-wrap" style={{ color: 'var(--ka-text)' }}>{n.body}</p>
            </li>
          ))}
        </ol>
        <textarea
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) submitNote()
          }}
          placeholder="Add a note"
          aria-label="Add a note"
          rows={2}
          className="mt-2 w-full text-sm rounded px-2 py-1 border"
          style={{ background: '#0f172a', borderColor: '#1e293b', color: 'var(--ka-text)' }}
        />
        <button
          type="button"
          onClick={submitNote}
          disabled={addNote.isPending || draft.trim() === ''}
          className="mt-1 text-xs px-2 py-1 rounded disabled:opacity-40"
          style={{ background: 'var(--ka-accent)', color: 'white' }}
        >
          {addNote.isPending ? 'Adding…' : 'Add note'}
        </button>
      </div>
    </div>
  )
}
