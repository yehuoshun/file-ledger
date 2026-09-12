<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const TYPE_META = {
  local: { label: '本地', tag: 'primary' },
  mount: { label: '挂载', tag: 'success' },
  virtual: { label: '虚拟', tag: 'warning' },
}

const loading = ref(false)
const entries = ref([])

// ---------- 列表 ----------
async function loadList() {
  loading.value = true
  try {
    entries.value = await api.list()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

// ---------- 新增 / 编辑 ----------
const dialog = reactive({
  visible: false,
  mode: 'create', // create | edit
  submitting: false,
  form: {
    id: '',
    alias: '',
    type: 'local',
    root: '',
    rel_path: '',
    jump_url: '',
    note: '',
  },
})

const formRef = ref(null)

const dialogTitle = () =>
  dialog.mode === 'create' ? '新增台账' : `编辑台账（${dialog.form.alias}）`

function openCreate() {
  dialog.mode = 'create'
  Object.assign(dialog.form, {
    id: '',
    alias: '',
    type: 'local',
    root: '',
    rel_path: '',
    jump_url: '',
    note: '',
  })
  dialog.visible = true
}

function openEdit(row) {
  dialog.mode = 'edit'
  Object.assign(dialog.form, {
    id: row.id,
    alias: row.alias,
    type: row.type,
    root: row.root || '',
    rel_path: row.rel_path || '',
    jump_url: row.jump_url || '',
    note: row.note || '',
  })
  dialog.visible = true
}

// type 联动校验：local/mount 要 root，virtual 要 rel_path
function validateType() {
  if (dialog.form.type === 'virtual') {
    return { required: true, message: '虚拟路径必填', trigger: 'blur' }
  }
  return { required: true, message: '根路径必填', trigger: 'blur' }
}

const rules = {
  alias: [
    { required: true, message: '别名必填', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅限字母/数字/_-', trigger: 'blur' },
  ],
  type: [{ required: true, message: '类型必选', trigger: 'change' }],
  root: [validateType],
  rel_path: [validateType],
}

async function submit() {
  await formRef.value.validate()
  dialog.submitting = true
  try {
    const payload = {
      alias: dialog.form.alias.trim(),
      type: dialog.form.type,
      root: dialog.form.type === 'virtual' ? '' : dialog.form.root.trim(),
      rel_path: dialog.form.type === 'virtual' ? dialog.form.rel_path.trim() : '',
      jump_url: dialog.form.jump_url.trim(),
      note: dialog.form.note.trim(),
    }
    if (dialog.mode === 'create') {
      await api.create(payload)
      ElMessage.success('已新增')
    } else {
      await api.update(dialog.form.id, payload)
      ElMessage.success('已保存')
    }
    dialog.visible = false
    loadList()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    dialog.submitting = false
  }
}

// ---------- 删除 ----------
async function remove(row) {
  try {
    await ElMessageBox.confirm(
      `确定删除台账「${row.alias}」吗？`,
      '删除确认',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
    )
  } catch {
    return // 用户取消
  }
  try {
    await api.remove(row.id)
    ElMessage.success('已删除')
    loadList()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

// ---------- 展示辅助 ----------
function typeMeta(type) {
  return TYPE_META[type] || { label: type, tag: 'info' }
}

function pathText(row) {
  if (row.type === 'virtual') return row.rel_path || '-'
  return row.root || '-'
}

function fmtTime(t) {
  if (!t) return '-'
  return t.replace('T', ' ').slice(0, 16)
}

onMounted(loadList)
</script>

<template>
  <div class="registry-page">
    <div class="page-header">
      <h2>台账管理</h2>
      <el-button type="primary" @click="openCreate">＋ 新增台账</el-button>
    </div>

    <el-table :data="entries" v-loading="loading" stripe border>
      <el-table-column prop="alias" label="别名" width="120">
        <template #default="{ row }">
          <code class="alias-code">{{ row.alias }}</code>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="90">
        <template #default="{ row }">
          <el-tag :type="typeMeta(row.type).tag" size="small">
            {{ typeMeta(row.type).label }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="路径" min-width="220">
        <template #default="{ row }">
          <span :class="{ 'path-virtual': row.type === 'virtual' }">{{ pathText(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="note" label="备注" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">{{ row.note || '-' }}</template>
      </el-table-column>
      <el-table-column label="更新时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.updated_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" size="small" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="还没有台账，点右上角「新增台账」开始" />
      </template>
    </el-table>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="dialog.visible" :title="dialogTitle" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="dialog.form" :rules="rules" label-width="90px">
        <el-form-item label="别名" prop="alias">
          <el-input v-model="dialog.form.alias" placeholder="如 z / games / ali" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="dialog.form.type">
            <el-radio-button value="local">本地目录</el-radio-button>
            <el-radio-button value="mount">挂载盘</el-radio-button>
            <el-radio-button value="virtual">虚拟路径</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item
          v-if="dialog.form.type !== 'virtual'"
          label="根路径"
          prop="root"
        >
          <el-input
            v-model="dialog.form.root"
            :placeholder="dialog.form.type === 'mount' ? '如 Z:' : '如 D:\\Games'"
          />
        </el-form-item>
        <template v-if="dialog.form.type === 'virtual'">
          <el-form-item label="虚拟路径" prop="rel_path">
            <el-input v-model="dialog.form.rel_path" placeholder="如 game/rpg" />
          </el-form-item>
          <el-form-item label="跳转链接" prop="jump_url">
            <el-input v-model="dialog.form.jump_url" placeholder="可选，网盘分享链接" />
          </el-form-item>
        </template>
        <el-form-item label="备注" prop="note">
          <el-input v-model="dialog.form.note" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="dialog.submitting" @click="submit">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.page-header h2 {
  margin: 0;
  font-size: 18px;
}
.alias-code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: ui-monospace, Consolas, monospace;
}
.path-virtual {
  color: var(--el-color-warning);
}
</style>
