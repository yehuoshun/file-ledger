<script setup>
import { ref } from 'vue'
import { Folder, Notebook, Picture } from '@element-plus/icons-vue'
import Registry from './views/Registry.vue'

const activeMenu = ref('registry')
</script>

<template>
  <el-container class="app-layout">
    <el-aside width="200px" class="app-aside">
      <div class="app-logo">📒 文件台账</div>
      <el-menu :default-active="activeMenu" class="app-menu">
        <el-menu-item index="registry">
          <el-icon><Notebook /></el-icon>
          <span>台账管理</span>
        </el-menu-item>
        <el-menu-item index="fs" disabled>
          <el-icon><Folder /></el-icon>
          <span>目录浏览（批次1-4）</span>
        </el-menu-item>
        <el-menu-item index="images" disabled>
          <el-icon><Picture /></el-icon>
          <span>图片预览（批次2）</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-main class="app-main">
      <Registry />
    </el-main>
  </el-container>
</template>

<style scoped>
.app-layout {
  height: 100vh;
}
.app-aside {
  border-right: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
}
.app-logo {
  height: 56px;
  line-height: 56px;
  text-align: center;
  font-weight: 600;
  font-size: 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.app-menu {
  border-right: none;
}
.app-main {
  background: var(--el-bg-color-page);
  padding: 20px;
}
</style>
