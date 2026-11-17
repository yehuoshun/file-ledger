import { computed, ref } from 'vue'
import { listTags, createTag, updateTag, deleteTag, type Tag } from '../api'

// 标签字典（后端 /api/tags）：模块级单例。
// 多个 TagSelector / 台账表格共享同一份 tags，只拉一次接口，不随组件实例重复请求。
const tags = ref<Tag[]>([])
const loading = ref(false)
const loaded = ref(false)
const error = ref<string | null>(null)
// 并发合并：多个组件同时喊 reload 时复用同一个请求
let inflight: Promise<void> | null = null

// 字典里没有的名字（例如刚在表单里手输、后端还没登记）用中性灰兜底
const FALLBACK_COLOR = { bg: '#e4e7ed', fg: '#303133' }

function upsert(t: Tag) {
  const i = tags.value.findIndex((x) => x.id === t.id)
  tags.value = i >= 0 ? tags.value.map((x) => (x.id === t.id ? t : x)) : [...tags.value, t]
}

function readError(e: unknown, fallback: string): string {
  const msg = (e as { message?: string } | null)?.message
  return msg || fallback
}

// 取数失败不抛给组件：组件照常渲染（颜色走兜底灰），错误只记录在 error 上
async function reload(): Promise<void> {
  if (inflight) return inflight
  loading.value = true
  error.value = null
  const p = (async () => {
    try {
      tags.value = await listTags()
      loaded.value = true
    } catch (e) {
      error.value = readError(e, '加载标签失败')
    } finally {
      loading.value = false
      inflight = null
    }
  })()
  inflight = p
  return p
}

function colorOf(name: string): { bg: string; fg: string } {
  const t = tags.value.find((x) => x.name === name)
  return t ? { bg: t.bg, fg: t.fg } : { ...FALLBACK_COLOR }
}

// 下面这些写操作把错误抛给调用方（设置页要弹后端文案），同时记到 error 上
async function addTag(name: string, color: { bg?: string; fg?: string } = {}): Promise<Tag | null> {
  const n = name.trim()
  if (!n) return null
  try {
    const t = await createTag({ name: n, ...color })
    upsert(t)
    return t
  } catch (e) {
    error.value = readError(e, '新增标签失败')
    throw e
  }
}

async function renameTag(id: string, name: string): Promise<Tag | null> {
  const n = name.trim()
  if (!n) return null
  try {
    const t = await updateTag(id, { name: n })
    upsert(t)
    return t
  } catch (e) {
    error.value = readError(e, '重命名标签失败')
    throw e
  }
}

async function recolorTag(id: string, bg: string, fg: string): Promise<Tag | null> {
  try {
    const t = await updateTag(id, { bg, fg })
    upsert(t)
    return t
  } catch (e) {
    error.value = readError(e, '修改标签颜色失败')
    throw e
  }
}

async function removeTag(id: string): Promise<void> {
  try {
    await deleteTag(id)
    tags.value = tags.value.filter((x) => x.id !== id)
  } catch (e) {
    error.value = readError(e, '删除标签失败')
    throw e
  }
}

const tagNames = computed(() => tags.value.map((t) => t.name))

export function useTags() {
  return {
    tags,
    loading,
    loaded,
    error,
    colorOf,
    reload,
    addTag,
    renameTag,
    recolorTag,
    removeTag,
    tagNames,
  }
}
