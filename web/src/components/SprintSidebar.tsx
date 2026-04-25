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
  mobileOpen,
  onMobileClose,
}: {
  sprintSummaries: SprintSummary[]
  selectedSprintId: number | null | undefined
  myTasksOnly: boolean
  onSelectSprint: (id: number | null | undefined) => void
  onToggleMyTasks: () => void
  mobileOpen: boolean
  onMobileClose: () => void
}) {
  function select(id: number | null | undefined) {
    onSelectSprint(id)
    onMobileClose()
  }

  return (
    <>
      {/* Mobile backdrop */}
      {mobileOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          style={{ top: 48 }}
          onClick={onMobileClose}
        />
      )}

      <div
        className={[
          'flex flex-col border-r overflow-y-auto py-3 transition-transform duration-200',
          // Mobile: fixed slide-in drawer; desktop: static in flex flow
          'fixed md:static',
          'top-12 md:top-auto left-0 md:left-auto',
          'h-[calc(100vh-48px)] md:h-auto',
          'z-50 md:z-auto',
          mobileOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
        ].join(' ')}
        style={{ width: 192, borderColor: '#1e293b', background: 'var(--ka-bg)', borderRightWidth: 1 }}
      >
        <div className="px-3">
          <p className="text-xs font-bold mb-2" style={{ color: 'var(--ka-muted)' }}>FILTERS</p>

          <button
            onClick={() => { onToggleMyTasks(); onMobileClose() }}
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
            <SidebarButton active={selectedSprintId === undefined} onClick={() => select(undefined)}>
              All tasks
            </SidebarButton>

            <SidebarButton active={selectedSprintId === null} onClick={() => select(null)}>
              No sprint
            </SidebarButton>

            {sprintSummaries.map(ss => (
              <SidebarButton
                key={ss.sprint.id}
                active={selectedSprintId === ss.sprint.id}
                onClick={() => select(ss.sprint.id)}
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
    </>
  )
}
