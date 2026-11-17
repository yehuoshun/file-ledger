import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listEntries, createEntry, updateEntry, deleteEntry, type Entry, type EntryForm } from '../api'

// 云盘列表一屏条数（后端 limit 上限 200）
export const PAGE_SIZE = 50
const MAX_PAGE_SIZE = 200

// 列表查询条件：当前文件夹 + 搜索词 + 类型过滤。页面改这里的字段，再调 load(true) 回第一屏。
export interface EntryQueryState {
  parentId: string | null
  q: string
  isDir: boolean | null
}

// 台账数据源：分页累积（滚动加载）+ 增改删（只管接口与数据，不碰界面状态）
export function useEntries() {
  const items = ref<Entry[]>([])
  const total = ref(0)
  const loading = ref(false) // 第一屏（重置）加载中
  const loadingMore = ref(false) // 追加一屏加载中

  const query = reactive<EntryQueryState>({ parentId: null, q: '', isDir: null })

  const loadedCount = computed(() => items.value.length)
  const hasMore = computed(() => items.value.length < total.value)

  // 请求序号：切文件夹/改搜索时，旧请求的迟到响应直接丢弃，避免列表串味道
  let seq = 0

  function fetchPage(offset: number, limit: number) {
    return listEntries({ ...query, limit, offset })
  }

  // 拉一屏：reset=true 清空重来（回第一屏），false 追加到已加载的末尾
  async function load(reset = true) {
    if (!reset && (loading.value || loadingMore.value || !hasMore.value)) return
    const my = ++seq
    if (reset) {
      loading.value = true
      loadingMore.value = false
      items.value = []
      total.value = 0
    } else {
      loadingMore.value = true
    }
    try {
      const page = await fetchPage(reset ? 0 : items.value.length, PAGE_SIZE)
      if (my !== seq) return // 已有更新的请求，本次结果作废
      items.value = reset ? page.items ?? [] : items.value.concat(page.items ?? [])
      total.value = page.total ?? 0
    } catch (e: any) {
      if (my !== seq) return
      ElMessage.error(e.message || '加载台账失败')
    } finally {
      if (my === seq) {
        loading.value = false
        loadingMore.value = false
      }
    }
  }

  // 滚动到底追加一屏
  function loadMore() {
    return load(false)
  }

  // 保存/删除后重新拉：保留已加载的条数（上限 200），total 重新对齐，
  // 这样在已翻几屏的情况下新增/删除不会把用户弹回第一屏。
  async function reload() {
    const limit = Math.min(Math.max(items.value.length, PAGE_SIZE), MAX_PAGE_SIZE)
    const my = ++seq
    loading.value = true
    loadingMore.value = false
    try {
      const page = await fetchPage(0, limit)
      if (my !== seq) return
      items.value = page.items ?? []
      total.value = page.total ?? 0
    } catch (e: any) {
      if (my !== seq) return
      ElMessage.error(e.message || '加载台账失败')
    } finally {
      if (my === seq) {
        loading.value = false
        loadingMore.value = false
      }
    }
  }

  // id 为空 = 新增，否则更新
  async function save(id: string | null, form: EntryForm) {
    if (id) await updateEntry(id, form)
    else await createEntry(form)
    await reload()
  }

  async function remove(id: string) {
    await deleteEntry(id)
    await reload()
  }

  return {
    items,
    total,
    loading,
    loadingMore,
    loadedCount,
    hasMore,
    query,
    load,
    loadMore,
    reload,
    save,
    remove,
  }
}
