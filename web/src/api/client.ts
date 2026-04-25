import type { Task, Project, User, Sprint, SprintSummary, CreateTaskInput, UpdateTaskInput } from './types'

const TOKEN_KEY = 'keroagile_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    clearToken()
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Auth
  login(userId: string, password: string): Promise<{ token: string }> {
    return request('POST', '/api/auth/login', { user_id: userId, password })
  },

  // Projects
  listProjects(): Promise<Project[]> {
    return request('GET', '/api/projects')
  },
  createProject(id: string, name: string, repoPath?: string): Promise<Project> {
    return request('POST', '/api/projects', { id, name, repo_path: repoPath })
  },

  // Tasks
  listTasks(params?: { project_id?: string; status?: string; assignee_id?: string; sprint_id?: number }): Promise<Task[]> {
    const qs = new URLSearchParams()
    if (params?.project_id) qs.set('project_id', params.project_id)
    if (params?.status) qs.set('status', params.status)
    if (params?.assignee_id) qs.set('assignee_id', params.assignee_id)
    if (params?.sprint_id) qs.set('sprint_id', String(params.sprint_id))
    return request('GET', `/api/tasks?${qs}`)
  },
  getTask(id: string): Promise<Task> {
    return request('GET', `/api/tasks/${id}`)
  },
  createTask(input: CreateTaskInput): Promise<Task> {
    return request('POST', '/api/tasks', input)
  },
  updateTask(id: string, input: UpdateTaskInput): Promise<Task> {
    return request('PATCH', `/api/tasks/${id}`, input)
  },
  moveTask(id: string, status: string): Promise<Task> {
    return request('PATCH', `/api/tasks/${id}`, { status })
  },
  deleteTask(id: string): Promise<void> {
    return request('DELETE', `/api/tasks/${id}`)
  },

  // Users
  listUsers(): Promise<User[]> {
    return request('GET', '/api/users')
  },

  // Sprints
  listSprints(projectId?: string): Promise<SprintSummary[]> {
    const qs = projectId ? `?project_id=${projectId}` : ''
    return request('GET', `/api/sprints${qs}`)
  },
  getSprint(id: number): Promise<Sprint> {
    return request('GET', `/api/sprints/${id}`)
  },
  createSprint(name: string, projectId: string, startDate?: string, endDate?: string): Promise<Sprint> {
    return request('POST', '/api/sprints', { name, project_id: projectId, start_date: startDate, end_date: endDate })
  },
}
