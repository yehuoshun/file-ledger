import { ref } from 'vue'
import { listTags, createTag, updateTag, deleteTag, type Tag } from '../api'

// 标签字典缓存（后端 /api/tags）：模块级单例，多处共享同一份「名字 → 标签」缓存。
// 标签可能有上千条，不再一次性拉全表：
//   - 选择器用 searchTags 按关键字现查
//   - 表格用 ensureColors 按需批量补颜色（每次 ≤100 个名字，并发请求自动合并）
//   - 设置页用 fetchPage 走服务端分页
// colorOf 同步读缓存：没取到先给兜底灰，缓存更新后界面自动重绘。
// 写操作（改名/改色/删除）会失效相关缓存，调用方随后 ensureColors 即可重新取到。

// 字典里没有的名字（刚手输、后端还没登记）用中性灰兜底
const FALLBACK_COLOR = { bg: '#e4e7ed', fg: '#303133' }
// 批量取色的单次上限（与后端 names 参数上限一致）
const NAMES_CHUNK = 100
// 选择器默认取多少条候选
const SEARCH_LIMIT = 20
// 「字典里没有」的结论只保留这么久，过期后允许重查（新标签可能是刚登记的）
const MISSING_TTL = 60_000

export interface TagColor {
  bg: string
  fg: string
}

// 名字 → 标签（整对象替换，保证响应式）
const cache = ref<Record<string, Tag>>({})
// 名字 → 确认「字典里没有」的时间戳
const missingAt = new Map<string, number>()
// 正在请求中的名字，同一批不重复打接口
const inflight = new Set<string>()
// 待取色的名字队列：多处并发 ensureColors 合并成一次 drain
const queued = new Set<string>()
let draining: Promise<void> | null = null

const error = ref<string | null>(null)

function readError(e: unknown, fallback: string): string {
  const msg = (e as { message?: string } | null)?.message
  return msg || fallback
}

// 把取到的标签并进缓存（同时清掉「字典里没有」的记录）
function mergeTags(items: Tag[]) {
  if (!items.length) return
  const next = { ...cache.value }
  for (const t of items) {
    next[t.name] = t
    missingAt.delete(t.name)
  }
  cache.value = next
}

// 让指定名字的缓存失效（改名/删除后旧名字不该再有颜色）
function invalidate(names: string[]) {
  if (!names.length) return
  const next = { ...cache.value }
  for (const n of names) {
    delete next[n]
    missingAt.delete(n)
  }
  cache.value = next
}

// 同步取色：缓存里没有就先给兜底灰（外部再调 ensureColors 补）
function colorOf(name: string): TagColor {
  const t = cache.value[name]
  return t ? { bg: t.bg, fg: t.fg } : { ...FALLBACK_COLOR }
}

function findNameById(id: string): string | null {
  for (const [name, t] of Object.entries(cache.value)) {
    if (t.id === id) return name
  }
  return null
}

// 挑出真正需要请求的名字：已缓存/请求中/队列中/TTL 内确认没有的都不再查
function pendingNames(names: string[]): string[] {
  const out: string[] = []
  const now = Date.now()
  for (const raw of names) {
    const n = raw.trim()
    if (!n || cache.value[n] || inflight.has(n) || queued.has(n)) continue
    const at = missingAt.get(n)
    if (at !== undefined) {
      if (now - at < MISSING_TTL) continue
      missingAt.delete(n)
    }
    out.push(n)
  }
  return out
}

async function fetchNamesChunk(names: string[]): Promise<void> {
  try {
    const page = await listTags({ names })
    mergeTags(page.items)
    const found = new Set(page.items.map((t) => t.name))
    const now = Date.now()
    for (const n of names) {
      if (!found.has(n)) missingAt.set(n, now)
    }
  } catch (e) {
    // 取色失败不打断渲染：颜色走兜底灰，下次 ensureColors 会重试
    error.value = readError(e, '加载标签颜色失败')
  } finally {
    for (const n of names) inflight.delete(n)
  }
}

async function drain(): Promise<void> {
  try {
    while (queued.size) {
      const batch = [...queued]
      queued.clear()
      for (const n of batch) inflight.add(n)
      const chunks: string[][] = []
      for (let i = 0; i < batch.length; i += NAMES_CHUNK) chunks.push(batch.slice(i, i + NAMES_CHUNK))
      await Promise.all(chunks.map((c) => fetchNamesChunk(c)))
    }
  } finally {
    draining = null
  }
}

// 按需补颜色：只请求缓存里没有的名字，≤100 个一批并发拉，并发调用合流
async function ensureColors(names: string[]): Promise<void> {
  const add = pendingNames(names)
  if (!add.length) return
  for (const n of add) queued.add(n)
  if (!draining) draining = drain()
  const run = draining
  if (run) await run
}

// 选择器用：按关键字搜索（空关键字 = 前 limit 条），失败返回空列表不抛
async function searchTags(q: string, limit = SEARCH_LIMIT): Promise<Tag[]> {
  try {
    const page = await listTags({ q: q.trim(), limit, offset: 0 })
    mergeTags(page.items)
    return page.items
  } catch (e) {
    error.value = readError(e, '搜索标签失败')
    return []
  }
}

// 设置页用：服务端分页（page 从 1 起）；失败抛给调用方弹提示
async function fetchPage(
  q: string,
  page: number,
  pageSize: number,
): Promise<{ items: Tag[]; total: number }> {
  const size = Math.max(1, Math.floor(pageSize) || 1)
  const p = Math.max(1, Math.floor(page) || 1)
  const data = await listTags({ q: q.trim(), limit: size, offset: (p - 1) * size })
  mergeTags(data.items)
  return { items: data.items, total: data.total }
}

// 写操作把错误抛给调用方（设置页要弹后端文案），同时记到 error 上
async function addTag(name: string, color: { bg?: string; fg?: string } = {}): Promise<Tag | null> {
  const n = name.trim()
  if (!n) return null
  try {
    const t = await createTag({ name: n, ...color })
    mergeTags([t])
    return t
  } catch (e) {
    error.value = readError(e, '新增标签失败')
    throw e
  }
}

async function renameTag(id: string, name: string): Promise<Tag | null> {
  const n = name.trim()
  if (!n) return null
  const prev = findNameById(id)
  try {
    const t = await updateTag(id, { name: n })
    // 旧名字失效：条目上还留着旧标签名，颜色回到兜底灰
    if (prev && prev !== t.name) invalidate([prev])
    mergeTags([t])
    return t
  } catch (e) {
    error.value = readError(e, '重命名标签失败')
    throw e
  }
}

async function recolorTag(id: string, bg: string, fg: string): Promise<Tag | null> {
  try {
    const t = await updateTag(id, { bg, fg })
    mergeTags([t])
    return t
  } catch (e) {
    error.value = readError(e, '修改标签颜色失败')
    throw e
  }
}

async function removeTag(id: string): Promise<void> {
  const prev = findNameById(id)
  try {
    await deleteTag(id)
    if (prev) invalidate([prev])
  } catch (e) {
    error.value = readError(e, '删除标签失败')
    throw e
  }
}

export function useTags() {
  return {
    colorOf,
    ensureColors,
    searchTags,
    fetchPage,
    invalidate,
    addTag,
    renameTag,
    recolorTag,
    removeTag,
    error,
  }
}
