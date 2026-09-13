<template>
  <el-container class="app-layout">
    <el-header class="top-bar">
      <span class="logo">🔖 file-ledger</span>
      <div class="top-actions">
        <el-input
          v-model="search"
          placeholder="搜索别名…"
          clearable
          size="small"
          class="search-input"
        />
        <el-button type="primary" size="small" @click="handleAdd">+ 新增</el-button>
      </div>
    </el-header>

    <el-container class="body">
      <el-aside width="280px" class="sidebar">
        <div class="sidebar-inner">
          <div class="filter-tabs">
            <span
              v-for="t in typeFilters"
              :key="t.value"
              class="filter-tab"
              :class="{ active: activeFilter === t.value }"
              @click="activeFilter = t.value"
            >{{ t.label }}</span>
          </div>

          <div class="entry-list">
            <div
              v-for="entry in filteredEntries"
              :key="entry.id"
              class="entry-item"
              :class="{ selected: selectedId === entry.id }"
              @click="selectedId = entry.id"
            >
              <div class="entry-icon">{{ typeIcon[entry.type] }}</div>
              <div class="entry-info">
                <div class="entry-alias">{{ entry.alias }}</div>
                <div class="entry-meta">
                  <span class="status-dot" :class="statusClass(entry)"></span>
                  <el-tag :type="tagType[entry.type]" size="small">{{ typeLabel[entry.type] }}</el-tag>
                </div>
              </div>
              <div class="entry-actions" @click.stop>
                <el-button link size="small" @click="handleEdit(entry)">✏️</el-button>
                <el-button link size="small" type="danger" @click="handleDelete(entry)">🗑️</el-button>
              </div>
            </div>

            <div v-if="filteredEntries.length === 0" class="empty-sidebar">
              <p>暂无条目</p>
            </div>
          </div>

          <div class="sidebar-footer">
            <el-button text size="small" @click="handleAdd">+ 新增路径</el-button>
          </div>
        </div>
      </el-aside>

      <el-main class="content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

// ── mock data ──
interface Entry {
  id: string
  alias: string
  type: 'local' | 'mount' | 'virtual'
  root: string
  status: 'online' | 'offline' | 'virtual'
  note: string
}

const entries = ref<Entry[]>([
  { id: '1', alias: '文档', type: 'local', root: 'D:\\docs', status: 'online', note: '项目文档' },
  { id: '2', alias: '游戏盘', type: 'mount', root: 'Z:', status: 'online', note: 'NAS 挂载' },
  { id: '3', alias: '备份网盘', type: 'virtual', root: 'ali:backup', status: 'virtual', note: '阿里云盘' },
  { id: '4', alias: '音乐库', type: 'local', root: 'E:\\music', status: 'offline', note: '' },
])

const search = ref('')
const activeFilter = ref('all')
const selectedId = ref<string | null>(null)

const typeFilters = [
  { value: 'all', label: '全部' },
  { value: 'local', label: '本地' },
  { value: 'mount', label: '挂载' },
  { value: 'virtual', label: '虚拟' },
]

const typeLabel: Record<string, string> = { local: '本地', mount: '挂载', virtual: '虚拟' }
const typeIcon: Record<string, string> = { local: '📁', mount: '💾', virtual: '☁️' }
const tagType: Record<string, string> = { local: '', mount: 'warning', virtual: 'info' }

const filteredEntries = computed(() => {
  let list = entries.value
  if (activeFilter.value !== 'all') {
    list = list.filter((e) => e.type === activeFilter.value)
  }
  if (search.value) {
    list = list.filter((e) => e.alias.includes(search.value))
  }
  return list
})

function statusClass(entry: Entry) {
  return entry.status === 'online' ? 'dot-online' : entry.status === 'offline' ? 'dot-offline' : 'dot-virtual'
}

function handleAdd() {
  // 步骤3 骨架：占位
}

function handleEdit(_entry: Entry) {
  // 步骤3 骨架：占位
}

function handleDelete(_entry: Entry) {
  // 步骤3 骨架：占位
}
</script>

<style scoped>
.app-layout {
  height: 100vh;
}

/* ── top bar ── */
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  padding: 0 16px;
  height: 48px;
}
.logo {
  font-size: 15px;
  font-weight: 600;
}
.top-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.search-input {
  width: 200px;
}

/* ── sidebar ── */
.sidebar {
  border-right: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color-page);
}
.sidebar-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.filter-tabs {
  display: flex;
  gap: 4px;
  padding: 12px 12px 8px;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.filter-tab {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--el-text-color-secondary);
}
.filter-tab.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.entry-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}
.entry-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  border-radius: 4px;
  margin: 0 4px;
  transition: background 0.15s;
}
.entry-item:hover {
  background: var(--el-fill-color-light);
}
.entry-item.selected {
  background: var(--el-color-primary-light-9);
}
.entry-icon {
  font-size: 18px;
  width: 24px;
  text-align: center;
  flex-shrink: 0;
}
.entry-info {
  flex: 1;
  min-width: 0;
}
.entry-alias {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.entry-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 2px;
}
.entry-actions {
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}
.entry-item:hover .entry-actions {
  opacity: 1;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot-online { background: #67c23a; }
.dot-offline { background: #f56c6c; }
.dot-virtual { background: #909399; }

.sidebar-footer {
  border-top: 1px solid var(--el-border-color-light);
  padding: 8px 12px;
}
.empty-sidebar {
  text-align: center;
  color: var(--el-text-color-placeholder);
  padding: 24px 12px;
  font-size: 13px;
}

.content {
  padding: 0;
  background: var(--el-bg-color);
}
</style>