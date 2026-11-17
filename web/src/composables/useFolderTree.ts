import { computed, type Ref } from 'vue'
import type { Entry } from '../api'

export interface TreeNode {
  id: string
  alias: string
  children: TreeNode[]
}

// 台账的树形派生：树结构、子孙计数、层级下拉选项（纯计算，不持有状态）
export function useFolderTree(entries: Ref<Entry[]>) {
  const pid = (e: Entry | null | undefined) => e?.parentId ?? null

  function buildTree(parentId: string | null = null): TreeNode[] {
    return entries.value
      .filter((e) => e.isDir && pid(e) === parentId)
      .map((e) => ({ id: e.id, alias: e.alias, children: buildTree(e.id) }))
  }

  const treeData = computed(() => buildTree(null))

  // 文件夹级联删子（Windows 语义）：子条目总数
  function countDescendants(id: string): number {
    let n = 0
    const stack = [id]
    while (stack.length) {
      const p = stack.pop()!
      for (const e of entries.value) {
        if (pid(e) === p) {
          n++
          if (e.isDir) stack.push(e.id)
        }
      }
    }
    return n
  }

  function isDescendantOf(id: string, ancestorId: string): boolean {
    let cur = entries.value.find((e) => e.id === id)
    while (cur?.parentId) {
      if (cur.parentId === ancestorId) return true
      cur = entries.value.find((e) => e.id === cur!.parentId)
    }
    return false
  }

  // 所属文件夹下拉选项：扁平「层级路径」，excludeId 及其子孙被排除（防把自己挂进自己的子树）
  function folderOptions(excludeId: string | null): { id: string; label: string }[] {
    const out: { id: string; label: string }[] = []
    const walk = (parent: string | null, prefix: string) => {
      const kids = entries.value
        .filter((e) => e.isDir && pid(e) === parent)
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

  return { pid, buildTree, treeData, countDescendants, isDescendantOf, folderOptions }
}
