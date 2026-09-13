<template>
  <div class="registry-page">
    <div class="page-header">
      <h2 class="page-title">路径管理</h2>
      <el-button type="primary" @click="openAdd">+ 新增路径</el-button>
    </div>

    <el-table :data="entries" stripe style="width: 100%">
      <el-table-column label="别名" prop="alias" min-width="140" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag :type="tagType[row.type]" size="small">{{ typeLabel[row.type] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="路径" min-width="220">
        <template #default="{ row }">
          <span v-if="row.type === 'virtual'">{{ row.relPath }}</span>
          <span v-else>{{ row.root }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <span class="status-dot" :class="statusClass(row)"></span>
        </template>
      </el-table-column>
      <el-table-column label="备注" prop="note" min-width="120" show-overflow-tooltip />
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button size="small" link @click="openEdit(row)">编辑</el-button>
          <el-button size="small" link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="480" :close-on-click-modal="false">
      <el-form :model="form" label-width="90">
        <el-form-item label="别名" required>
          <el-input v-model="form.alias" placeholder="例如：文档" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-select v-model="form.type" style="width: 100%">
            <el-option value="local" label="本地目录" />
            <el-option value="mount" label="挂载盘符" />
            <el-option value="virtual" label="虚拟路径" />
          </el-select>
        </el-form-item>
        <template v-if="form.type !== 'virtual'">
          <el-form-item label="根路径" required>
            <el-input v-model="form.root" :placeholder="form.type === 'mount' ? 'Z:' : 'D:\\docs'" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="虚拟路径" required>
            <el-input v-model="form.relPath" placeholder="ali:game/rpg" />
          </el-form-item>
          <el-form-item label="跳转链接">
            <el-input v-model="form.jumpUrl" placeholder="https://" />
          </el-form-item>
        </template>
        <el-form-item label="备注">
          <el-input v-model="form.note" type="textarea" :rows="2" />
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
import { ref, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface Entry {
  id: string
  alias: string
  type: 'local' | 'mount' | 'virtual'
  root: string
  relPath: string
  status: 'online' | 'offline' | 'virtual'
  note: string
}

interface EntryForm {
  alias: string
  type: 'local' | 'mount' | 'virtual'
  root: string
  relPath: string
  jumpUrl: string
  note: string
}

const typeLabel: Record<string, string> = { local: '本地目录', mount: '挂载盘符', virtual: '虚拟路径' }
const tagType: Record<string, string> = { local: '', mount: 'warning', virtual: 'info' }

// mock data — 后续接后端
const entries = ref<Entry[]>([
  { id: '1', alias: '文档', type: 'local', root: 'D:\\docs', relPath: '', status: 'online', note: '项目文档' },
  { id: '2', alias: '游戏盘', type: 'mount', root: 'Z:', relPath: '', status: 'online', note: 'NAS 挂载' },
  { id: '3', alias: '备份网盘', type: 'virtual', root: '', relPath: 'ali:backup', status: 'virtual', note: '阿里云盘' },
])

let nextId = 100

const dialogVisible = ref(false)
const dialogTitle = ref('')
const editingId = ref<string | null>(null)
const form = reactive<EntryForm>({ alias: '', type: 'local', root: '', relPath: '', jumpUrl: '', note: '' })

function resetForm() {
  form.alias = ''
  form.type = 'local'
  form.root = ''
  form.relPath = ''
  form.jumpUrl = ''
  form.note = ''
  editingId.value = null
}

function openAdd() {
  resetForm()
  dialogTitle.value = '新增路径'
  dialogVisible.value = true
}

function openEdit(entry: Entry) {
  editingId.value = entry.id
  form.alias = entry.alias
  form.type = entry.type
  form.root = entry.root
  form.relPath = entry.relPath
  form.jumpUrl = ''
  form.note = entry.note
  dialogTitle.value = '编辑路径'
  dialogVisible.value = true
}

function handleSave() {
  if (!form.alias || (form.type !== 'virtual' && !form.root) || (form.type === 'virtual' && !form.relPath)) {
    ElMessage.warning('请填写必填项')
    return
  }
  if (editingId.value) {
    const idx = entries.value.findIndex((e) => e.id === editingId.value)
    if (idx !== -1) {
      entries.value[idx] = {
        ...entries.value[idx],
        alias: form.alias,
        type: form.type,
        root: form.root,
        relPath: form.relPath,
        note: form.note,
      }
    }
    ElMessage.success('已更新')
  } else {
    entries.value.push({
      id: String(nextId++),
      alias: form.alias,
      type: form.type,
      root: form.root,
      relPath: form.relPath,
      status: form.type === 'virtual' ? 'virtual' : 'online',
      note: form.note,
    })
    ElMessage.success('已新增')
  }
  dialogVisible.value = false
}

async function handleDelete(entry: Entry) {
  try {
    await ElMessageBox.confirm(`确定删除「${entry.alias}」？`, '确认', { type: 'warning' })
    entries.value = entries.value.filter((e) => e.id !== entry.id)
    ElMessage.success('已删除')
  } catch {
    // cancelled
  }
}

function statusClass(entry: Entry) {
  return entry.status === 'online' ? 'dot-online' : entry.status === 'offline' ? 'dot-offline' : 'dot-virtual'
}
</script>

<style scoped>
.registry-page {
  max-width: 960px;
}
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.page-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.dot-online { background: #67c23a; }
.dot-offline { background: #f56c6c; }
.dot-virtual { background: #909399; }
</style>