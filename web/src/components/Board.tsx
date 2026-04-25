import { useState } from 'react'
import {
  DndContext,
  DragOverlay,
  useDroppable,
  useDraggable,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import type { Status, Task } from '../api/types'

const COLUMNS: { status: Status; label: string; color: string }[] = [
  { status: 'backlog', label: 'Backlog', color: 'var(--ka-yellow)' },
  { status: 'todo', label: 'To Do', color: 'var(--ka-orange)' },
  { status: 'in_progress', label: 'In Progress', color: 'var(--ka-green)' },
  { status: 'review', label: 'Review', color: 'var(--ka-accent-lt)' },
  { status: 'done', label: 'Done', color: 'var(--ka-muted)' },
]

const PRIORITY_COLORS: Record<string, string> = {
  low: 'var(--ka-muted)',
  medium: 'var(--ka-yellow)',
  high: 'var(--ka-orange)',
  critical: 'var(--ka-red)',
}

function TaskCard({
  task,
  onClick,
  ghost = false,
}: {
  task: Task
  onClick?: () => void
  ghost?: boolean
}) {
  return (
    <div
      onClick={onClick}
      className="p-3 rounded-lg border text-sm mb-2 transition-colors"
      style={{
        background: '#0f1629',
        borderColor: '#1e293b',
        color: 'var(--ka-text)',
        opacity: ghost ? 0.3 : 1,
        cursor: onClick ? 'pointer' : 'grabbing',
      }}
    >
      <div className="font-medium mb-1 leading-tight">
        {task.blockers && task.blockers.length > 0 && (
          <span style={{ color: 'var(--ka-red)' }}>⚠ </span>
        )}
        {task.title}
      </div>
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs" style={{ color: 'var(--ka-muted)' }}>{task.id}</span>
        <span
          className="text-xs px-1.5 py-0.5 rounded"
          style={{
            background: PRIORITY_COLORS[task.priority] + '33',
            color: PRIORITY_COLORS[task.priority],
          }}
        >
          {task.priority}
        </span>
        {task.assignee_id && (
          <span className="text-xs" style={{ color: 'var(--ka-accent-lt)' }}>@{task.assignee_id}</span>
        )}
        {task.points != null && (
          <span className="text-xs" style={{ color: 'var(--ka-muted)' }}>{task.points}pt</span>
        )}
      </div>
      {task.labels && task.labels.length > 0 && (
        <div className="flex gap-1 mt-1.5 flex-wrap">
          {task.labels.map(l => (
            <span
              key={l}
              className="text-xs px-1 rounded"
              style={{ background: '#1e1b4b', color: 'var(--ka-accent-lt)' }}
            >
              {l}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function DraggableCard({ task, onSelect }: { task: Task; onSelect: () => void }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: task.id })

  return (
    <div ref={setNodeRef} {...listeners} {...attributes} style={{ touchAction: 'none' }}>
      <TaskCard task={task} onClick={isDragging ? undefined : onSelect} ghost={isDragging} />
    </div>
  )
}

function DroppableColumn({
  status,
  label,
  color,
  tasks,
  onSelect,
  onNewTask,
}: {
  status: Status
  label: string
  color: string
  tasks: Task[]
  onSelect: (task: Task) => void
  onNewTask: () => void
}) {
  const { setNodeRef, isOver } = useDroppable({ id: status })

  return (
    <div className="flex flex-col min-w-52 flex-1">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xs font-bold" style={{ color }}>◆ {label}</span>
        <span className="text-xs" style={{ color: 'var(--ka-muted)' }}>({tasks.length})</span>
        <button
          onClick={onNewTask}
          className="ml-auto text-xs transition-opacity opacity-40 hover:opacity-100"
          style={{ color: 'var(--ka-accent-lt)' }}
          title="New task"
        >
          +
        </button>
      </div>
      <div
        ref={setNodeRef}
        className="flex-1 min-h-16 rounded-lg p-1 transition-colors"
        style={{ background: isOver ? 'rgba(124,58,237,0.1)' : 'transparent' }}
      >
        {tasks.map(t => (
          <DraggableCard key={t.id} task={t} onSelect={() => onSelect(t)} />
        ))}
      </div>
    </div>
  )
}

export default function Board({
  tasks,
  onSelectTask,
  onNewTask,
  onMove,
}: {
  tasks: Task[]
  onSelectTask: (task: Task) => void
  onNewTask: (status: Status) => void
  onMove: (taskId: string, status: Status) => void
}) {
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  )

  function handleDragStart(e: DragStartEvent) {
    const task = tasks.find(t => t.id === e.active.id)
    if (task) setActiveTask(task)
  }

  function handleDragEnd(e: DragEndEvent) {
    const { active, over } = e
    setActiveTask(null)
    if (!over) return
    const newStatus = over.id as Status
    const task = tasks.find(t => t.id === active.id)
    if (!task || task.status === newStatus) return
    onMove(active.id as string, newStatus)
  }

  const byStatus = (status: Status) => tasks.filter(t => t.status === status)

  return (
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <div className="flex gap-4 min-w-max h-full">
        {COLUMNS.map(col => (
          <DroppableColumn
            key={col.status}
            {...col}
            tasks={byStatus(col.status)}
            onSelect={onSelectTask}
            onNewTask={() => onNewTask(col.status)}
          />
        ))}
      </div>
      <DragOverlay dropAnimation={null}>
        {activeTask && <TaskCard task={activeTask} />}
      </DragOverlay>
    </DndContext>
  )
}
