<template>
  <el-breadcrumb separator="/">
    <el-breadcrumb-item v-for="(b, i) in crumbs" :key="b.id ?? 'root'">
      <a v-if="i < crumbs.length - 1" @click="emit('navigate', b.id)">{{ b.label }}</a>
      <span v-else>{{ b.label }}</span>
    </el-breadcrumb-item>
  </el-breadcrumb>
</template>

<script setup lang="ts">
// 面包屑：首项固定「虚拟网盘」，其余由父级按 parentId 上溯算出
defineProps<{ crumbs: { id: string | null; label: string }[] }>()

const emit = defineEmits<{ navigate: [string | null] }>()
</script>
