<template>
  <el-container>
    <el-header class="header">
      <h1 class="title">file-ledger</h1>
    </el-header>
    <el-main>
      <router-view />
    </el-main>
  </el-container>
</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--el-border-color-light);
}
.title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}
.el-main {
  padding: 20px;
}
</style>