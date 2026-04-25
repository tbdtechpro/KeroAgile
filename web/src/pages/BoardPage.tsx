import { useMemo, useState } from 'react'
import { useProjects, useTasks, useSprints, useMoveTask } from '../api/queries'
import type { Status, Task } from '../api/types'
import { useToast } from '../hooks/useToast'
import { getCurrentUserId } from '../api/client'
import Board from '../components/Board'
import TaskModal from '../components/TaskModal'
import TaskDetail from '../components/TaskDetail'
import SprintSidebar from '../components/SprintSidebar'
import ToastContainer from '../components/Toast'

type ModalState =
  | { open: false }
  | { open: true; status: Status; task?: undefined }
  | { open: true; task: Task; status?: undefined }

export default function BoardPage() {
  const { data: projects = [], isLoading: projectsLoading } = useProjects()
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)
  const [modal, setModal] = useState<ModalState>({ open: false })
  const [sprintFilter, setSprintFilter] = useState<number | null | undefined>(undefined)
  const [myTasksOnly, setMyTasksOnly] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const { toasts, push: pushToast, dismiss } = useToast()
  const moveTask = useMoveTask()

  const currentProjectId = selectedProjectId ?? projects[0]?.id ?? null
  const { data: sprintSummaries = [] } = useSprints(currentProjectId ?? undefined)
  const { data: allTasks = [], isLoading: tasksLoading } = useTasks({
    project_id: currentProjectId ?? undefined,
  })

  const displayedTasks = useMemo(() => {
    let filtered = allTasks
    if (sprintFilter === null) {
      filtered = filtered.filter(t => t.sprint_id === null)
    } else if (typeof sprintFilter === 'number') {
      filtered = filtered.filter(t => t.sprint_id === sprintFilter)
    }
    if (myTasksOnly) {
      const me = getCurrentUserId()
      if (me) filtered = filtered.filter(t => t.assignee_id === me)
    }
    return filtered
  }, [allTasks, sprintFilter, myTasksOnly])

  const selectedTask = useMemo(
    () => (selectedTaskId ? allTasks.find(t => t.id === selectedTaskId) ?? null : null),
    [selectedTaskId, allTasks]
  )

  async function handleMove(taskId: string, status: Status) {
    try {
      await moveTask.mutateAsync({ id: taskId, status })
    } catch {
      pushToast('Failed to move task', 'error')
    }
  }

  function selectProject(id: string) {
    setSelectedProjectId(id)
    setSelectedTaskId(null)
    setSprintFilter(undefined)
    setSidebarOpen(false)
  }

  if (projectsLoading) {
    return <div className="p-8 text-sm" style={{ color: 'var(--ka-muted)' }}>Loading…</div>
  }

  if (projects.length === 0) {
    return (
      <div className="p-8">
        <p style={{ color: 'var(--ka-muted)' }}>No projects found.</p>
        <p className="text-sm mt-2" style={{ color: 'var(--ka-muted)' }}>
          Create one with:{' '}
          <code className="px-1 py-0.5 rounded" style={{ background: '#1e293b' }}>
            KeroAgile project add MYAPP "My App"
          </code>
        </p>
      </div>
    )
  }

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 48px)' }}>
      {/* Project tabs */}
      <div
        className="flex items-center gap-1 px-4 pt-2 border-b shrink-0"
        style={{ borderColor: '#1e293b' }}
      >
        {/* Mobile: hamburger to open sidebar drawer */}
        <button
          onClick={() => setSidebarOpen(v => !v)}
          className="md:hidden mr-2 text-base opacity-60 hover:opacity-100 transition-opacity"
          style={{ color: 'var(--ka-text)' }}
          aria-label="Toggle filters"
        >
          ☰
        </button>

        {projects.map(p => (
          <button
            key={p.id}
            onClick={() => selectProject(p.id)}
            className="px-3 py-1 text-sm rounded-t transition-colors"
            style={{
              background: p.id === currentProjectId ? 'var(--ka-surface)' : 'transparent',
              color: p.id === currentProjectId ? 'var(--ka-accent-lt)' : 'var(--ka-muted)',
              borderBottom: p.id === currentProjectId
                ? '2px solid var(--ka-accent)'
                : '2px solid transparent',
            }}
          >
            {p.id}
          </button>
        ))}
        <button
          onClick={() => setModal({ open: true, status: 'backlog' })}
          className="ml-auto mb-1 px-2 py-1 text-xs rounded transition-opacity opacity-60 hover:opacity-100"
          style={{ color: 'var(--ka-accent-lt)', background: 'var(--ka-surface)' }}
        >
          + New task
        </button>
      </div>

      {/* Main area */}
      <div className="flex flex-1 overflow-hidden">
        <SprintSidebar
          sprintSummaries={sprintSummaries}
          selectedSprintId={sprintFilter}
          myTasksOnly={myTasksOnly}
          onSelectSprint={setSprintFilter}
          onToggleMyTasks={() => setMyTasksOnly(v => !v)}
          mobileOpen={sidebarOpen}
          onMobileClose={() => setSidebarOpen(false)}
        />

        <div className="flex-1 overflow-x-auto p-4">
          {tasksLoading ? (
            <div className="text-sm" style={{ color: 'var(--ka-muted)' }}>Loading tasks…</div>
          ) : currentProjectId ? (
            <Board
              tasks={displayedTasks}
              onSelectTask={t => setSelectedTaskId(t.id)}
              onNewTask={status => setModal({ open: true, status })}
              onMove={handleMove}
            />
          ) : null}
        </div>

        {selectedTask && (
          <TaskDetail
            task={selectedTask}
            onClose={() => setSelectedTaskId(null)}
            onEdit={() => setModal({ open: true, task: selectedTask })}
            onToast={pushToast}
          />
        )}
      </div>

      {modal.open && currentProjectId && (
        <TaskModal
          projectId={currentProjectId}
          initialStatus={modal.task ? undefined : modal.status}
          task={modal.task}
          onClose={() => setModal({ open: false })}
          onSuccess={msg => pushToast(msg, 'success')}
        />
      )}

      <ToastContainer toasts={toasts} dismiss={dismiss} />
    </div>
  )
}
