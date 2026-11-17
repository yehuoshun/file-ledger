<template>
  <el-container class="app-layout">
    <el-aside width="200px" class="sidebar">
      <div class="logo-area">
        <span class="logo">🔖 file-ledger</span>
      </div>
      <el-menu
        :default-active="route.path"
        router
        class="nav-menu"
      >
        <el-menu-item index="/dashboard">
          <el-icon><HomeFilled /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>

        <el-menu-item index="/cloud">
          <el-icon><FolderOpened /></el-icon>
          <span>虚拟网盘</span>
        </el-menu-item>
        <el-sub-menu index="settings">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>设置</span>
          </template>
          <el-menu-item index="/settings/general">
            <el-icon><Operation /></el-icon>
            <span>基本设置</span>
          </el-menu-item>
          <el-menu-item index="/settings/tags">
            <el-icon><PriceTag /></el-icon>
            <span>标签管理</span>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>

    <el-container>
      <el-main class="content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { HomeFilled, FolderOpened, Setting, Operation, PriceTag } from '@element-plus/icons-vue'

const route = useRoute()
</script>

<style scoped>
.app-layout {
  height: 100vh;
}
.sidebar {
  border-right: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color-page);
  display: flex;
  flex-direction: column;
}
.logo-area {
  padding: 16px;
  border-bottom: 1px solid var(--el-border-color-light);
}
.logo {
  font-size: 15px;
  font-weight: 600;
}
.nav-menu {
  border-right: none;
}
.content {
  padding: 24px;
  background: var(--el-bg-color);
}
</style>