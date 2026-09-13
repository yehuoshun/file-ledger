<template>
  <div class="settings-page">
    <h2 class="page-title">设置</h2>

    <el-form label-width="120px" class="settings-form">
      <el-form-item label="监听端口">
        <el-input v-model="form.port" placeholder="18765" style="width: 200px" />
        <span class="form-hint">重启后生效</span>
      </el-form-item>

      <el-form-item label="挂载盘符">
        <el-input v-model="form.defaultDrive" placeholder="Z:" style="width: 200px" />
        <span class="form-hint">mount 类型默认盘符</span>
      </el-form-item>

      <el-form-item label="数据文件">
        <span class="form-hint">data/registry.json</span>
      </el-form-item>

      <el-form-item label="主题模式">
        <el-radio-group v-model="form.theme">
          <el-radio value="light">浅色</el-radio>
          <el-radio value="dark">暗色</el-radio>
          <el-radio value="auto">跟随系统</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" disabled>保存设置</el-button>
        <span class="form-hint">（后端就绪后启用）</span>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const form = ref({
  port: '18765',
  defaultDrive: 'Z:',
  theme: 'light',
})
</script>

<style scoped>
.settings-page {
  max-width: 560px;
}
.page-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 24px;
}
.settings-form {
  background: var(--el-bg-color);
}
.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
</style>