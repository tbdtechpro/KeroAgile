import { useState } from 'react'
import { useProjects } from '../api/queries'
import Board from '../components/Board'

export default function BoardPage() {
  const { data: projects = [], isLoading } = useProjects()
  const [selectedProject, setSelectedProject] = useState<string | null>(null)

  const currentProject = selectedProject ?? projects[0]?.id ?? null

  if (isLoading) {
    return <div className="p-8 text-sm" style={{ color: 'var(--ka-muted)' }}>Loading…</div>
  }

  if (projects.length === 0) {
    return (
      <div className="p-8">
        <p style={{ color: 'var(--ka-muted)' }}>No projects found.</p>
        <p className="text-sm mt-2" style={{ color: 'var(--ka-muted)' }}>
          Create one with: <code className="px-1 py-0.5 rounded" style={{ background: '#1e293b' }}>
            KeroAgile project add MYAPP "My App"
          </code>
        </p>
      </div>
    )
  }

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 48px)' }}>
      {/* Project tabs */}
      <div className="flex gap-1 px-4 pt-2 border-b" style={{ borderColor: '#1e293b' }}>
        {projects.map(p => (
          <button
            key={p.id}
            onClick={() => setSelectedProject(p.id)}
            className="px-3 py-1 text-sm rounded-t transition-colors"
            style={{
              background: p.id === currentProject ? 'var(--ka-surface)' : 'transparent',
              color: p.id === currentProject ? 'var(--ka-accent-lt)' : 'var(--ka-muted)',
              borderBottom: p.id === currentProject ? '2px solid var(--ka-accent)' : '2px solid transparent',
            }}
          >
            {p.id}
          </button>
        ))}
      </div>

      {currentProject && <Board projectId={currentProject} />}
    </div>
  )
}
