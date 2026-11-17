import { ref } from 'vue'
import { fsStatus } from '../api'

// 盘符在线状态缓存（后端 /api/fs/status）：模块级单例，表格多处共享同一份「盘符 → 状态」。
//   - ensureDrives 按需批量补：只查缓存缺失/过期的，≤26 个一批并发，多处并发调用自动合流
//   - onlineOf 同步读缓存：undefined = 还没查到（界面不显示状态点），true/false = 在线/离线
//   - 接口失败（如后端还没实现该路由 → 404）静默降级：状态保持未知，不弹错、不影响列表渲染
// 状态带 TTL：拔插盘后最多 30 秒再重查一次。

// 在线状态的有效期
const TTL = 30_000
// 单次请求的盘符上限（盘符本来最多 26 个，与后端 drives 参数上限一致）
const CHUNK = 26

export interface DriveState {
  online: boolean
  reason: string
  at: number
}

// 盘符 → 状态（整对象替换，保证响应式）
const cache = ref<Record<string, DriveState>>({})
// 正在请求中的盘符，同一批不重复打接口
const inflight = new Set<string>()
// 待查队列：多处并发 ensureDrives 合并成一次 drain
const queued = new Set<string>()
// 查询失败的盘符：TTL 内不再重试（否则每次列表变化都会打一次失败接口）
const failedAt = new Map<string, number>()
let draining: Promise<void> | null = null

const error = ref<string | null>(null)

// 只认 "D:" 这种形态；别的一律丢掉
function normDrive(drive: string): string {
  const d = (drive || '').trim().toUpperCase()
  return /^[A-Z]:$/.test(d) ? d : ''
}

// 挑出真正需要请求的盘符：已缓存且未过期 / 请求中 / 队列中 / 刚失败过的都跳过
function pending(drives: string[]): string[] {
  const out: string[] = []
  const now = Date.now()
  for (const raw of drives) {
    const d = normDrive(raw)
    if (!d || inflight.has(d) || queued.has(d)) continue
    const c = cache.value[d]
    if (c && now - c.at < TTL) continue
    const f = failedAt.get(d)
    if (f !== undefined && now - f < TTL) continue
    out.push(d)
  }
  return out
}

async function fetchChunk(drives: string[]): Promise<void> {
  try {
    const page = await fsStatus(drives)
    const items = page?.items ?? []
    const now = Date.now()
    const next = { ...cache.value }
    const seen = new Set<string>()
    for (const it of items) {
      const d = normDrive(it.drive)
      if (!d) continue
      seen.add(d)
      next[d] = { online: !!it.online, reason: it.reason || '', at: now }
      failedAt.delete(d)
    }
    cache.value = next
    error.value = null
    // 后端没返回的盘符按「未知」处理：TTL 内别反复问
    for (const d of drives) {
      if (!seen.has(d)) failedAt.set(d, now)
    }
  } catch (e) {
    // 静默降级：状态保持未知，界面不显示状态点，列表照常渲染
    const now = Date.now()
    for (const d of drives) failedAt.set(d, now)
    error.value = (e as { message?: string } | null)?.message || '查询盘符状态失败'
  } finally {
    for (const d of drives) inflight.delete(d)
  }
}

async function drain(): Promise<void> {
  try {
    while (queued.size) {
      const batch = [...queued]
      queued.clear()
      for (const d of batch) inflight.add(d)
      const chunks: string[][] = []
      for (let i = 0; i < batch.length; i += CHUNK) chunks.push(batch.slice(i, i + CHUNK))
      await Promise.all(chunks.map((c) => fetchChunk(c)))
    }
  } finally {
    draining = null
  }
}

// 按需补状态：只请求缓存里没有/过期的盘符，并发调用合流成一批
async function ensureDrives(drives: string[]): Promise<void> {
  const add = pending(drives)
  if (!add.length) return
  for (const d of add) queued.add(d)
  if (!draining) draining = drain()
  const run = draining
  if (run) await run
}

function infoOf(drive: string): DriveState | undefined {
  const d = normDrive(drive)
  return d ? cache.value[d] : undefined
}

// 同步读：undefined = 未知（还没查过 / 查失败）
function onlineOf(drive: string): boolean | undefined {
  const s = infoOf(drive)
  return s ? s.online : undefined
}

export function useDriveStatus() {
  return { ensureDrives, infoOf, onlineOf, error }
}
