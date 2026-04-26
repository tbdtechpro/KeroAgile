import { useState } from 'react'
import type { Task } from '../api/types'
import { useDeleteTask, useRemoveBlocker } from '../api/queries'

const PRIORITY_COLORS: Record<string, string> = {
  low: 'var(--ka-muted)',
  medium: 'var(--ka-yellow)',
  high: 'var(--ka-orange)',
  critical: 'var(--ka-red)',
}

const STATUS_COLORS: Record<string, string> = {
  backlog: 'var(--ka-yellow)',
  todo: 'var(--ka-orange)',
  in_progress: 'var(--ka-green)',
  review: 'var(--ka-accent-lt)',
  done: 'var(--ka-muted)',
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="text-xs mb-1" style={{ color: 'var(--ka-muted)' }}>{label}</p>
      {children}
    </div>
  )
}

export default function TaskDetail({
  task,
  onClose,
  onEdit,
  onToast,
}: {
  task: Task
  onClose: () => void
  onEdit: () => void
  onToast: (msg: string, type: 'success' | 'error') => void
}) {
  const deleteTask = useDeleteTask()
  const removeBlocker = useRemoveBlocker()
  const [confirmDelete, setConfirmDelete] = useState(false)

  function handleBlockerClick(_blockerProjectId: string, _taskId: string) {
    onClose()
  }

  async function handleDelete() {
    if (!confirmDelete) {
      setConfirmDelete(true)
      return
    }
    try {
      await deleteTask.mutateAsync(task.id)
      onToast(`Deleted ${task.id}`, 'success')
      onClose()
    } catch {
      onToast('Failed to delete task', 'error')
      setConfirmDelete(false)
    }
  }

  return (
    <div
      className={[
        'flex flex-col border-l overflow-y-auto',
        // Mobile: full-screen overlay; desktop: 320px right panel
        'fixed md:static md:shrink-0',
        'inset-0 md:inset-auto',
        'top-12 md:top-auto',
        'z-30 md:z-auto',
        'w-full md:w-80',
      ].join(' ')}
      style={{ borderColor: '#1e293b', background: 'var(--ka-bg)' }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-4 py-3 border-b shrink-0"
        style={{ borderColor: '#1e293b' }}
      >
        <span className="text-xs font-bold" style={{ color: 'var(--ka-muted)' }}>{task.id}</span>
        <div className="flex items-center gap-2">
          <button
            onClick={onEdit}
            className="text-xs px-2 py-1 rounded"
            style={{ background: 'var(--ka-accent)', color: 'white' }}
          >
            Edit
          </button>
          <button
            onClick={onClose}
            className="text-xs opacity-50 hover:opacity-100 transition-opacity"
            style={{ color: 'var(--ka-text)' }}
          >
            ✕
          </button>
        </div>
      </div>

      {/* Body */}
      <div className="px-4 py-4 flex flex-col gap-4 flex-1">
        <div>
          {task.blockers && task.blockers.length > 0 && (
            <span className="text-sm mr-1" style={{ color: 'var(--ka-red)' }}>⚠</span>
          )}
          <h3 className="text-sm font-medium leading-snug inline" style={{ color: 'var(--ka-text)' }}>
            {task.title}
          </h3>
        </div>

        <div className="flex flex-wrap gap-2">
          <span
            className="text-xs px-2 py-0.5 rounded"
            style={{
              background: STATUS_COLORS[task.status] + '22',
              color: STATUS_COLORS[task.status],
            }}
          >
            {task.status.replace('_', ' ')}
          </span>
          <span
            className="text-xs px-2 py-0.5 rounded"
            style={{
              background: PRIORITY_COLORS[task.priority] + '22',
              color: PRIORITY_COLORS[task.priority],
            }}
          >
            {task.priority}
          </span>
          {task.points != null && (
            <span
              className="text-xs px-2 py-0.5 rounded"
              style={{ background: '#1e293b', color: 'var(--ka-muted)' }}
            >
              {task.points}pt
            </span>
          )}
        </div>

        {task.description && (
          <Field label="Description">
            <p className="text-sm whitespace-pre-wrap" style={{ color: 'var(--ka-text)' }}>
              {task.description}
            </p>
          </Field>
        )}

        {task.assignee_id && (
          <Field label="Assignee">
            <p className="text-sm" style={{ color: 'var(--ka-accent-lt)' }}>@{task.assignee_id}</p>
          </Field>
        )}

        {task.labels && task.labels.length > 0 && (
          <Field label="Labels">
            <div className="flex flex-wrap gap-1">
              {task.labels.map(l => (
                <span
                  key={l}
                  className="text-xs px-2 py-0.5 rounded"
                  style={{ background: '#1e1b4b', color: 'var(--ka-accent-lt)' }}
                >
                  {l}
                </span>
              ))}
            </div>
          </Field>
        )}

        {task.blockers && task.blockers.length > 0 && (
          <Field label="Blocked by">
            <div className="flex flex-wrap gap-1">
              {(task.blocker_details && task.blocker_details.length > 0
                ? task.blocker_details
                : task.blockers.map(id => ({ id, title: '', project_id: task.project_id, status: task.status }))
              ).map(b => {
                const isCross = b.project_id !== task.project_id
                return (
                  <span
                    key={b.id}
                    className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded cursor-pointer"
                    style={{
                      background: isCross ? '#1e3a5f' : '#7f1d1d',
                      color: isCross ? '#93c5fd' : 'var(--ka-red)',
                    }}
                    onClick={() => handleBlockerClick(b.project_id || task.project_id, b.id)}
                    title={b.title || b.id}
                  >
                    {isCross && <span>↗</span>}
                    <span>{b.id}</span>
                    {b.title && <span className="opacity-75">{b.title.length > 24 ? b.title.slice(0, 21) + '…' : b.title}</span>}
                    {isCross && (
                      <span
                        className="text-xs px-1 rounded"
                        style={{ background: 'rgba(255,255,255,0.1)', fontSize: '0.65rem' }}
                      >
                        {b.project_id}
                      </span>
                    )}
                    <button
                      type="button"
                      onClick={e => {
                        e.stopPropagation()
                        removeBlocker.mutate({ taskId: task.id, blockerId: b.id })
                      }}
                      className="ml-1 opacity-50 hover:opacity-100"
                    >
                      ×
                    </button>
                  </span>
                )
              })}
            </div>
          </Field>
        )}

        {task.blocking_details && task.blocking_details.length > 0 && (
          <Field label="Blocking">
            <div className="flex flex-wrap gap-1">
              {task.blocking_details.map(b => {
                const isCross = b.project_id !== task.project_id
                return (
                  <span
                    key={b.id}
                    className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded cursor-pointer"
                    style={{
                      background: isCross ? '#1e3a5f' : '#064e3b',
                      color: isCross ? '#93c5fd' : '#6ee7b7',
                    }}
                    onClick={() => handleBlockerClick(b.project_id || task.project_id, b.id)}
                    title={b.title || b.id}
                  >
                    {isCross && <span>↗</span>}
                    <span>►</span>
                    <span>{b.id}</span>
                    {b.title && <span className="opacity-75">{b.title.length > 24 ? b.title.slice(0, 21) + '…' : b.title}</span>}
                    {isCross && (
                      <span
                        className="text-xs px-1 rounded"
                        style={{ background: 'rgba(255,255,255,0.1)', fontSize: '0.65rem' }}
                      >
                        {b.project_id}
                      </span>
                    )}
                  </span>
                )
              })}
            </div>
          </Field>
        )}

        {task.branch && (
          <Field label="Branch">
            <code className="text-xs" style={{ color: 'var(--ka-green)' }}>{task.branch}</code>
          </Field>
        )}

        {task.pr_number != null && (
          <Field label="PR">
            <span className="text-xs" style={{ color: 'var(--ka-accent-lt)' }}>
              #{task.pr_number} {task.pr_merged ? '(merged)' : '(open)'}
            </span>
          </Field>
        )}

        <Field label="Updated">
          <p className="text-xs" style={{ color: 'var(--ka-muted)' }}>
            {new Date(task.updated_at).toLocaleString()}
          </p>
        </Field>
      </div>

      {/* Footer */}
      <div className="px-4 py-3 border-t shrink-0" style={{ borderColor: '#1e293b' }}>
        <button
          onClick={handleDelete}
          disabled={deleteTask.isPending}
          className="w-full text-xs py-1.5 rounded border transition-colors"
          style={{
            borderColor: confirmDelete ? 'var(--ka-red)' : '#1e293b',
            color: confirmDelete ? 'var(--ka-red)' : 'var(--ka-muted)',
          }}
        >
          {deleteTask.isPending ? 'Deleting…' : confirmDelete ? 'Confirm delete?' : 'Delete task'}
        </button>
      </div>
    </div>
  )
}
