<template>
  <el-table :data="entries" stripe style="width: 100%">
    <el-table-column label="别名" prop="alias" min-width="140" />
    <el-table-column label="类型" width="100">
      <template #default="{ row }">
        <el-tag :type="tagType[row.type]" size="small">{{ typeLabel[row.type] }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="路径" min-width="220">
      <template #default="{ row }">
        {{ row.type === 'virtual' ? row.relPath : row.root }}
      </template>
    </el-table-column>
    <el-table-column label="状态" width="80">
      <template #default="{ row }">
        <span class="dot" :class="row.status === 'online' ? 'on' : row.status === 'offline' ? 'off' : 'v'"></span>
      </template>
    </el-table-column>
    <el-table-column label="备注" prop="note" min-width="120" show-overflow-tooltip />
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
  { alias: '文档', type: 'local', root: 'D:\\docs', relPath: '', status: 'online', note: '项目文档' },
  { alias: '游戏盘', type: 'mount', root: 'Z:', relPath: '', status: 'online', note: 'NAS 挂载' },
  { alias: '备份网盘', type: 'virtual', root: '', relPath: 'ali:backup', status: 'virtual', note: '阿里云盘' },
]
</script>

<style scoped>
.dot {
  display: inline-block;
  width: 8px; height: 8px; border-radius: 50%;
}
.on { background: #67c23a; }
.off { background: #f56c6c; }
.v { background: #909399; }
</style>