<template>
  <div class="tag-selector" @click="open = !open" v-click-outside="() => (open = false)">
    <!-- 已选标签 -->
    <div class="tag-chips" v-if="modelValue.length || open">
      <span
        v-for="t in modelValue"
        :key="t"
        class="tag-chip"
        :style="{ background: getColor(t).bg, color: getColor(t).text }"
      >
        {{ t }}
        <span class="tag-remove" @click.stop="remove(t)">×</span>
      </span>
      <input
        ref="inputRef"
        v-model="q"
        :placeholder="modelValue.length ? '' : '添加标签…'"
        class="tag-input"
        @keydown.enter.prevent="createTag"
        @keydown.backspace="handleBackspace"
        @focus="open = true"
      />
    </div>
    <div v-else class="tag-placeholder" @click="open = true">
      添加标签…
    </div>

    <!-- 下拉面板 -->
    <div v-if="open" class="tag-dropdown">
      <div v-if="filteredTags.length === 0 && q" class="tag-dropdown-item create" @click="createTag">
        创建「{{ q }}」
      </div>
      <div
        v-for="t in filteredTags"
        :key="t"
        class="tag-dropdown-item"
        :class="{ selected: modelValue.includes(t) }"
        @click="toggle(t)"
      >
        <span class="tag-dot" :style="{ background: getColor(t).bg }"></span>
        {{ t }}
        <span v-if="modelValue.includes(t)" class="check">✓</span>
      </div>
      <div v-if="filteredTags.length === 0 && !q" class="tag-dropdown-empty">暂无标签</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = defineProps<{ modelValue: string[] }>()
const emit = defineEmits<{ 'update:modelValue': [v: string[]] }>()

const open = ref(false)
const q = ref('')
const inputRef = ref<HTMLInputElement>()

// ── tag pool（共享，模拟后端搜索）──
const COLORS = [
  { bg: '#e8e8e8', text: '#555' },
  { bg: '#f5e6d0', text: '#8a5a2a' },
  { bg: '#fae3c8', text: '#b35d0a' },
  { bg: '#fcedd4', text: '#8a6d2b' },
  { bg: '#d9f0d9', text: '#2e7d32' },
  { bg: '#d4eaff', text: '#1565c0' },
  { bg: '#e6d4f5', text: '#6a1b9a' },
  { bg: '#fce4ec', text: '#c2185b' },
  { bg: '#fee2e2', text: '#b71c1c' },
]

const tagColorMap = ref<Record<string, number>>({
  '工作': 0, '重要': 5, '娱乐': 1, '文档': 5, '备份': 3, '项目': 5,
})

let colorIdx = Object.keys(tagColorMap.value).length % COLORS.length

function getColor(name: string) {
  if (!(name in tagColorMap.value)) {
    tagColorMap.value[name] = colorIdx
    colorIdx = (colorIdx + 1) % COLORS.length
  }
  return COLORS[tagColorMap.value[name]]
}

// 所有标签（模拟后端池）
const allTags = computed(() => Object.keys(tagColorMap.value).sort())

const filteredTags = computed(() => {
  if (!q.value) return allTags.value
  const lower = q.value.toLowerCase()
  return allTags.value.filter((t) => t.toLowerCase().includes(lower))
})

function toggle(t: string) {
  const current = [...props.modelValue]
  const idx = current.indexOf(t)
  if (idx >= 0) current.splice(idx, 1)
  else current.push(t)
  emit('update:modelValue', current)
  q.value = ''
}

function remove(t: string) {
  const current = [...props.modelValue]
  const idx = current.indexOf(t)
  if (idx >= 0) current.splice(idx, 1)
  emit('update:modelValue', current)
}

function createTag() {
  const name = q.value.trim()
  if (!name) return
  const current = [...props.modelValue]
  if (!current.includes(name)) {
    current.push(name)
    emit('update:modelValue', current)
  }
  q.value = ''
}

function handleBackspace() {
  if (!q.value && props.modelValue.length) {
    remove(props.modelValue[props.modelValue.length - 1])
  }
}

// 打开时聚焦输入框
watch(open, (v) => {
  if (v) setTimeout(() => inputRef.value?.focus(), 50)
})

// 点击外部关闭
const vClickOutside = {
  mounted(el: HTMLElement, bind: any) {
    const handler = (e: MouseEvent) => {
      if (!el.contains(e.target as Node)) bind.value()
    }
    ;(el as any)._co = handler
    document.addEventListener('click', handler)
  },
  unmounted(el: HTMLElement) {
    document.removeEventListener('click', (el as any)._co)
  },
}
</script>

<style scoped>
.tag-selector {
  position: relative;
  min-height: 32px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  padding: 2px 6px;
  cursor: text;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}
.tag-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  width: 100%;
}
.tag-chip {
  font-size: 12px;
  padding: 1px 6px;
  border-radius: 3px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.tag-remove {
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  opacity: 0.6;
}
.tag-remove:hover { opacity: 1; }
.tag-input {
  border: none;
  outline: none;
  font-size: 13px;
  min-width: 80px;
  flex: 1;
  background: transparent;
  padding: 2px 0;
}
.tag-placeholder {
  color: var(--el-text-color-placeholder);
  font-size: 13px;
  padding: 4px 0;
}

.tag-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  z-index: 100;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  margin-top: 2px;
  max-height: 200px;
  overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0,0,0,.1);
}
.tag-dropdown-item {
  padding: 6px 10px;
  font-size: 13px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
}
.tag-dropdown-item:hover { background: var(--el-fill-color-light); }
.tag-dropdown-item.selected { font-weight: 600; }
.tag-dropdown-item.create { color: var(--el-color-primary); }
.tag-dot {
  width: 10px; height: 10px; border-radius: 50%;
  flex-shrink: 0;
}
.check { margin-left: auto; color: var(--el-color-primary); }
.tag-dropdown-empty {
  padding: 10px;
  color: var(--el-text-color-placeholder);
  font-size: 12px;
  text-align: center;
}
</style>