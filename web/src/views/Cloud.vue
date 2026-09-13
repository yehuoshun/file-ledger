<template>
  <el-table :data="entries" stripe style="width: 100%">
    <el-table-column label="名字" prop="alias" min-width="140" />
    <el-table-column label="标签" width="90">
      <template #default="{ row }">
        <el-tag :type="tagType[row.type]" size="small">{{ typeLabel[row.type] }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="路径" min-width="240" show-overflow-tooltip>
      <template #default="{ row }">
        {{ row.type === 'virtual' ? row.relPath : row.root }}
      </template>
    </el-table-column>
    <el-table-column label="文件夹" width="80" align="center">
      <template #default="{ row }">
        <span>{{ row.isDir ? '📁' : '📄' }}</span>
      </template>
    </el-table-column>
    <el-table-column label="大小" width="100" align="right">
      <template #default="{ row }">
        <span v-if="row.size">{{ row.size }}</span>
        <span v-else class="muted">--</span>
      </template>
    </el-table-column>
    <el-table-column label="描述" prop="note" min-width="140" show-overflow-tooltip />
    <el-table-column label="操作" width="120" fixed="right">
      <template #default>
        <el-button link size="small">编辑</el-button>
        <el-button link size="small" type="danger">删除</el-button>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
const typeLabel: Record<string, string> = { local: '本地', mount: '挂载', virtual: '虚拟' }
const tagType: Record<string, string> = { local: '', mount: 'warning', virtual: 'info' }

const entries = [
  { alias: '文档', type: 'local', root: 'D:\\docs', relPath: '', isDir: true, size: '1.2 GB', note: '项目文档' },
  { alias: '游戏盘', type: 'mount', root: 'Z:', relPath: '', isDir: true, size: '128 GB', note: 'NAS 挂载' },
  { alias: 'config.yaml', type: 'local', root: 'D:\\config.yaml', relPath: '', isDir: false, size: '4 KB', note: '' },
  { alias: '备份网盘', type: 'virtual', root: '', relPath: 'ali:backup', isDir: true, size: '', note: '阿里云盘' },
]
</script>

<style scoped>
.muted { color: var(--el-text-color-placeholder); }
</style>