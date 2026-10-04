<template>
  <div class="cloud-layout">
    <!-- 左侧：文件夹树 -->
    <aside class="tree-panel">
      <div class="tree-header">
        <span class="tree-title">虚拟网盘</span>
        <el-button link type="primary" size="small" @click="openAdd(null)">+ 新建文件夹</el-button>
      </div>
      <el-tree
        :data="treeData"
        node-key="id"
        :props="{ label: 'alias', children: 'children' }"
        default-expand-all
        highlight-current
        @node-click="onNodeClick"
      />
      <div v-if="!treeData.length" class="tree-empty muted">还没有文件夹，点「+ 新建文件夹」</div>
    </aside>

    <!-- 右侧：当前文件夹内容 -->
    <main class="content-panel">
      <div class="toolbar">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item v-for="(b, i) in breadcrumbs" :key="b.id ?? 'root'">
            <a v-if="i < breadcrumbs.length - 1" @click="currentFolder = b.id">{{ b.label }}</a>
            <span v-else>{{ b.label }}</span>
          </el-breadcrumb-item>
        </el-breadcrumb>

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

      <el-table v-loading="loading" :data="filtered" stripe style="width: 100%" empty-text="空文件夹，点右上角「+ 新增」添加条目">
        <el-table-column label="名字" min-width="140">
          <template #default="{ row }">
            <span v-if="row.isDir" class="dir-name">📁 {{ row.alias }}</span>
            <span v-else>📄 {{ row.alias }}</span>
          </template>
        </el-table-column>

        <el-table-column label="路径" min-width="320">
          <template #default="{ row }">
            <div class="path-tags">
              <el-tag
                v-for="(p, i) in row.paths"
                :key="i"
                :type="PATH_TYPE_TAG[p.type]"
                size="small"
                class="path-tag"
              >
                {{ PATH_TYPE_LABEL[p.type] }} {{ p.path }}
              </el-tag>
              <span v-if="!row.paths?.length" class="muted">--</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="大小" width="90" align="right">
          <template #default="{ row }">
            <span>{{ row.size || '--' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="标签" min-width="140">
          <template #default="{ row }">
            <span v-for="t in row.tags" :key="t" class="tag-chip colored" :style="getTagColor(t)">{{ t }}</span>
            <span v-if="!row.tags?.length" class="muted">--</span>
          </template>
        </el-table-column>

        <el-table-column label="描述" prop="note" min-width="120" show-overflow-tooltip />

        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </main>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560" :close-on-click-modal="false">
      <el-form :model="form" label-width="90">
        <el-form-item label="名字" required>
          <el-input v-model="form.alias" placeholder="例如：游戏、config.yaml" />
        </el-form-item>

        <el-form-item label="类型" required>
          <el-radio-group v-model="form.isDir" :disabled="editingId !== null">
            <el-radio :value="true">文件夹</el-radio>
            <el-radio :value="false">文件</el-radio>
          </el-radio-group>
          <span v-if="editingId !== null" class="muted" style="margin-left: 8px">创建后不可改</span>
        </el-form-item>

        <el-form-item label="所属文件夹">
          <el-tree-select
            v-model="form.parentId"
            :data="treeSelectData"
            :props="{ label: 'alias', children: 'children' }"
            node-key="id"
            check-strictly
            clearable
            placeholder="不选 = 顶层"
            style="width: 100%"
          />
        </el-form-item>

        <template v-if="form.isDir">
          <el-form-item :label="form.dirType === 'virtual' ? '指向（可选）' : '路径（可选）'">
            <el-select v-model="form.dirType" style="width: 100px">
              <el-option v-for="o in PATH_TYPE_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
            </el-select>
            <el-input
              v-model="form.dirPath"
              :placeholder="form.dirType === 'virtual' ? 'ali:docs 或 https://…' : (form.dirType === 'mount' ? 'Z:\\docs' : 'D:\\docs')"
              style="width: 300px; margin-left: 8px"
            />
            <div class="field-hint muted">虚拟文件夹可不填（纯分类容器），填了表示映射真实目录</div>
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="路径" required>
            <div class="path-list">
              <div v-for="(p, i) in form.paths" :key="i" class="path-row">
                <el-select v-model="p.type" style="width: 100px">
                  <el-option v-for="o in PATH_TYPE_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
                </el-select>
                <el-input v-model="p.path" :placeholder="p.type === 'virtual' ? 'ali:docs 或 https://…' : 'D:\\docs'" style="flex: 1" />
                <el-button v-if="form.paths.length > 1" link type="danger" @click="removePath(i)">×</el-button>
              </div>
            </div>
            <el-button text size="small" @click="addPath">+ 添加备份路径</el-button>
          </el-form-item>
        </template>

        <el-form-item label="大小">
          <el-input v-model="form.size" placeholder="例如：1.2 GB" style="width: 200px" />
        </el-form-item>

        <el-form-item label="标签">
          <TagSelector v-model="form.tags" />
        </el-form-item>

        <el-form-item label="描述">
          <el-input v-model="form.note" type="textarea" :rows="2" placeholder="备忘信息（可选）" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import TagSelector from '../components/TagSelector.vue'
import { getTagColor } from '../constants/tags'
import { PATH_TYPE_LABEL, PATH_TYPE_TAG, PATH_TYPE_OPTIONS } from '../constants/pathTypes'
import { listEntries, createEntry, updateEntry, deleteEntry, type Entry, type EntryForm, type PathType } from '../api'

// ── 数据（真实 API，全量加载 → 前端组树）──
const entries = ref<Entry[]>([])
const loading = ref(false)

async function loadEntries() {
  loading.value = true
  try {
    entries.value = await listEntries()
  } catch (e: any) {
    ElMessage.error(e.message || '加载台账失败')
  } finally {
    loading.value = false
  }
}
onMounted(loadEntries)

const pid = (e: Entry | null | undefined) => (e?.parentId ?? null)

// ── 左侧树 ──
interface TreeNode {
  id: string
  alias: string
  children: TreeNode[]
}

function buildTree(parentId: string | null): TreeNode[] {
  return entries.value
    .filter((e) => e.isDir && pid(e) === parentId)
    .map((e) => ({ id: e.id, alias: e.alias, children: buildTree(e.id) }))
}

const treeData = computed(() => buildTree(null))

// 树选择器数据：根节点「顶层」+ 全树
const treeSelectData = computed(() => [
  { id: '', alias: '顶层', children: buildTree(null) as any },
])

const currentFolder = ref<string | null>(null)

function onNodeClick(node: TreeNode) {
  currentFolder.value = node.id
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

// ── dialog ──
const dialogVisible = ref(false)
const dialogTitle = ref('')
const editingId = ref<string | null>(null)

const form = reactive({
  alias: '',
  parentId: null as string | null,
  isDir: true,
  size: '',
  note: '',
  tags: [] as string[],
  dirType: 'virtual' as PathType,
  dirPath: '',
  paths: [{ type: 'local' as PathType, path: '' }],
})

function resetForm() {
  form.alias = ''
  form.parentId = null
  form.isDir = true
  form.size = ''
  form.note = ''
  form.tags = []
  form.paths = [{ type: 'local', path: '' }]
  form.dirType = 'virtual'
  form.dirPath = ''
  editingId.value = null
}

function openAdd(parent: string | null) {
  resetForm()
  form.parentId = parent
  dialogTitle.value = '新增'
  dialogVisible.value = true
}

function openEdit(entry: Entry) {
  editingId.value = entry.id
  form.alias = entry.alias
  form.parentId = pid(entry)
  form.isDir = entry.isDir
  form.size = entry.size || ''
  form.note = entry.note || ''
  form.tags = [...(entry.tags || [])]
  if (entry.isDir) {
    form.dirType = entry.paths[0]?.type || 'virtual'
    form.dirPath = entry.paths[0]?.path || ''
  } else {
    form.paths = entry.paths.map((p) => ({ type: p.type, path: p.path }))
  }
  dialogTitle.value = '编辑'
  dialogVisible.value = true
}

function addPath() {
  form.paths.push({ type: 'local' as PathType, path: '' })
}

function removePath(i: number) {
  form.paths.splice(i, 1)
}

async function handleSave() {
  if (!form.alias) { ElMessage.warning('名字不能为空'); return }
  let paths
  if (form.isDir) {
    paths = form.dirPath.trim() ? [{ type: form.dirType, path: form.dirPath.trim() }] : []
  } else {
    paths = form.paths.filter((p) => p.path.trim())
    if (!paths.length) { ElMessage.warning('文件至少填一条路径'); return }
  }

  const entry: EntryForm = {
    alias: form.alias,
    parentId: form.parentId || null,
    isDir: form.isDir,
    size: form.size || '',
    note: form.note,
    tags: [...form.tags],
    paths: paths.map((p) => ({ type: p.type as PathType, path: p.path })),
  }

  try {
    if (editingId.value) {
      await updateEntry(editingId.value, entry)
      ElMessage.success('已更新')
    } else {
      await createEntry(entry)
      ElMessage.success('已新增')
    }
    dialogVisible.value = false
    await loadEntries()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}

// ── 删除：文件夹级联删子（Windows 语义），确认框提示子条目数 ──
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

async function handleDelete(row: Entry) {
  const extra = row.isDir ? countDescendants(row.id) : 0
  const tip = extra > 0 ? `，将同时删除其下 ${extra} 个子条目` : ''
  try {
    await ElMessageBox.confirm(`确定删除「${row.alias}」？${tip}`, '确认', { type: 'warning' })
    await deleteEntry(row.id)
    ElMessage.success('已删除')
    if (currentFolder.value === row.id) currentFolder.value = null
    await loadEntries()
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return // 用户取消
    ElMessage.error(e.message || '删除失败')
  }
}
</script>

<style scoped>
.cloud-layout {
  display: flex;
  gap: 16px;
  height: 100%;
}
.tree-panel {
  width: 220px;
  flex-shrink: 0;
  border-right: 1px solid var(--el-border-color-light);
  padding-right: 12px;
  overflow: auto;
}
.tree-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.tree-title {
  font-weight: 600;
  font-size: 14px;
}
.tree-empty {
  font-size: 12px;
  padding: 8px 0;
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
.dir-name { font-weight: 500; }
.path-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.path-tag { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag-chip { margin-right: 4px; display: inline-block; }
.tag-chip.colored { padding: 1px 6px; border-radius: 3px; font-size: 12px; margin-right: 4px; }
.muted { color: var(--el-text-color-placeholder); font-size: 12px; }
.field-hint { margin-top: 4px; }

.path-list { display: flex; flex-direction: column; gap: 8px; }
.path-row { display: flex; gap: 8px; align-items: center; }
</style>
