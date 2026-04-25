import { useEffect, useState } from 'react'
import type { Priority, Status, Task } from '../api/types'
import type { CreateTaskInput, UpdateTaskInput } from '../api/types'
import { useCreateTask, useUpdateTask, useUsers, useSprints } from '../api/queries'

const PRIORITIES: Priority[] = ['low', 'medium', 'high', 'critical']
const STATUSES: Status[] = ['backlog', 'todo', 'in_progress', 'review', 'done']

const fieldStyle = {
  background: '#0f172a',
  borderColor: '#1e293b',
  color: 'var(--ka-text)',
}

export default function TaskModal({
  projectId,
  initialStatus,
  task,
  onClose,
  onSuccess,
}: {
  projectId: string
  initialStatus?: Status
  task?: Task
  onClose: () => void
  onSuccess?: (message: string) => void
}) {
  const isEdit = !!task
  const { data: users = [] } = useUsers()
  const { data: sprintSummaries = [] } = useSprints(projectId)
  const createTask = useCreateTask()
  const updateTask = useUpdateTask()

  const [title, setTitle] = useState(task?.title ?? '')
  const [description, setDescription] = useState(task?.description ?? '')
  const [priority, setPriority] = useState<Priority>(task?.priority ?? 'medium')
  const [status, setStatus] = useState<Status>(task?.status ?? initialStatus ?? 'backlog')
  const [points, setPoints] = useState(task?.points != null ? String(task.points) : '')
  const [assigneeId, setAssigneeId] = useState(task?.assignee_id ?? '')
  const [labelsRaw, setLabelsRaw] = useState(task?.labels?.join(', ') ?? '')
  const [sprintId, setSprintId] = useState(task?.sprint_id != null ? String(task.sprint_id) : '')
  const [error, setError] = useState('')

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim()) {
      setError('Title is required')
      return
    }
    setError('')

    const labels = labelsRaw.split(',').map(s => s.trim()).filter(Boolean)
    const ptsNum = points ? parseInt(points) : undefined
    const sprint = sprintId ? parseInt(sprintId) : undefined

    try {
      if (isEdit) {
        const input: UpdateTaskInput = {
          title: title.trim(),
          description,
          priority,
          status,
          assignee_id: assigneeId || undefined,
          points: ptsNum,
          labels,
          sprint_id: sprint,
        }
        await updateTask.mutateAsync({ id: task.id, input })
        onSuccess?.(`Updated ${task.id}`)
      } else {
        const input: CreateTaskInput = {
          title: title.trim(),
          description,
          project_id: projectId,
          priority,
          status,
          assignee_id: assigneeId || undefined,
          points: ptsNum,
          labels,
          sprint_id: sprint,
        }
        await createTask.mutateAsync(input)
        onSuccess?.('Task created')
      }
      onClose()
    } catch {
      setError('Failed to save task')
    }
  }

  const isPending = createTask.isPending || updateTask.isPending

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50"
      style={{ background: 'rgba(0,0,0,0.6)' }}
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg rounded-xl border p-6 shadow-2xl"
        style={{ background: 'var(--ka-bg)', borderColor: '#1e293b' }}
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-base font-bold mb-4" style={{ color: 'var(--ka-text)' }}>
          {isEdit ? `Edit ${task.id}` : 'New Task'}
        </h2>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <input
            autoFocus
            placeholder="Title *"
            value={title}
            onChange={e => setTitle(e.target.value)}
            className="w-full px-3 py-2 rounded border text-sm"
            style={fieldStyle}
          />

          <textarea
            placeholder="Description"
            value={description}
            onChange={e => setDescription(e.target.value)}
            rows={3}
            className="w-full px-3 py-2 rounded border text-sm resize-none"
            style={fieldStyle}
          />

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs mb-1 block" style={{ color: 'var(--ka-muted)' }}>Priority</label>
              <select
                value={priority}
                onChange={e => setPriority(e.target.value as Priority)}
                className="w-full px-3 py-2 rounded border text-sm"
                style={fieldStyle}
              >
                {PRIORITIES.map(p => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs mb-1 block" style={{ color: 'var(--ka-muted)' }}>Status</label>
              <select
                value={status}
                onChange={e => setStatus(e.target.value as Status)}
                className="w-full px-3 py-2 rounded border text-sm"
                style={fieldStyle}
              >
                {STATUSES.map(s => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs mb-1 block" style={{ color: 'var(--ka-muted)' }}>Assignee</label>
              <select
                value={assigneeId}
                onChange={e => setAssigneeId(e.target.value)}
                className="w-full px-3 py-2 rounded border text-sm"
                style={fieldStyle}
              >
                <option value="">— unassigned —</option>
                {users.map(u => (
                  <option key={u.id} value={u.id}>{u.display_name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs mb-1 block" style={{ color: 'var(--ka-muted)' }}>Points</label>
              <input
                type="number"
                placeholder="Points"
                value={points}
                onChange={e => setPoints(e.target.value)}
                min="0"
                className="w-full px-3 py-2 rounded border text-sm"
                style={fieldStyle}
              />
            </div>
          </div>

          {sprintSummaries.length > 0 && (
            <div>
              <label className="text-xs mb-1 block" style={{ color: 'var(--ka-muted)' }}>Sprint</label>
              <select
                value={sprintId}
                onChange={e => setSprintId(e.target.value)}
                className="w-full px-3 py-2 rounded border text-sm"
                style={fieldStyle}
              >
                <option value="">— no sprint —</option>
                {sprintSummaries.map(ss => (
                  <option key={ss.sprint.id} value={ss.sprint.id}>{ss.sprint.name}</option>
                ))}
              </select>
            </div>
          )}

          <div>
            <label className="text-xs mb-1 block" style={{ color: 'var(--ka-muted)' }}>Labels (comma-separated)</label>
            <input
              placeholder="bug, frontend, …"
              value={labelsRaw}
              onChange={e => setLabelsRaw(e.target.value)}
              className="w-full px-3 py-2 rounded border text-sm"
              style={fieldStyle}
            />
          </div>

          {error && (
            <p className="text-xs" style={{ color: 'var(--ka-red)' }}>{error}</p>
          )}

          <div className="flex gap-2 justify-end mt-1">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm rounded border"
              style={{ borderColor: '#1e293b', color: 'var(--ka-muted)' }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="px-4 py-2 text-sm rounded font-medium"
              style={{
                background: 'var(--ka-accent)',
                color: 'white',
                opacity: isPending ? 0.7 : 1,
              }}
            >
              {isPending ? 'Saving…' : isEdit ? 'Save' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
