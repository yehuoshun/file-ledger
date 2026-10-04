// API 层：与后端 internal/registry 模型对齐（批次1步骤2，PostgreSQL 版）
// 开发期走 vite proxy（/api → http://127.0.0.1:18765），生产 embed 后同源。

export type PathType = 'local' | 'mount' | 'virtual'

export interface Path {
  type: PathType
  path: string
}

export interface Entry {
  id: string
  alias: string
  parentId?: string
  isDir: boolean
  size: string
  note: string
  tags: string[]
  paths: Path[]
  createdAt?: string
  updatedAt?: string
}

export interface EntryForm {
  alias: string
  parentId?: string | null
  isDir: boolean
  size: string
  note: string
  tags: string[]
  paths: Path[]
}

const BASE = '/api/registry'

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (resp.status === 204) return undefined as T
  const data = await resp.json().catch(() => null)
  if (!resp.ok) {
    const msg = (data as { error?: string } | null)?.error || `请求失败（${resp.status}）`
    throw new Error(msg)
  }
  return data as T
}

export function listEntries(): Promise<Entry[]> {
  return request<Entry[]>(BASE)
}

export function createEntry(form: EntryForm): Promise<Entry> {
  return request<Entry>(BASE, { method: 'POST', body: JSON.stringify(form) })
}

export function updateEntry(id: string, form: EntryForm): Promise<Entry> {
  return request<Entry>(`${BASE}/${id}`, { method: 'PUT', body: JSON.stringify(form) })
}

export function deleteEntry(id: string): Promise<void> {
  return request<void>(`${BASE}/${id}`, { method: 'DELETE' })
}
