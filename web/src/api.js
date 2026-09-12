// API 封装：与后端 REST 边界一一对应（见 doc/03-架构设计.md）
// USE_MOCK = true 时走本地 mock（后端未就绪时前端可独立开发）
// 后端就绪后改为 false 即可无缝切换，接口签名完全一致

const USE_MOCK = true

// ---------- mock 数据（结构与后端 Entry 一致） ----------
let mockEntries = [
  {
    id: '1',
    alias: 'z',
    type: 'mount',
    root: 'Z:',
    rel_path: '',
    jump_url: '',
    note: '挂载盘',
    created_at: '2026-09-13T06:00:00+08:00',
    updated_at: '2026-09-13T06:00:00+08:00',
  },
  {
    id: '2',
    alias: 'games',
    type: 'local',
    root: 'D:\\Games',
    rel_path: '',
    jump_url: '',
    note: '本地游戏目录',
    created_at: '2026-09-13T06:00:00+08:00',
    updated_at: '2026-09-13T06:00:00+08:00',
  },
  {
    id: '3',
    alias: 'ali',
    type: 'virtual',
    root: '',
    rel_path: 'game/rpg',
    jump_url: 'https://www.aliyundrive.com/xxx',
    note: '阿里云盘虚拟路径',
    created_at: '2026-09-13T06:00:00+08:00',
    updated_at: '2026-09-13T06:00:00+08:00',
  },
]

// ---------- mock 实现 ----------
const mockDelay = (ms = 200) => new Promise((r) => setTimeout(r, ms))

const mockApi = {
  async list() {
    await mockDelay()
    return [...mockEntries].sort((a, b) => a.alias.localeCompare(b.alias))
  },
  async create(entry) {
    await mockDelay()
    if (mockEntries.some((e) => e.alias === entry.alias)) {
      throw new Error('别名已存在')
    }
    const now = new Date().toISOString()
    const created = {
      ...entry,
      id: String(Date.now()),
      created_at: now,
      updated_at: now,
    }
    mockEntries.push(created)
    return created
  },
  async update(id, entry) {
    await mockDelay()
    const idx = mockEntries.findIndex((e) => e.id === id)
    if (idx === -1) throw new Error('台账不存在')
    if (mockEntries.some((e) => e.alias === entry.alias && e.id !== id)) {
      throw new Error('别名已存在')
    }
    mockEntries[idx] = {
      ...mockEntries[idx],
      ...entry,
      updated_at: new Date().toISOString(),
    }
    return mockEntries[idx]
  },
  async remove(id) {
    await mockDelay()
    mockEntries = mockEntries.filter((e) => e.id !== id)
    return { ok: true }
  },
}

// ---------- 真实 API（fetch） ----------
async function http(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    // 后端统一错误格式：{"error": "中文消息"}
    throw new Error(data.error || `请求失败 (${res.status})`)
  }
  return data
}

const realApi = {
  list: () => http('GET', '/api/registry'),
  create: (entry) => http('POST', '/api/registry', entry),
  update: (id, entry) => http('PUT', `/api/registry/${id}`, entry),
  remove: (id) => http('DELETE', `/api/registry/${id}`),
}

export const api = USE_MOCK ? mockApi : realApi
