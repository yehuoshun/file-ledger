import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listEntries, createEntry, updateEntry, deleteEntry, type Entry, type EntryForm } from '../api'

// 台账数据源：加载 + 增改删（只管接口与数据，不碰界面状态）
export function useEntries() {
  const entries = ref<Entry[]>([])
  const loading = ref(false)

  async function reload() {
    loading.value = true
    try {
      entries.value = await listEntries()
    } catch (e: any) {
      ElMessage.error(e.message || '加载台账失败')
    } finally {
      loading.value = false
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

  return { entries, loading, reload, save, remove }
}
