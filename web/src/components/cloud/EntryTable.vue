<template>
  <div ref="scrollEl" class="entry-scroll" @scroll.passive="onScroll">
    <el-table
      v-loading="loading"
      :data="rows"
      stripe
      highlight-current-row
      style="width: 100%"
      :empty-text="emptyText"
      @row-dblclick="onRowDblclick"
    >
      <el-table-column label="名字" min-width="140">
        <template #default="{ row }">
          <span v-if="row.isDir" class="dir-name">📁 {{ row.alias }}</span>
          <span v-else>📄 {{ row.alias }}</span>
        </template>
      </el-table-column>

      <el-table-column label="路径" min-width="320">
        <template #default="{ row }">
          <div class="path-tags">
            <el-tooltip
              v-for="(p, i) in row.paths"
              :key="i"
              :disabled="!isOffline(p)"
              :content="offlineTip(p)"
              placement="top"
            >
              <span class="path-item">
                <!-- 路径芯片：点一下就复制（只抛事件，剪贴板由页面处理） -->
                <el-tag
                  :type="PATH_TYPE_TAG[p.type]"
                  size="small"
                  class="path-tag clickable"
                  title="点击复制路径"
                  @click.stop="emit('copy-path', p.path)"
                >
                  <span v-if="isOffline(p)" class="drive-dot offline" />
                  <span v-else-if="isOnline(p)" class="drive-dot online" />
                  {{ PATH_TYPE_LABEL[p.type] }} {{ p.path }}
                  <span v-if="isOffline(p)" class="offline-text">离线</span>
                </el-tag>
                <el-button
                  v-if="canBrowsePath(p.type, p.path)"
                  link
                  type="primary"
                  size="small"
                  :icon="FolderOpened"
                  class="browse-btn"
                  @click.stop="emit('browse', p.path)"
                >
                  浏览
                </el-button>
                <!-- 定位 = 打开所在目录；虚拟路径暂不支持（文件夹双击即进入，不需要） -->
                <el-button
                  v-if="canReveal(row, p)"
                  link
                  type="primary"
                  size="small"
                  :icon="Aim"
                  class="browse-btn"
                  @click.stop="emit('reveal', p.path)"
                >
                  定位
                </el-button>
              </span>
            </el-tooltip>
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
          <span v-for="t in row.tags" :key="t" class="tag-chip colored" :style="tagStyle(t)">{{ t }}</span>
          <span v-if="!row.tags?.length" class="muted">--</span>
        </template>
      </el-table-column>

      <el-table-column label="描述" prop="note" min-width="120" show-overflow-tooltip />

      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link size="small" @click="emit('edit', row)">编辑</el-button>
          <el-button link size="small" type="danger" @click="emit('delete', row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 滚动到底的加载状态：跟着列表一起滚动，不额外占版面 -->
    <div v-if="showFooter" class="load-hint">
      <span v-if="loadingMore">加载中…</span>
      <span v-else-if="!hasMore">已全部加载（共 {{ total }} 条）</span>
      <span v-else>滚动到底部加载更多</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Aim, FolderOpened } from '@element-plus/icons-vue'
import { PATH_TYPE_LABEL, PATH_TYPE_TAG } from '../../constants/pathTypes'
import { useTags } from '../../composables/useTags'
import { useDriveStatus } from '../../composables/useDriveStatus'
import { canBrowsePath, driveOf } from '../../utils/paths'
import type { Entry, Path } from '../../api'

// 台账列表（云盘 = 滚动加载）：只负责展示与抛事件，数据与分页由父级/composable 管
const props = withDefaults(
  defineProps<{
    rows: Entry[]
    loading: boolean
    loadingMore?: boolean
    total?: number
    hasMore?: boolean
    emptyText?: string
  }>(),
  {
    loadingMore: false,
    total: 0,
    hasMore: false,
    emptyText: '空文件夹，点右上角「+ 新增」添加条目',
  },
)

const emit = defineEmits<{
  'row-dblclick': [Entry]
  edit: [Entry]
  delete: [Entry]
  browse: [string]
  'copy-path': [string]
  reveal: [string]
  'load-more': []
}>()

// 距底小于这个像素就触发加载，别等真滚到最后一像素
const NEAR_BOTTOM = 80

const scrollEl = ref<HTMLElement | null>(null)

const showFooter = computed(() => props.rows.length > 0 || props.loadingMore)

function onRowDblclick(row: Entry) {
  emit('row-dblclick', row)
}

function onScroll(e: Event) {
  const el = e.target as HTMLElement | null
  if (!el) return
  if (el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM) emit('load-more')
}

// 首屏不满一屏时没有滚动条，滚动事件永远不触发：数据变化后补一次「填满视口」。
// 用「上一轮自动补齐时的条数」兜底，避免后端一直返回空页时无限请求。
let autoFilledAt = -1

function fillViewport() {
  const el = scrollEl.value
  if (!el) return
  // 列表被清空（换文件夹/改搜索）时把兜底计数归零，新的一屏还能自动补齐
  if (!props.rows.length) {
    autoFilledAt = -1
    return
  }
  if (props.loading || props.loadingMore || !props.hasMore) return
  if (el.scrollHeight - el.clientHeight >= NEAR_BOTTOM) return
  if (props.rows.length === autoFilledAt) return
  autoFilledAt = props.rows.length
  emit('load-more')
}

watch(
  () => [props.rows, props.hasMore, props.loading, props.loadingMore] as const,
  () => {
    nextTick(fillViewport)
  },
  { immediate: true },
)

// 换文件夹/改搜索时，列表回到顶部（父级调这个方法）
function scrollToTop() {
  scrollEl.value?.scrollTo({ top: 0 })
}

defineExpose({ scrollToTop })

// 标签颜色不预加载整本字典（可能上千条）：只把表格里出现过的标签名按需批量取一次，
// 缓存是模块级单例，多次渲染/多组件共享；取到前先渲染兜底灰。
const { colorOf, ensureColors } = useTags()

watch(
  () => props.rows,
  (rows) => {
    const names: string[] = []
    for (const row of rows) {
      for (const t of row.tags || []) names.push(t)
    }
    ensureColors(names)
  },
  { immediate: true },
)

function tagStyle(name: string) {
  const c = colorOf(name)
  return { background: c.bg, color: c.fg }
}

// ── 盘符在线状态（步骤5）──
// 只对 local/mount 的盘符绝对路径显示状态点：虚拟路径没有盘符概念。
// 缓存与批量查询都在 composables/useDriveStatus.ts，这里只读、只抛事件。
const { onlineOf, infoOf, ensureDrives } = useDriveStatus()

function driveFor(p: Path): string {
  if (p.type !== 'local' && p.type !== 'mount') return ''
  return driveOf(p.path)
}

function isOnline(p: Path): boolean {
  const d = driveFor(p)
  return d ? onlineOf(d) === true : false
}

function isOffline(p: Path): boolean {
  const d = driveFor(p)
  return d ? onlineOf(d) === false : false
}

function offlineTip(p: Path): string {
  const d = driveFor(p)
  const s = d ? infoOf(d) : undefined
  if (!d || !s || s.online) return ''
  return s.reason ? `盘符 ${d} 离线（${s.reason}）` : `盘符 ${d} 离线`
}

// 定位按钮：文件条目 + local/mount 盘符路径才有；虚拟路径不显示（暂不支持定位）
function canReveal(row: Entry, p: Path): boolean {
  return !row.isDir && !!driveFor(p)
}

// 列表变化（换文件夹/滚动加载）时，把当前行里出现的盘符按需补一次状态
watch(
  () => props.rows,
  (rows) => {
    const drives: string[] = []
    for (const row of rows) {
      for (const p of row.paths || []) drives.push(driveFor(p))
    }
    ensureDrives(drives)
  },
  { immediate: true },
)
</script>

<style scoped>
/* 固定高度 + 内部滚动：滚动条在这个框里，触底就向父级要下一屏 */
.entry-scroll {
  flex: 1 1 auto;
  min-height: 0;
  max-height: calc(100vh - 96px);
  overflow-y: auto;
}
.dir-name { font-weight: 500; }
.path-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.path-item { display: inline-flex; align-items: center; gap: 2px; }
.path-tag { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.path-tag.clickable { cursor: pointer; }
.path-tag.clickable:hover { opacity: 0.85; }
.browse-btn { padding: 0 2px; }
/* 盘符在线状态点：绿=在线，红=离线（未知不显示） */
.drive-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
  vertical-align: middle;
}
.drive-dot.online { background: var(--el-color-success); }
.drive-dot.offline { background: var(--el-color-danger); }
.offline-text { margin-left: 4px; color: var(--el-color-danger); font-size: 12px; }
.tag-chip { margin-right: 4px; display: inline-block; }
.tag-chip.colored { padding: 1px 6px; border-radius: 3px; font-size: 12px; margin-right: 4px; }
.muted { color: var(--el-text-color-placeholder); font-size: 12px; }
.load-hint {
  display: flex;
  justify-content: center;
  padding: 12px 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
