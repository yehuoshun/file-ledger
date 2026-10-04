<template>
  <div>
    <div class="toolbar">
      <el-select v-model="dirFilter" placeholder="类型" clearable size="default" style="width: 120px">
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

      <el-button type="primary" size="default" @click="openAdd">+ 新增</el-button>
    </div>

    <el-table v-loading="loading" :data="filtered" stripe style="width: 100%" empty-text="暂无台账，点右上角「+ 新增」开始">
      <el-table-column label="名字" prop="alias" min-width="120" />

      <el-table-column label="路径" min-width="360">
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
          </div>
        </template>
      </el-table-column>

      <el-table-column label="文件夹" width="70" align="center">
        <template #default="{ row }">
          <span>{{ row.isDir ? '📁' : '📄' }}</span>
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

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560" :close-on-click-modal="false">
      <el-form :model="form" label-width="80">
        <el-form-item label="名字" required>
          <el-input v-model="form.alias" placeholder="例如：文档、config.yaml" />
        </el-form-item>

        <el-form-item label="类型" required>
          <el-radio-group v-model="form.isDir">
            <el-radio :value="true">文件夹</el-radio>
            <el-radio :value="false">文件</el-radio>
          </el-radio-group>
        </el-form-item>

        <template v-if="form.isDir">
          <el-form-item :label="form.dirType === 'virtual' ? '指向' : '路径'" required>
            <el-select v-model="form.dirType" style="width: 100px">
              <el-option v-for="o in PATH_TYPE_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
            </el-select>
            <el-input
              v-model="form.dirPath"
              :placeholder="form.dirType === 'virtual' ? 'ali:docs 或 https://…' : (form.dirType === 'mount' ? 'Z:\\docs' : 'D:\\docs')"
              style="width: 300px; margin-left: 8px"
            />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="路径">
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

// ── 数据（真实 API）──
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

// ── search / filter ──
const search = ref('')
const dirFilter = ref<boolean | null>(null)

const filtered = computed(() =>
  entries.value.filter((e: Entry) => {
    if (dirFilter.value !== null && e.isDir !== dirFilter.value) return false
    if (search.value) {
      const q = search.value.toLowerCase()
      const matchName = e.alias.toLowerCase().includes(q)
      const matchNote = (e.note || '').toLowerCase().includes(q)
      const matchTags = (e.tags || []).some((t: string) => t.toLowerCase().includes(q))
      if (!matchName && !matchNote && !matchTags) return false
    }
    return true
  })
)

// ── dialog ──
const dialogVisible = ref(false)
const dialogTitle = ref('')
const editingId = ref<string | null>(null)

const form = reactive({
  alias: '',
  isDir: true,
  size: '',
  note: '',
  tags: [] as string[],
  dirType: 'virtual' as PathType, // 文件夹默认虚拟（虚拟文件夹实际不存在，仅记录指向）
  dirPath: '',
  paths: [{ type: 'local' as PathType, path: '' }],
})

function resetForm() {
  form.alias = ''
  form.isDir = true
  form.size = ''
  form.note = ''
  form.tags = []
  form.paths = [{ type: 'local', path: '' }]
  form.dirType = 'virtual'
  form.dirPath = ''
  editingId.value = null
}

function openAdd() {
  resetForm()
  dialogTitle.value = '新增'
  dialogVisible.value = true
}

function openEdit(entry: Entry) {
  editingId.value = entry.id
  form.alias = entry.alias
  form.isDir = entry.isDir
  form.size = entry.size || ''
  form.note = entry.note || ''
  form.tags = [...(entry.tags || [])]
  if (entry.isDir) {
    form.dirType = entry.paths[0]?.type || 'virtual'
    form.dirPath = entry.paths[0]?.path || ''
  } else {
    form.paths = entry.paths.map((p: any) => ({ type: p.type, path: p.path }))
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
    if (!form.dirPath.trim()) { ElMessage.warning('路径不能为空'); return }
    paths = [{ type: form.dirType, path: form.dirPath.trim() }]
  } else {
    paths = form.paths.filter((p) => p.path.trim())
    if (!paths.length) { ElMessage.warning('至少填一条路径'); return }
  }

  const entry: EntryForm = {
    alias: form.alias,
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

async function handleDelete(row: Entry) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.alias}」？`, '确认', { type: 'warning' })
    await deleteEntry(row.id)
    ElMessage.success('已删除')
    await loadEntries()
  } catch (e: any) {
    if (e === 'cancel' || e === 'close') return // 用户取消
    ElMessage.error(e.message || '删除失败')
  }
}
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.search-input { width: 200px; }
.path-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.path-tag { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag-chip { margin-right: 4px; display: inline-block; }
.tag-chip.colored { padding: 1px 6px; border-radius: 3px; font-size: 12px; margin-right: 4px; }
.muted { color: var(--el-text-color-placeholder); }

.path-list { display: flex; flex-direction: column; gap: 8px; }
.path-row { display: flex; gap: 8px; align-items: center; }
.tag-list { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 6px; }
</style>