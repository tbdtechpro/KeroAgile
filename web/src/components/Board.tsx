import { useTasks } from '../api/queries'
import type { Status, Task } from '../api/types'

const COLUMNS: { status: Status; label: string; color: string }[] = [
  { status: 'backlog', label: 'Backlog', color: '#eab308' },
  { status: 'todo', label: 'To Do', color: '#f97316' },
  { status: 'in_progress', label: 'In Progress', color: '#22c55e' },
  { status: 'review', label: 'Review', color: '#a78bfa' },
  { status: 'done', label: 'Done', color: '#6b7280' },
]

const PRIORITY_COLORS: Record<string, string> = {
  low: '#6b7280',
  medium: '#eab308',
  high: '#f97316',
  critical: '#ef4444',
}

function TaskCard({ task }: { task: Task }) {
  return (
    <div
      className="p-3 rounded-lg border text-sm mb-2 cursor-pointer hover:border-opacity-80 transition-colors"
      style={{ background: '#0f1629', borderColor: '#1e293b' }}
    >
      <div className="font-medium mb-1 leading-tight" style={{ color: '#f8fafc' }}>
        {task.blockers && task.blockers.length > 0 && (
          <span style={{ color: '#ef4444' }}>⚠ </span>
        )}
        {task.title}
      </div>
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs" style={{ color: '#6b7280' }}>{task.id}</span>
        <span
          className="text-xs px-1.5 py-0.5 rounded"
          style={{ background: PRIORITY_COLORS[task.priority] + '22', color: PRIORITY_COLORS[task.priority] }}
        >
          {task.priority}
        </span>
        {task.assignee_id && (
          <span className="text-xs" style={{ color: '#a78bfa' }}>@{task.assignee_id}</span>
        )}
        {task.points && (
          <span className="text-xs" style={{ color: '#6b7280' }}>{task.points}pt</span>
        )}
      </div>
      {task.labels && task.labels.length > 0 && (
        <div className="flex gap-1 mt-1.5 flex-wrap">
          {task.labels.map(l => (
            <span key={l} className="text-xs px-1 rounded" style={{ background: '#1e1b4b', color: '#a78bfa' }}>
              {l}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function Column({ label, color, tasks }: { status: Status; label: string; color: string; tasks: Task[] }) {
  return (
    <div className="flex flex-col min-w-52 flex-1">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs font-bold" style={{ color }}>◆ {label}</span>
        <span className="text-xs" style={{ color: '#6b7280' }}>({tasks.length})</span>
      </div>
      <div className="flex-1 min-h-16">
        {tasks.map(t => <TaskCard key={t.id} task={t} />)}
      </div>
    </div>
  )
}

export default function Board({ projectId }: { projectId: string }) {
  const { data: tasks = [], isLoading } = useTasks({ project_id: projectId })

  if (isLoading) {
    return <div className="p-4 text-sm" style={{ color: 'var(--ka-muted)' }}>Loading tasks…</div>
  }

  const byStatus = (status: Status) => tasks.filter(t => t.status === status)

  return (
    <div className="flex-1 overflow-x-auto p-4">
      <div className="flex gap-4 min-w-max h-full">
        {COLUMNS.map(col => (
          <Column
            key={col.status}
            {...col}
            tasks={byStatus(col.status)}
          />
        ))}
      </div>
    </div>
  )
}
