import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type { CreateTaskInput, UpdateTaskInput } from './types'

export const keys = {
  projects: () => ['projects'] as const,
  tasks: (params?: object) => ['tasks', params] as const,
  task: (id: string) => ['tasks', id] as const,
  users: () => ['users'] as const,
  sprints: (projectId?: string) => ['sprints', projectId] as const,
}

export function useProjects() {
  return useQuery({ queryKey: keys.projects(), queryFn: api.listProjects })
}

export function useTasks(params?: { project_id?: string; status?: string; assignee_id?: string; sprint_id?: number }) {
  return useQuery({
    queryKey: keys.tasks(params),
    queryFn: () => api.listTasks(params),
  })
}

export function useTask(id: string) {
  return useQuery({ queryKey: keys.task(id), queryFn: () => api.getTask(id) })
}

export function useUsers() {
  return useQuery({ queryKey: keys.users(), queryFn: api.listUsers })
}

export function useSprints(projectId?: string) {
  return useQuery({
    queryKey: keys.sprints(projectId),
    queryFn: () => api.listSprints(projectId),
  })
}

export function useCreateTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateTaskInput) => api.createTask(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useUpdateTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateTaskInput }) => api.updateTask(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useMoveTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => api.moveTask(id, status),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}

export function useDeleteTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteTask(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tasks'] }),
  })
}
