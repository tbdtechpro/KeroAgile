import type { SprintSummary } from '../api/types'

function SidebarButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className="w-full text-left text-xs px-2 py-1.5 rounded transition-colors"
      style={{
        background: active ? '#1e1b4b' : 'transparent',
        color: active ? 'var(--ka-accent-lt)' : 'var(--ka-muted)',
      }}
    >
      {children}
    </button>
  )
}

export default function SprintSidebar({
  sprintSummaries,
  selectedSprintId,
  myTasksOnly,
  onSelectSprint,
  onToggleMyTasks,
}: {
  sprintSummaries: SprintSummary[]
  selectedSprintId: number | null | undefined
  myTasksOnly: boolean
  onSelectSprint: (id: number | null | undefined) => void
  onToggleMyTasks: () => void
}) {
  return (
    <div
      className="flex flex-col border-r shrink-0 overflow-y-auto py-3"
      style={{ width: 192, borderColor: '#1e293b', background: 'var(--ka-bg)' }}
    >
      <div className="px-3">
        <p className="text-xs font-bold mb-2" style={{ color: 'var(--ka-muted)' }}>FILTERS</p>

        <button
          onClick={onToggleMyTasks}
          className="w-full text-left text-xs px-2 py-1.5 rounded mb-4 transition-colors"
          style={{
            background: myTasksOnly ? 'var(--ka-accent)' : 'transparent',
            color: myTasksOnly ? 'white' : 'var(--ka-muted)',
          }}
        >
          {myTasksOnly ? '● My tasks' : '○ My tasks'}
        </button>

        <p className="text-xs font-bold mb-2" style={{ color: 'var(--ka-muted)' }}>SPRINT</p>

        <div className="flex flex-col gap-0.5">
          <SidebarButton active={selectedSprintId === undefined} onClick={() => onSelectSprint(undefined)}>
            All tasks
          </SidebarButton>

          <SidebarButton active={selectedSprintId === null} onClick={() => onSelectSprint(null)}>
            No sprint
          </SidebarButton>

          {sprintSummaries.map(ss => (
            <SidebarButton
              key={ss.sprint.id}
              active={selectedSprintId === ss.sprint.id}
              onClick={() => onSelectSprint(ss.sprint.id)}
            >
              <span style={{ color: ss.sprint.status === 'active' ? 'var(--ka-green)' : undefined }}>
                {ss.sprint.status === 'active' ? '▶ ' : '  '}
              </span>
              {ss.sprint.name}
              {ss.task_count > 0 && (
                <span className="ml-1 opacity-50">{ss.task_count}</span>
              )}
            </SidebarButton>
          ))}
        </div>
      </div>
    </div>
  )
}
