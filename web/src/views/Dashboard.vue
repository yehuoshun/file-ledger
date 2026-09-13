<template>
  <div class="dashboard">
    <div class="card-row">
      <el-card class="stat-card"><div class="stat-num">3</div><div class="stat-label">总路径</div></el-card>
      <el-card class="stat-card"><div class="stat-num">1</div><div class="stat-label">本地</div></el-card>
      <el-card class="stat-card"><div class="stat-num">1</div><div class="stat-label">挂载</div></el-card>
      <el-card class="stat-card"><div class="stat-num">1</div><div class="stat-label">虚拟</div></el-card>
    </div>

    <div class="table-section">
      <div class="section-header">
        <h3>路径列表</h3>
        <el-button size="small" type="primary">+ 新增</el-button>
      </div>

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
            <span class="stat-dot" :class="row.status === 'online' ? 'dot-on' : row.status === 'offline' ? 'dot-off' : 'dot-v'"></span>
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
    </div>
  </div>
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
.card-row {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}
.stat-card {
  flex: 1;
  text-align: center;
}
.stat-num {
  font-size: 28px;
  font-weight: 700;
  color: var(--el-color-primary);
}
.stat-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.section-header h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
}
.stat-dot {
  display: inline-block;
  width: 8px; height: 8px; border-radius: 50%;
}
.dot-on { background: #67c23a; }
.dot-off { background: #f56c6c; }
.dot-v { background: #909399; }
.table-section {
  background: var(--el-bg-color-overlay);
  padding: 16px;
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
}
</style>