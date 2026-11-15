export type Status = 'backlog' | 'todo' | 'in_progress' | 'review' | 'done'
export type Priority = 'low' | 'medium' | 'high' | 'critical'

export interface TaskSummary {
  id: string
  title: string
  project_id: string
  status: Status
}

export interface Task {
  id: string
  project_id: string
  sprint_id: number | null
  title: string
  description: string
  status: Status
  priority: Priority
  points: number | null
  assignee_id: string | null
  branch: string
  pr_number: number | null
  pr_merged: boolean
  labels: string[]
  blockers: string[] | null
  blocking: string[] | null
  blocker_details?: TaskSummary[] | null
  blocking_details?: TaskSummary[] | null
  created_at: string
  updated_at: string
  notes?: Note[]
  artifacts?: Artifact[]
}

export type ArtifactKind = 'script' | 'log' | 'image' | 'audio' | 'video' | 'document' | 'other'

export interface Artifact {
  id: number
  project_id: string
  task_id: string | null
  kind: ArtifactKind
  name: string
  media_type?: string
  sha256: string
  size: number
  note?: string
  origin: string
  created_by?: string
  created_at: string
  where: 'here' | 'remote' | 'both' | 'unavailable'
}

export interface Note {
  id: number
  task_id: string
  author_id?: string
  body: string
  created_at: string
}

export interface Project {
  id: string
  name: string
  repo_path: string
  repo_dir?: string
  repo_remote?: string
  artifact_policy?: '' | 'remote' | 'local' | 'off'
  sprint_mode: boolean
  sync_origin?: string
  sync_status?: string
}

export interface User {
  id: string
  display_name: string
  is_agent: boolean
}

export interface Sprint {
  id: number
  project_id: string
  name: string
  start_date: string | null
  end_date: string | null
  status: 'planning' | 'active' | 'closed'
}

export interface SprintSummary {
  sprint: Sprint
  committed: number
  completed: number
  task_count: number
}

export interface CreateTaskInput {
  title: string
  description?: string
  project_id: string
  assignee_id?: string
  priority?: Priority
  status?: Status
  points?: number
  labels?: string[]
  sprint_id?: number
}

export interface UpdateTaskInput {
  title?: string
  description?: string
  status?: Status
  priority?: Priority
  assignee_id?: string
  points?: number
  labels?: string[]
  sprint_id?: number
}

export interface Secondary {
  id: string
  display_name: string
  last_seen_at?: string
  created_at: string
}
