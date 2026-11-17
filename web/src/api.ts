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

// ── 本地文件系统浏览（批次步骤4）：只读目录列表 ──
// 后端 GET /api/fs/list?path=<绝对路径>：path 指目录则列该目录，指文件则自动列其所在目录，
// 响应里的 path 是实际被列出的目录，parent 是上一级（盘符根的 parent 为空串）。

export interface FsEntry {
  name: string
  isDir: boolean
  size: number
  modTime: string
  ext: string
}

export interface FsListing {
  path: string
  parent: string
  entries: FsEntry[]
  total: number
  truncated: boolean
}

export function listDir(path: string): Promise<FsListing> {
  return request<FsListing>(`/api/fs/list?path=${encodeURIComponent(path)}`)
}

// ── 标签字典（标签后端化）──
// 后端 GET/POST/PUT/DELETE /api/tags；重名 409（{"error":"标签已存在"}）由 request() 抛成 Error。
// 保存条目时后端会自动把没见过的标签名登记进字典，所以表单里直接填新标签也能落库。

export interface Tag {
  id: string
  name: string
  bg: string
  fg: string
  createdAt?: string
}

// 新增/更新都只传需要改的字段（PUT 是部分更新）
export interface TagInput {
  name?: string
  bg?: string
  fg?: string
}

const TAGS_BASE = '/api/tags'

export function listTags(): Promise<Tag[]> {
  return request<Tag[]>(TAGS_BASE)
}

export function createTag(input: TagInput & { name: string }): Promise<Tag> {
  return request<Tag>(TAGS_BASE, { method: 'POST', body: JSON.stringify(input) })
}

export function updateTag(id: string, input: TagInput): Promise<Tag> {
  return request<Tag>(`${TAGS_BASE}/${id}`, { method: 'PUT', body: JSON.stringify(input) })
}

export function deleteTag(id: string): Promise<void> {
  return request<void>(`${TAGS_BASE}/${id}`, { method: 'DELETE' })
}
