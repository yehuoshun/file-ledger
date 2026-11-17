<template>
  <el-drawer
    :model-value="modelValue"
    title="浏览目录"
    direction="rtl"
    size="760px"
    destroy-on-close
    @update:model-value="onVisibleChange"
  >
    <div class="dir-browser">
      <!-- 工具栏：上级 + 面包屑 + 刷新 -->
      <div class="browser-toolbar">
        <el-button size="small" :icon="Back" :disabled="!upTarget" @click="load(upTarget)">上级</el-button>

        <el-breadcrumb class="browser-crumbs" separator="/">
          <el-breadcrumb-item v-for="(b, i) in crumbs" :key="b.path">
            <a v-if="i < crumbs.length - 1" class="crumb-link" @click="load(b.path)">{{ b.label }}</a>
            <span v-else class="crumb-current">{{ b.label }}</span>
          </el-breadcrumb-item>
        </el-breadcrumb>

        <el-button size="small" :icon="Refresh" :loading="loading" @click="load(currentPath)">刷新</el-button>
      </div>

      <div class="browser-body">
        <!-- 错误：直接展示后端文案，界面不崩 -->
        <el-alert v-if="error" type="error" :closable="false" show-icon class="browser-alert">
          <template #title>无法列出该目录</template>
          <div class="alert-text">{{ error }}</div>
          <div class="alert-hint">可点「上级」返回上一层，或点「刷新」重试。</div>
        </el-alert>

        <template v-else>
          <el-alert v-if="fromFilePath" type="info" :closable="false" show-icon class="browser-alert">
            <template #title>已定位到文件所在目录</template>
          </el-alert>

          <el-alert v-if="listing?.truncated" type="warning" :closable="false" show-icon class="browser-alert">
            <template #title>目录过大，仅显示前 1000 条（共 {{ listing?.total }} 项）</template>
          </el-alert>

          <el-table
            v-loading="loading"
            element-loading-text="加载中…"
            :data="entries"
            height="calc(100vh - 210px)"
            row-key="name"
            :row-class-name="rowClass"
            empty-text="这个目录是空的"
            class="browser-table"
            @row-click="onRowClick"
            @row-dblclick="onRowClick"
          >
            <el-table-column label="名称" min-width="300">
              <template #default="{ row }">
                <span class="entry-name">
                  <span class="entry-icon">{{ row.isDir ? '📁' : '📄' }}</span>
                  <span class="entry-label">{{ row.name }}</span>
                </span>
              </template>
            </el-table-column>

            <el-table-column label="大小" width="110" align="right">
              <template #default="{ row }">
                <!-- 目录不显示大小（大小只对文件有意义） -->
                <span v-if="row.isDir" class="muted">--</span>
                <span v-else class="mono">{{ formatSize(row.size) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="修改时间" width="160">
              <template #default="{ row }">
                <span class="mono">{{ formatTime(row.modTime) }}</span>
              </template>
            </el-table-column>
          </el-table>
        </template>
      </div>

      <div class="browser-footer muted">
        只读浏览，不读取文件内容
      </div>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Back, Refresh } from '@element-plus/icons-vue'
import { listDir, type FsEntry, type FsListing } from '../api'
import { normPath, joinPath, parentOf } from '../utils/paths'

const props = defineProps<{ modelValue: boolean; path: string }>()
const emit = defineEmits<{ 'update:modelValue': [v: boolean] }>()

// ── 状态 ──
const listing = ref<FsListing | null>(null) // 最近一次成功的结果
const attemptedPath = ref('') // 最近一次请求的路径（失败时仍用它显示面包屑/上级）
const error = ref('')
const loading = ref(false)

// 只采用最后一次请求的结果，避免连点目录时旧响应覆盖新响应
let reqSeq = 0

const entries = computed<FsEntry[]>(() => listing.value?.entries ?? [])
const currentPath = computed(() => listing.value?.path || attemptedPath.value)

// 起点是文件时后端会自动列出所在目录，这里给个提示
const fromFilePath = computed(() => {
  const l = listing.value
  if (!l) return false
  return normPath(attemptedPath.value) !== normPath(l.path)
})

// 上级：正常用后端给的 parent；出错时按路径自己算（否则错误状态下无法返回）
const upTarget = computed(() => (listing.value ? listing.value.parent : parentOf(attemptedPath.value)))

interface Crumb {
  label: string
  path: string
}

// 面包屑：盘符根（如 D:\）逐级到当前目录，前几级可点
const crumbs = computed<Crumb[]>(() => {
  const p = currentPath.value
  const out: Crumb[] = []
  if (!p) return out
  const m = /^([A-Za-z]:)[\\/]?/.exec(p)
  if (!m) {
    out.push({ label: p, path: p })
    return out
  }
  const drive = m[1]
  let acc = drive + '\\'
  out.push({ label: acc, path: acc })
  const rest = p.slice(m[0].length).split(/[\\/]+/).filter(Boolean)
  for (const seg of rest) {
    acc = acc.endsWith('\\') ? acc + seg : acc + '\\' + seg
    out.push({ label: seg, path: acc })
  }
  return out
})

// ── 加载 ──
async function load(target: string) {
  const p = (target || '').trim()
  attemptedPath.value = p
  if (!p) {
    listing.value = null
    error.value = '路径为空，无法浏览'
    return
  }
  const seq = ++reqSeq
  loading.value = true
  error.value = ''
  try {
    const res = await listDir(p)
    if (seq !== reqSeq) return
    listing.value = res
  } catch (e: any) {
    if (seq !== reqSeq) return
    listing.value = null
    error.value = e?.message || '加载目录失败'
  } finally {
    if (seq === reqSeq) loading.value = false
  }
}

function onRowClick(row: FsEntry) {
  if (row.isDir) load(joinPath(currentPath.value, row.name))
}

function rowClass({ row }: { row: FsEntry }) {
  return row.isDir ? 'entry-row is-dir' : 'entry-row'
}

function onVisibleChange(v: boolean) {
  emit('update:modelValue', v)
}

// ── 格式化 ──
function formatSize(n: number) {
  if (!n || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const text = i === 0 ? String(Math.round(v)) : v >= 100 ? v.toFixed(0) : v.toFixed(1)
  return `${text} ${units[i]}`
}

function formatTime(iso: string) {
  if (!iso) return '--'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const p2 = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p2(d.getMonth() + 1)}-${p2(d.getDate())} ${p2(d.getHours())}:${p2(d.getMinutes())}`
}

// 打开抽屉时重置到传入路径并加载
watch(
  () => props.modelValue,
  (v) => {
    if (!v) return
    listing.value = null
    error.value = ''
    load(props.path)
  },
  { immediate: true },
)
</script>

<style scoped>
.dir-browser {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
}
.browser-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}
.browser-crumbs {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}
.crumb-link {
  color: var(--el-color-primary);
  cursor: pointer;
  text-decoration: none;
}
.crumb-link:hover { text-decoration: underline; }
.crumb-current { color: var(--el-text-color-regular); }
.browser-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.browser-alert { flex-shrink: 0; }
.alert-text {
  font-size: 12px;
  margin-top: 2px;
  word-break: break-all;
}
.alert-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  margin-top: 2px;
}
.entry-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
}
.entry-icon { flex-shrink: 0; }
.entry-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.is-dir .entry-name { font-weight: 500; }
.mono { font-variant-numeric: tabular-nums; }
.muted {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
.browser-footer { flex-shrink: 0; }
:deep(.entry-row.is-dir) { cursor: pointer; }
</style>
