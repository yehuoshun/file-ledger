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
            <span v-for="(p, i) in row.paths" :key="i" class="path-item">
              <el-tag :type="PATH_TYPE_TAG[p.type]" size="small" class="path-tag">
                {{ PATH_TYPE_LABEL[p.type] }} {{ p.path }}
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
            </span>
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
import { FolderOpened } from '@element-plus/icons-vue'
import { PATH_TYPE_LABEL, PATH_TYPE_TAG } from '../../constants/pathTypes'
import { useTags } from '../../composables/useTags'
import { canBrowsePath } from '../../utils/paths'
import type { Entry } from '../../api'

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
.browse-btn { padding: 0 2px; }
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
