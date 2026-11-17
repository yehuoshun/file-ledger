<template>
  <el-dialog v-model="visible" :title="mode === 'create' ? '新增' : '编辑'" width="560" :close-on-click-modal="false">
    <el-form :model="form" label-width="90">
      <el-form-item label="名字" required>
        <el-input v-model="form.alias" placeholder="例如：游戏、config.yaml" />
      </el-form-item>

      <el-form-item label="类型" required>
        <el-radio-group v-model="form.isDir" :disabled="mode === 'edit'">
          <el-radio :value="true">文件夹</el-radio>
          <el-radio :value="false">文件</el-radio>
        </el-radio-group>
        <span v-if="mode === 'edit'" class="muted" style="margin-left: 8px">创建后不可改</span>
      </el-form-item>

      <el-form-item label="所属文件夹">
        <!-- 用扁平「层级路径」下拉，不用树选择器：树里点一下就选中，极易误触（老板 2026-11-17 反馈） -->
        <el-select
          v-model="form.parentId"
          filterable
          clearable
          placeholder="不选 = 顶层（虚拟网盘根）"
          style="width: 100%"
          no-data-text="还没有文件夹"
        >
          <el-option v-for="o in folderOptions" :key="o.id" :label="o.label" :value="o.id" />
        </el-select>
      </el-form-item>

      <!-- 文件夹=纯分类容器，不再经表单填指向/路径（定版 2026-11-07）；存量已填的编辑时原样保留 -->
      <template v-if="!form.isDir">
        <el-form-item label="路径" required>
          <div class="path-list">
            <div v-for="(p, i) in form.paths" :key="i" class="path-row">
              <el-select v-model="p.type" style="width: 100px">
                <el-option v-for="o in PATH_TYPE_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
              </el-select>
              <el-input
                v-model="p.path"
                :placeholder="p.type === 'virtual' ? 'ali:docs 或 https://…' : 'D:\\docs'"
                style="flex: 1"
              />
              <el-button v-if="form.paths.length > 1" link type="danger" @click="removePath(i)">×</el-button>
            </div>
          </div>
          <el-button text size="small" @click="addPath">+ 添加备份路径</el-button>
        </el-form-item>
      </template>

      <!-- 大小：仅文件条目手填；文件夹大小是计算式（子条目合计），不在此填 -->
      <el-form-item v-if="!form.isDir" label="大小">
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
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="handleSave">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import TagSelector from '../TagSelector.vue'
import { PATH_TYPE_OPTIONS } from '../../constants/pathTypes'
import type { Entry, EntryForm, PathType } from '../../api'

// 新增/编辑弹窗：自己管表单状态与校验，保存只把 EntryForm 抛给父级（父级负责调接口）
const props = defineProps<{
  modelValue: boolean
  mode: 'create' | 'edit'
  entry: Entry | null
  defaultParentId: string | null
  folderOptions: { id: string; label: string }[]
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  submit: [EntryForm]
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})

interface FormState {
  alias: string
  parentId: string | null
  isDir: boolean
  size: string
  note: string
  tags: string[]
  dirType: PathType
  dirPath: string
  paths: { type: PathType; path: string }[]
}

function emptyForm(): FormState {
  return {
    alias: '',
    parentId: null,
    isDir: true,
    size: '',
    note: '',
    tags: [],
    dirType: 'virtual',
    dirPath: '',
    paths: [{ type: 'local', path: '' }],
  }
}

const form = reactive<FormState>(emptyForm())

// 每次打开都按 entry / defaultParentId 重置，避免上一轮的残留
watch(
  () => props.modelValue,
  (open) => {
    if (open) fill()
  },
)

function fill() {
  Object.assign(form, emptyForm())
  const e = props.entry
  if (!e) {
    form.parentId = props.defaultParentId
    return
  }
  form.alias = e.alias
  form.parentId = e.parentId ?? null
  form.isDir = e.isDir
  form.size = e.size || ''
  form.note = e.note || ''
  form.tags = [...(e.tags || [])]
  if (e.isDir) {
    form.dirType = e.paths?.[0]?.type || 'virtual'
    form.dirPath = e.paths?.[0]?.path || ''
  } else {
    const ps = (e.paths || []).map((p) => ({ type: p.type, path: p.path }))
    form.paths = ps.length ? ps : [{ type: 'local', path: '' }]
  }
}

function addPath() {
  form.paths.push({ type: 'local' as PathType, path: '' })
}

function removePath(i: number) {
  form.paths.splice(i, 1)
}

function handleSave() {
  if (!form.alias.trim()) {
    ElMessage.warning('名字不能为空')
    return
  }
  let paths: { type: PathType; path: string }[]
  if (form.isDir) {
    // 文件夹=纯分类容器（定版 2026-11-07）：表单不再提供指向/路径输入。
    // 编辑存量带路径的文件夹时 fill 已把 paths[0] 回填进 dirType/dirPath，
    // 这里用它原样保留（用户不可改，但不会被误清空）。
    paths = form.dirPath.trim() ? [{ type: form.dirType, path: form.dirPath.trim() }] : []
  } else {
    paths = form.paths.filter((p) => p.path.trim())
    if (!paths.length) {
      ElMessage.warning('文件至少填一条路径')
      return
    }
  }

  emit('submit', {
    alias: form.alias,
    parentId: form.parentId || null,
    isDir: form.isDir,
    size: form.isDir ? '' : form.size || '',
    note: form.note,
    tags: [...form.tags],
    paths: paths.map((p) => ({ type: p.type, path: p.path })),
  })
}
</script>

<style scoped>
.muted { color: var(--el-text-color-placeholder); font-size: 12px; }
.path-list { display: flex; flex-direction: column; gap: 8px; }
.path-row { display: flex; gap: 8px; align-items: center; }
</style>
