<template>
  <aside class="tree-panel">
    <div class="tree-header">
      <span class="tree-title">虚拟网盘</span>
      <el-button link type="primary" size="small" @click="emit('add')">+ 新建文件夹</el-button>
    </div>
    <el-tree
      ref="treeRef"
      :data="data"
      node-key="id"
      :props="{ label: 'alias', children: 'children' }"
      default-expand-all
      highlight-current
      @node-click="onNodeClick"
    />
    <div v-if="!data.length" class="tree-empty muted">还没有文件夹，点「+ 新建文件夹」</div>
  </aside>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { TreeNode } from '../../composables/useFolderTree'

// 左侧文件夹树：只负责渲染与点击，不碰数据与业务（数据由父级传入）
defineProps<{ data: TreeNode[] }>()

const emit = defineEmits<{
  select: [string]
  add: []
}>()

const treeRef = ref<{ setCurrentKey: (key: string | null) => void } | null>(null)

function onNodeClick(node: TreeNode) {
  emit('select', node.id)
}

// 进入文件夹时同步高亮，避免「列表已经在里面、树还停在原处」
function setCurrentKey(id: string | null) {
  treeRef.value?.setCurrentKey(id)
}

defineExpose({ setCurrentKey })
</script>

<style scoped>
.tree-panel {
  width: 220px;
  flex-shrink: 0;
  border-right: 1px solid var(--el-border-color-light);
  padding-right: 12px;
  overflow: auto;
}
.tree-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.tree-title {
  font-weight: 600;
  font-size: 14px;
}
.tree-empty {
  font-size: 12px;
  padding: 8px 0;
}
.muted {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
</style>
