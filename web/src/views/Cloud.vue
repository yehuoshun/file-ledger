<template>
  <div class="cloud-layout">
    <!-- 左侧：文件夹树 -->
    <FolderTreePanel ref="treeRef" :data="treeData" @select="enterFolder" @add="openAdd(null)" />

    <!-- 右侧：当前文件夹内容 -->
    <main class="content-panel">
      <div class="toolbar">
        <FolderBreadcrumbs :crumbs="breadcrumbs" @navigate="enterFolder" />

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
        :rows="filtered"
        :loading="loading"
        @row-dblclick="onRowDblclick"
        @edit="openEdit"
        @delete="handleDelete"
        @browse="openBrowser"
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
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import FolderTreePanel from '../components/cloud/FolderTreePanel.vue'
import FolderBreadcrumbs from '../components/cloud/FolderBreadcrumbs.vue'
import EntryTable from '../components/cloud/EntryTable.vue'
import EntryDialog from '../components/cloud/EntryDialog.vue'
import DirBrowser from '../components/DirBrowser.vue'
import { useEntries } from '../composables/useEntries'
import { useFolderTree } from '../composables/useFolderTree'
import { canBrowsePath } from '../utils/paths'
import type { Entry, EntryForm } from '../api'

// ── 数据与派生（全部逻辑在 composable 里，本组件只做编排）──
const { entries, loading, reload, save, remove } = useEntries()
const { pid, treeData, countDescendants, folderOptions } = useFolderTree(entries)
onMounted(reload)

// ── 当前文件夹与树高亮 ──
const currentFolder = ref<string | null>(null)
const treeRef = ref<InstanceType<typeof FolderTreePanel> | null>(null)

// 进入某个文件夹（Windows 语义：双击文件夹 = 进入）；id 为 null 表示回到顶层
function enterFolder(id: string | null) {
  currentFolder.value = id
  if (id) nextTick(() => treeRef.value?.setCurrentKey(id))
}

// ── 面包屑 ──
const breadcrumbs = computed(() => {
  const chain: { id: string | null; label: string }[] = [{ id: null, label: '虚拟网盘' }]
  let cur: Entry | undefined
  if (currentFolder.value) {
    cur = entries.value.find((e) => e.id === currentFolder.value)
  }
  const stack: Entry[] = []
  while (cur && cur.isDir) {
    stack.unshift(cur)
    cur = cur.parentId ? entries.value.find((e) => e.id === cur!.parentId) : undefined
  }
  chain.push(...stack.map((e) => ({ id: e.id, label: e.alias })))
  return chain
})

// ── 右侧列表：当前文件夹的直接子条目（文件夹在前，别名排序）──
const search = ref('')
const dirFilter = ref<boolean | null>(null)

const filtered = computed(() => {
  let list = entries.value.filter((e) => pid(e) === currentFolder.value)
  if (dirFilter.value !== null) list = list.filter((e) => e.isDir === dirFilter.value)
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter((e) => {
      const matchName = e.alias.toLowerCase().includes(q)
      const matchNote = (e.note || '').toLowerCase().includes(q)
      const matchTags = (e.tags || []).some((t) => t.toLowerCase().includes(q))
      return matchName || matchNote || matchTags
    })
  }
  return [...list].sort((a, b) =>
    a.isDir === b.isDir ? a.alias.localeCompare(b.alias, 'zh') : a.isDir ? -1 : 1,
  )
})

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
    ElMessage.info('虚拟路径暂不支持双击打开，步骤 5 会做成复制路径 / 跳转链接')
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
    ElMessage.success(isEdit ? '已更新' : '已新增')
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}

// ── 删除：文件夹级联删子（Windows 语义），确认框提示子条目数 ──
async function handleDelete(row: Entry) {
  const extra = row.isDir ? countDescendants(row.id) : 0
  const tip = extra > 0 ? `，将同时删除其下 ${extra} 个子条目` : ''
  try {
    await ElMessageBox.confirm(`确定删除「${row.alias}」？${tip}`, '确认', { type: 'warning' })
    await remove(row.id)
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

function openBrowser(path: string) {
  browserPath.value = path
  browserVisible.value = true
}
</script>

<style scoped>
.cloud-layout {
  display: flex;
  gap: 16px;
  height: 100%;
}
.content-panel {
  flex: 1;
  min-width: 0;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}
.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.search-input { width: 180px; }
</style>
