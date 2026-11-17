<template>
  <div class="settings-page">
    <h2 class="page-title">设置</h2>

    <el-form label-width="120px" class="settings-form">
      <el-form-item label="监听端口">
        <el-input v-model="form.port" placeholder="18765" style="width: 200px" />
        <span class="form-hint">重启后生效</span>
      </el-form-item>

      <el-form-item label="挂载盘符">
        <el-input v-model="form.defaultDrive" placeholder="Z:" style="width: 200px" />
        <span class="form-hint">mount 类型默认盘符</span>
      </el-form-item>

      <el-form-item label="数据库">
        <span class="form-hint">PostgreSQL 127.0.0.1:5432/file_ledger（后端就绪后展示/可改）</span>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" disabled>保存设置</el-button>
        <span class="form-hint">（后端就绪后启用）</span>
      </el-form-item>
    </el-form>

    <h2 class="section-title">标签管理</h2>

    <!-- 新增：名字 + 可选颜色；颜色不选走后端默认 -->
    <div class="tag-add">
      <el-input
        v-model="newName"
        placeholder="新标签名"
        style="width: 200px"
        @keydown.enter="onAdd"
      />
      <el-color-picker v-model="newBg" title="背景色" />
      <el-color-picker v-model="newFg" title="文字色" />
      <el-button type="primary" :disabled="!newName.trim()" @click="onAdd">添加</el-button>
      <span class="form-hint">左＝背景色，右＝文字色（可不选）</span>
    </div>

    <el-table
      v-loading="loading"
      :data="tags"
      stripe
      empty-text="还没有标签，输入名字后点「添加」"
      style="width: 100%"
    >
      <el-table-column label="标签" min-width="160">
        <template #default="{ row }">
          <span class="tag-chip" :style="{ background: colorOf(row.name).bg, color: colorOf(row.name).fg }">
            {{ row.name }}
          </span>
        </template>
      </el-table-column>

      <el-table-column label="重命名" min-width="200">
        <template #default="{ row }">
          <el-input
            size="small"
            :model-value="drafts[row.id] ?? row.name"
            @update:model-value="(v: string) => (drafts[row.id] = v)"
            @blur="commitRename(row)"
            @keydown.enter="commitRename(row)"
          />
        </template>
      </el-table-column>

      <el-table-column label="背景色" width="110" align="center">
        <template #default="{ row }">
          <el-color-picker :model-value="row.bg" @change="(c: string | null) => onRecolor(row, c, row.fg)" />
        </template>
      </el-table-column>

      <el-table-column label="文字色" width="110" align="center">
        <template #default="{ row }">
          <el-color-picker :model-value="row.fg" @change="(c: string | null) => onRecolor(row, row.bg, c)" />
        </template>
      </el-table-column>

      <el-table-column label="操作" width="90" align="center">
        <template #default="{ row }">
          <el-button link type="danger" size="small" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useTags } from '../composables/useTags'
import type { Tag } from '../api'

const form = ref({
  port: '18765',
  defaultDrive: 'Z:',
})

// 标签管理：取数与写操作都在 useTags（单例），页面只做编排与提示
const { tags, loading, colorOf, reload, addTag, renameTag, recolorTag, removeTag } = useTags()

const newName = ref('')
const newBg = ref<string | null>(null)
const newFg = ref<string | null>(null)
// 重命名草稿：按 id 存，避免直接改动字典里的对象（失败时列表要显示原值）
const drafts = reactive<Record<string, string>>({})

// 颜色默认值（清空色板时回到它，与 composable 的兜底色一致）
const DEFAULT_BG = '#e4e7ed'
const DEFAULT_FG = '#303133'

function errText(e: unknown, fallback = '操作失败'): string {
  const msg = (e as { message?: string } | null)?.message
  return msg || fallback
}

onMounted(() => {
  reload()
})

async function onAdd() {
  const name = newName.value.trim()
  if (!name) return
  try {
    await addTag(name, { bg: newBg.value || undefined, fg: newFg.value || undefined })
    newName.value = ''
    newBg.value = null
    newFg.value = null
  } catch (e) {
    // 409 重名 → 后端返回「标签已存在」
    ElMessage.error(errText(e, '新增标签失败'))
  }
}

async function commitRename(row: Tag) {
  const draft = drafts[row.id]
  if (draft === undefined) return
  const name = draft.trim()
  if (!name || name === row.name) {
    delete drafts[row.id]
    return
  }
  try {
    await renameTag(row.id, name)
    delete drafts[row.id]
  } catch (e) {
    ElMessage.error(errText(e, '重命名失败'))
  }
}

async function onRecolor(row: Tag, bg: string | null, fg: string | null) {
  const nextBg = bg || DEFAULT_BG
  const nextFg = fg || DEFAULT_FG
  if (nextBg === row.bg && nextFg === row.fg) return
  try {
    await recolorTag(row.id, nextBg, nextFg)
  } catch (e) {
    ElMessage.error(errText(e, '修改颜色失败'))
  }
}

async function onDelete(row: Tag) {
  try {
    await ElMessageBox.confirm(
      `删除标签「${row.name}」？已用该标签的条目不会被改动。`,
      '删除标签',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
  } catch {
    return // 用户取消
  }
  try {
    await removeTag(row.id)
    delete drafts[row.id]
  } catch (e) {
    ElMessage.error(errText(e, '删除标签失败'))
  }
}
</script>

<style scoped>
.settings-page {
  max-width: 880px;
}
.page-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 24px;
}
.section-title {
  font-size: 16px;
  font-weight: 600;
  margin: 32px 0 16px;
}
.settings-form {
  background: var(--el-bg-color);
}
.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
.tag-add {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.tag-chip {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 3px;
  font-size: 12px;
}
</style>
