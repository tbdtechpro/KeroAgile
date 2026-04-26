export type Status = 'backlog' | 'todo' | 'in_progress' | 'review' | 'done'
export type Priority = 'low' | 'medium' | 'high' | 'critical'

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
  created_at: string
  updated_at: string
}

export interface Project {
  id: string
  name: string
  repo_path: string
  sprint_mode: boolean
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
