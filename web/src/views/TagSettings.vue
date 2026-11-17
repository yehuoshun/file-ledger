<template>
  <div class="tag-page">
    <h2 class="page-title">标签管理</h2>

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

    <!-- 标签可能有上千条：列表走服务端分页，顶部按名字搜索（防抖） -->
    <div class="tag-toolbar">
      <el-input
        v-model="keyword"
        placeholder="按名字搜索标签…"
        clearable
        style="width: 240px"
      />
    </div>

    <el-table
      v-loading="loading"
      :data="items"
      stripe
      empty-text="没有匹配的标签"
      style="width: 100%"
    >
      <el-table-column label="标签" min-width="160">
        <template #default="{ row }">
          <span class="tag-chip" :style="{ background: row.bg, color: row.fg }">
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

    <div class="tag-pager">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        background
        @current-change="onPageChange"
        @size-change="onSizeChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useTags } from '../composables/useTags'
import type { Tag } from '../api'

// 标签管理：取数与写操作都在 useTags（单例），页面只做分页/搜索编排与提示
const { fetchPage, addTag, renameTag, recolorTag, removeTag } = useTags()

// 服务端分页 + 按名字搜索：只持有当前页数据，不在前端拼全表
const items = ref<Tag[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const loading = ref(false)

const SEARCH_DEBOUNCE = 300
let searchTimer: ReturnType<typeof setTimeout> | undefined

const newName = ref('')
const newBg = ref<string | null>(null)
const newFg = ref<string | null>(null)
// 重命名草稿：按 id 存，避免直接改动列表里的对象（失败时列表要显示原值）
const drafts = reactive<Record<string, string>>({})

// 颜色默认值（清空色板时回到它，与 composable 的兜底色一致）
const DEFAULT_BG = '#e4e7ed'
const DEFAULT_FG = '#303133'

function errText(e: unknown, fallback = '操作失败'): string {
  const msg = (e as { message?: string } | null)?.message
  return msg || fallback
}

async function load() {
  loading.value = true
  try {
    let data = await fetchPage(keyword.value, page.value, pageSize.value)
    // 删掉当前页最后一条时页码会越界：退到最后一页再取一次
    const maxPage = Math.max(1, Math.ceil(data.total / pageSize.value))
    if (page.value > maxPage) {
      page.value = maxPage
      data = await fetchPage(keyword.value, page.value, pageSize.value)
    }
    items.value = data.items
    total.value = data.total
  } catch (e) {
    ElMessage.error(errText(e, '加载标签失败'))
  } finally {
    loading.value = false
  }
}

onMounted(load)
onUnmounted(() => {
  if (searchTimer !== undefined) clearTimeout(searchTimer)
})

// 搜索防抖：改关键字回到第一页再查
watch(keyword, () => {
  if (searchTimer !== undefined) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    searchTimer = undefined
    page.value = 1
    load()
  }, SEARCH_DEBOUNCE)
})

function onPageChange() {
  load()
}

function onSizeChange() {
  page.value = 1
  load()
}

async function onAdd() {
  const name = newName.value.trim()
  if (!name) return
  try {
    await addTag(name, { bg: newBg.value || undefined, fg: newFg.value || undefined })
    newName.value = ''
    newBg.value = null
    newFg.value = null
    await load()
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
    await load()
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
    await load()
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
    await load()
  } catch (e) {
    ElMessage.error(errText(e, '删除标签失败'))
  }
}
</script>

<style scoped>
.tag-page {
  max-width: 880px;
}
.page-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 24px;
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
.tag-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.tag-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
.tag-chip {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 3px;
  font-size: 12px;
}
</style>
