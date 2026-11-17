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

// ── 台账列表（云盘滚动加载版）──
// 后端 GET /api/registry?parent_id=&q=&limit=&offset=&is_dir= → {items,total}
//   parent_id 空 = 顶层；q 按名字/描述/标签包含匹配；limit 默认 50 最大 200；
//   is_dir 为 'true'/'false'，不传 = 都返回。
// 文件夹树单独走 GET /api/registry/folders（全部文件夹的扁平列表，量小，一次拉全）。

export interface EntryPage {
  items: Entry[]
  total: number
}

export interface EntryQuery {
  parentId?: string | null
  q?: string
  limit?: number
  offset?: number
  isDir?: boolean | null
}

export function listEntries(params: EntryQuery = {}): Promise<EntryPage> {
  const qs = new URLSearchParams()
  // 顶层用空串表示（契约写法 parent_id=），文件夹传 id
  qs.set('parent_id', params.parentId?.trim() || '')
  if (params.q?.trim()) qs.set('q', params.q.trim())
  if (params.limit !== undefined) qs.set('limit', String(params.limit))
  if (params.offset !== undefined) qs.set('offset', String(params.offset))
  if (params.isDir === true) qs.set('is_dir', 'true')
  else if (params.isDir === false) qs.set('is_dir', 'false')
  return request<EntryPage>(`${BASE}?${qs.toString()}`)
}

// 文件夹引用：树/面包屑/所属文件夹下拉只关心这三个字段
export interface FolderRef {
  id: string
  alias: string
  parentId?: string | null
}

export interface FolderPage {
  items: FolderRef[]
  total: number
}

export function listFolders(): Promise<FolderPage> {
  return request<FolderPage>(`${BASE}/folders`)
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

// ── 盘符在线状态（步骤5）：批量查盘符是否可访问 ──
// 后端 GET /api/fs/status?drives=D:,Z: → {items:[{drive,online,reason?}],total}
// 只查台账里出现过的盘符（≤26 个/次）；后端未实现时接口报错，前端按「未知」显示，不打扰用户。

export interface DriveStatus {
  drive: string
  online: boolean
  reason?: string
}

export interface FsStatusPage {
  items: DriveStatus[]
  total: number
}

export function fsStatus(drives: string[]): Promise<FsStatusPage> {
  const qs = new URLSearchParams()
  if (drives?.length) qs.set('drives', drives.join(','))
  const suffix = qs.toString()
  return request<FsStatusPage>(suffix ? `/api/fs/status?${suffix}` : '/api/fs/status')
}

// ── 标签字典（标签后端化）──
// 后端 GET /api/tags 一个接口两种用法，都返回 {items,total}：
//   分页/搜索：?q=&limit=&offset=（limit 默认 50，最大 200）
//   批量精确取：?names=a,b,c（≤100 个名字）
// 重名 409（{"error":"标签已存在"}）由 request() 抛成 Error。
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

// 列表/批量取色的统一响应
export interface TagPage {
  items: Tag[]
  total: number
}

export interface TagQuery {
  q?: string
  limit?: number
  offset?: number
  names?: string[]
}

const TAGS_BASE = '/api/tags'

export function listTags(params: TagQuery = {}): Promise<TagPage> {
  const qs = new URLSearchParams()
  if (params.q?.trim()) qs.set('q', params.q.trim())
  if (params.names?.length) qs.set('names', params.names.join(','))
  if (params.limit !== undefined) qs.set('limit', String(params.limit))
  if (params.offset !== undefined) qs.set('offset', String(params.offset))
  const suffix = qs.toString()
  const url = suffix ? TAGS_BASE + '?' + suffix : TAGS_BASE
  return request<TagPage>(url)
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
