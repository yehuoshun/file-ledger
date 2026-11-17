<template>
  <div class="cloud-layout">
    <!-- 左侧：文件夹树 -->
    <FolderTreePanel ref="treeRef" :data="treeData" @select="enterFolder" @add="openAdd(null)" />

    <!-- 右侧：当前文件夹内容 -->
    <main class="content-panel">
      <div class="toolbar">
        <div class="toolbar-left">
          <FolderBreadcrumbs :crumbs="breadcrumbs" @navigate="enterFolder" />
          <!-- 云端列表是滚动加载，总数只能用后端给的 total（不是已加载条数） -->
          <span class="total-hint">共 {{ total }} 条</span>
        </div>

        <div class="toolbar-actions">
          <el-select v-model="dirFilter" placeholder="类型" clearable size="default" style="width: 110px">
            <el-option :value="true" label="文件夹" />
            <el-option :value="false" label="文件" />
          </el-select>

          <el-input
            v-model="search"
            placeholder="搜索名字或描述…"
            clearable
            size="default"
            class="search-input"
          />

          <el-button type="primary" size="default" @click="openAdd(currentFolder)">+ 新增</el-button>
        </div>
      </div>

      <EntryTable
        ref="tableRef"
        :rows="items"
        :loading="loading"
        :loading-more="loadingMore"
        :total="total"
        :has-more="hasMore"
        :empty-text="emptyText"
        @load-more="loadMore"
        @row-dblclick="onRowDblclick"
        @edit="openEdit"
        @delete="handleDelete"
        @browse="openBrowser"
        @copy-path="copyPath"
        @reveal="revealPath"
      />
    </main>

    <!-- 新增/编辑弹窗 -->
    <EntryDialog
      v-model="dialogVisible"
      :mode="dialogMode"
      :entry="editingEntry"
      :default-parent-id="defaultParentId"
      :folder-options="folderOpts"
      @submit="onDialogSubmit"
    />

    <!-- 目录浏览器（步骤4）：从台账路径只读浏览本地目录 -->
    <DirBrowser v-model="browserVisible" :path="browserPath" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import FolderTreePanel from '../components/cloud/FolderTreePanel.vue'
import FolderBreadcrumbs from '../components/cloud/FolderBreadcrumbs.vue'
import EntryTable from '../components/cloud/EntryTable.vue'
import EntryDialog from '../components/cloud/EntryDialog.vue'
import DirBrowser from '../components/DirBrowser.vue'
import { useEntries } from '../composables/useEntries'
import { useFolderTree } from '../composables/useFolderTree'
import { canBrowsePath, driveOf, parentOf } from '../utils/paths'
import { useDriveStatus } from '../composables/useDriveStatus'
import type { Entry, EntryForm } from '../api'

// ── 数据与派生（全部逻辑在 composable 里，本组件只做编排）──
// 列表：分页累积（滚动加载）；文件夹树：单独拉全部文件夹（量小）
const { items, total, loading, loadingMore, hasMore, query, load, loadMore, save, remove } = useEntries()
const { treeData, loadFolders, countDescendants, folderOptions, findFolder } = useFolderTree()

const tableRef = ref<InstanceType<typeof EntryTable> | null>(null)
const treeRef = ref<InstanceType<typeof FolderTreePanel> | null>(null)

onMounted(() => {
  load(true)
  loadFolders()
})

// ── 当前文件夹与树高亮 ──
const currentFolder = ref<string | null>(null)

// 回第一屏：列表回到顶部 + 重新拉第一屏（切文件夹/改搜索/改类型过滤都走这里）
function resetList() {
  tableRef.value?.scrollToTop()
  load(true)
}

// 进入某个文件夹（Windows 语义：双击文件夹 = 进入）；id 为 null 表示回到顶层
function enterFolder(id: string | null) {
  currentFolder.value = id
  query.parentId = id
  nextTick(() => treeRef.value?.setCurrentKey(id))
  resetList()
}

// ── 面包屑：从 folders 按 parentId 上溯（列表已分页，不能再从当前页取父级）──
const breadcrumbs = computed(() => {
  const chain: { id: string | null; label: string }[] = [{ id: null, label: '虚拟网盘' }]
  if (!currentFolder.value) return chain
  const stack: { id: string; label: string }[] = []
  let cur = findFolder(currentFolder.value)
  while (cur) {
    stack.unshift({ id: cur.id, label: cur.alias })
    cur = cur.parentId ? findFolder(cur.parentId) : undefined
  }
  return chain.concat(stack)
})

// ── 右侧列表：搜索词与类型过滤（都走后端，改了就回第一屏）──
const search = ref('')
const dirFilter = ref<boolean | null>(null)

const SEARCH_DEBOUNCE = 300
let searchTimer: ReturnType<typeof setTimeout> | undefined

watch(search, () => {
  if (searchTimer !== undefined) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    searchTimer = undefined
    query.q = search.value.trim()
    resetList()
  }, SEARCH_DEBOUNCE)
})

watch(dirFilter, () => {
  query.isDir = dirFilter.value
  resetList()
})

onUnmounted(() => {
  if (searchTimer !== undefined) clearTimeout(searchTimer)
})

const emptyText = computed(() =>
  search.value.trim() || dirFilter.value !== null
    ? '没有匹配的条目'
    : '空文件夹，点右上角「+ 新增」添加条目',
)

// ── 双击行：文件夹 → 进入；文件 → 能浏览的直接打开所在目录，虚拟路径只提示（复制/跳转是步骤5）──
function onRowDblclick(row: Entry) {
  if (row.isDir) {
    enterFolder(row.id)
    return
  }
  const browsable = (row.paths || []).find((p) => canBrowsePath(p.type, p.path))
  if (browsable) {
    openBrowser(browsable.path)
    return
  }
  if (row.paths?.length) {
    ElMessage.info('虚拟路径暂不支持双击打开，可点路径芯片复制')
  } else {
    ElMessage.warning('这条没有路径，无法打开')
  }
}

// ── 新增/编辑弹窗 ──
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingEntry = ref<Entry | null>(null)
const defaultParentId = ref<string | null>(null)

// 所属文件夹下拉：编辑时排除自身与子孙，避免把文件夹挂进自己的子树
const folderOpts = computed(() => folderOptions(editingEntry.value?.id ?? null))

function openAdd(parent: string | null) {
  editingEntry.value = null
  defaultParentId.value = parent
  dialogMode.value = 'create'
  dialogVisible.value = true
}

function openEdit(entry: Entry) {
  editingEntry.value = entry
  defaultParentId.value = null
  dialogMode.value = 'edit'
  dialogVisible.value = true
}

async function onDialogSubmit(form: EntryForm) {
  const isEdit = !!editingEntry.value
  try {
    await save(editingEntry.value?.id ?? null, form)
    await loadFolders() // 可能新增/移动了文件夹，树要跟着变
    ElMessage.success(isEdit ? '已更新' : '已新增')
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}

// ── 删除：文件夹级联删子（Windows 语义），确认框提示子孙文件夹数 ──
async function handleDelete(row: Entry) {
  const extra = row.isDir ? countDescendants(row.id) : 0
  const tip = extra > 0 ? `，其下 ${extra} 个子文件夹会一起删除` : ''
  try {
    await ElMessageBox.confirm(`确定删除「${row.alias}」？${tip}`, '确认', { type: 'warning' })
    await remove(row.id)
    if (row.isDir) await loadFolders() // 删了文件夹，树要重算
    ElMessage.success('已删除')
    if (currentFolder.value === row.id) enterFolder(null)
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return // 用户取消
    ElMessage.error(e.message || '删除失败')
  }
}

// ── 目录浏览器（步骤4）：从台账路径只读浏览本地目录 ──
const browserVisible = ref(false)
const browserPath = ref('')

// 盘符在线状态（只读缓存，用于「盘符离线，无法定位」的提示）
const { onlineOf } = useDriveStatus()

function openBrowser(path: string) {
  browserPath.value = path
  browserVisible.value = true
}

// ── 步骤5：路径复制 / 定位 ──
// 复制：优先 Clipboard API（127.0.0.1 属安全上下文），失败降级 textarea + execCommand。
// 剪贴板这类浏览器副作用留在页面，表格组件只抛 copy-path 事件。
async function writeClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    // 无权限 / 非安全上下文：落到下面的降级方案
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '-1000px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}

async function copyPath(path: string) {
  const text = (path || '').trim()
  if (!text) return
  if (await writeClipboard(text)) ElMessage.success('已复制路径')
  else ElMessage.warning('复制失败，请手动选择路径复制')
}

// 定位：打开文件所在目录（盘符离线也照开，DirBrowser 自己会显示后端返回的错误文案）
function revealPath(path: string) {
  const p = (path || '').trim()
  if (!p) return
  const drive = driveOf(p)
  if (drive && onlineOf(drive) === false) ElMessage.warning('盘符离线，无法定位')
  openBrowser(parentOf(p))
}
</script>

<style scoped>
.cloud-layout {
  display: flex;
  gap: 16px;
  height: 100%;
}
/* 右栏做成纵向 flex：工具栏固定，列表自己滚动（固定高度交给 entry-scroll） */
.content-panel {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.toolbar {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}
.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.total-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.search-input { width: 180px; }
</style>
