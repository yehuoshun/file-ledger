<template>
  <el-table
    v-loading="loading"
    :data="rows"
    stripe
    highlight-current-row
    style="width: 100%"
    empty-text="空文件夹，点右上角「+ 新增」添加条目"
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
        <span v-for="t in row.tags" :key="t" class="tag-chip colored" :style="getTagColor(t)">{{ t }}</span>
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
</template>

<script setup lang="ts">
import { FolderOpened } from '@element-plus/icons-vue'
import { PATH_TYPE_LABEL, PATH_TYPE_TAG } from '../../constants/pathTypes'
import { getTagColor } from '../../constants/tags'
import { canBrowsePath } from '../../utils/paths'
import type { Entry } from '../../api'

// 台账列表：只负责展示与抛事件，双击/编辑/删除/浏览都由父级处理
defineProps<{ rows: Entry[]; loading: boolean }>()

const emit = defineEmits<{
  'row-dblclick': [Entry]
  edit: [Entry]
  delete: [Entry]
  browse: [string]
}>()

function onRowDblclick(row: Entry) {
  emit('row-dblclick', row)
}
</script>

<style scoped>
.dir-name { font-weight: 500; }
.path-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.path-item { display: inline-flex; align-items: center; gap: 2px; }
.path-tag { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.browse-btn { padding: 0 2px; }
.tag-chip { margin-right: 4px; display: inline-block; }
.tag-chip.colored { padding: 1px 6px; border-radius: 3px; font-size: 12px; margin-right: 4px; }
.muted { color: var(--el-text-color-placeholder); font-size: 12px; }
</style>
