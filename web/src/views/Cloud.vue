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

      <el-button type="primary" size="default" @click="handleAdd">+ 新增</el-button>
    </div>

    <el-table :data="filtered" stripe style="width: 100%">
      <el-table-column label="名字" prop="alias" min-width="120" />

      <el-table-column label="路径" min-width="360">
        <template #default="{ row }">
          <div class="path-tags">
            <el-tag
              v-for="(p, i) in row.paths"
              :key="i"
              :type="tagType[p.type]"
              size="small"
              class="path-tag"
            >
              {{ typeLabel[p.type] }} {{ p.path }}
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

      <el-table-column label="描述" prop="note" min-width="120" show-overflow-tooltip />

      <el-table-column label="操作" width="100" fixed="right">
        <template #default>
          <el-button link size="small">编辑</el-button>
          <el-button link size="small" type="danger">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const typeLabel: Record<string, string> = { local: '本地', mount: '挂载', virtual: '虚拟' }
const tagType: Record<string, string> = { local: '', mount: 'warning', virtual: 'info' }

const entries = [
  {
    alias: '文档',
    isDir: true,
    size: '1.2 GB',
    note: '项目文档',
    paths: [
      { type: 'local', path: 'D:\\docs' },
      { type: 'mount', path: 'Z:\\docs' },
      { type: 'virtual', path: 'ali:docs' },
    ],
  },
  {
    alias: '游戏盘',
    isDir: true,
    size: '128 GB',
    note: 'NAS 挂载',
    paths: [
      { type: 'mount', path: 'Z:' },
    ],
  },
  {
    alias: 'config.yaml',
    isDir: false,
    size: '4 KB',
    note: '',
    paths: [
      { type: 'local', path: 'D:\\config.yaml' },
      { type: 'virtual', path: 'ali:config' },
    ],
  },
]

const search = ref('')
const dirFilter = ref<boolean | null>(null)

const filtered = computed(() =>
  entries.filter((e) => {
    if (search.value && !e.alias.toLowerCase().includes(search.value.toLowerCase()) && !e.note.toLowerCase().includes(search.value.toLowerCase())) return false
    if (dirFilter.value !== null && e.isDir !== dirFilter.value) return false
    return true
  })
)

function handleAdd() {
  // 等后端就绪
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
.search-input {
  width: 200px;
}
.path-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.path-tag {
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>