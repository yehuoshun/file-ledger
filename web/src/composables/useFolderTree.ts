import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listFolders, type FolderRef } from '../api'

export interface TreeNode {
  id: string
  alias: string
  children: TreeNode[]
}

// 文件夹树的数据源 + 树形派生（树结构、子孙计数、层级下拉选项）。
// 云盘列表改分页后，树不能再用「当前页的条目」拼：单独拉全部文件夹（量小，一次拉全）。
export function useFolderTree() {
  const folders = ref<FolderRef[]>([])
  const loading = ref(false)

  async function loadFolders() {
    loading.value = true
    try {
      const page = await listFolders()
      folders.value = page.items ?? []
    } catch (e: any) {
      ElMessage.error(e.message || '加载文件夹失败')
    } finally {
      loading.value = false
    }
  }

  const pid = (f: FolderRef | null | undefined) => f?.parentId ?? null

  const folderMap = computed(() => {
    const m = new Map<string, FolderRef>()
    for (const f of folders.value) m.set(f.id, f)
    return m
  })

  // 面包屑/上溯用：按 id 取文件夹（拿不到 = 已删除或还没加载）
  function findFolder(id: string): FolderRef | undefined {
    return folderMap.value.get(id)
  }

  function buildTree(parentId: string | null = null): TreeNode[] {
    return folders.value
      .filter((f) => pid(f) === parentId)
      .map((f) => ({ id: f.id, alias: f.alias, children: buildTree(f.id) }))
  }

  const treeData = computed(() => buildTree(null))

  // 文件夹级联删子（Windows 语义）：其下子孙文件夹总数（文件不在文件夹表里，不计）
  function countDescendants(id: string): number {
    let n = 0
    const stack = [id]
    while (stack.length) {
      const p = stack.pop()!
      for (const f of folders.value) {
        if (pid(f) === p) {
          n++
          stack.push(f.id)
        }
      }
    }
    return n
  }

  function isDescendantOf(id: string, ancestorId: string): boolean {
    let cur = folderMap.value.get(id)
    while (cur?.parentId) {
      if (cur.parentId === ancestorId) return true
      cur = folderMap.value.get(cur.parentId)
    }
    return false
  }

  // 所属文件夹下拉选项：扁平「层级路径」，excludeId 及其子孙被排除（防把自己挂进自己的子树）
  function folderOptions(excludeId: string | null): { id: string; label: string }[] {
    const out: { id: string; label: string }[] = []
    const walk = (parent: string | null, prefix: string) => {
      const kids = folders.value
        .filter((f) => pid(f) === parent)
        .sort((a, b) => a.alias.localeCompare(b.alias, 'zh'))
      for (const k of kids) {
        if (excludeId && (k.id === excludeId || isDescendantOf(k.id, excludeId))) continue
        const label = prefix ? `${prefix} / ${k.alias}` : k.alias
        out.push({ id: k.id, label })
        walk(k.id, label)
      }
    }
    walk(null, '')
    return out
  }

  return {
    folders,
    loading,
    loadFolders,
    findFolder,
    pid,
    buildTree,
    treeData,
    countDescendants,
    isDescendantOf,
    folderOptions,
  }
}
