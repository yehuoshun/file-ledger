<template>
  <div ref="rootRef" class="tag-selector" @click="open = true">
    <div class="tag-chips">
      <span
        v-for="t in tags"
        :key="t"
        class="tag-chip"
        :style="{ background: colorOf(t).bg, color: colorOf(t).fg }"
      >
        {{ t }}
        <span class="tag-remove" @click.stop="remove(t)">×</span>
      </span>
      <input
        ref="inputRef"
        v-model="q"
        :placeholder="tags.length ? '' : '添加标签…'"
        class="tag-input"
        @keydown="onKeydown"
        @focus="open = true"
        @click.stop="open = true"
      />
    </div>

    <!-- 下拉面板：点面板内任何地方都不关闭（关闭只看点外部） -->
    <div v-if="open" class="tag-dropdown" @click.stop>
      <div v-if="canCreate" class="tag-dropdown-item create" @click="createTag">
        创建「{{ q.trim() }}」
      </div>
      <div
        v-for="t in filteredTags"
        :key="t"
        class="tag-dropdown-item"
        :class="{ selected: tags.includes(t) }"
        @click="toggle(t)"
      >
        <span class="tag-dot" :style="{ background: colorOf(t).bg }"></span>
        {{ t }}
        <span v-if="tags.includes(t)" class="check">✓</span>
      </div>
      <div v-if="!canCreate && !filteredTags.length" class="tag-dropdown-empty">输入新标签后回车创建</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useTags } from '../composables/useTags'

// 标签选择器：候选池与颜色都来自后端标签字典（composable 单例），组件自己不发请求
const props = defineProps<{ modelValue: string[] }>()
const emit = defineEmits<{ 'update:modelValue': [v: string[]] }>()

const { tagNames, colorOf, addTag, reload: reloadTags } = useTags()

const tags = computed(() => props.modelValue || [])
const open = ref(false)
const q = ref('')
const rootRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)

// 候选 = 字典里的标签 + 当前已选（保证已选标签也能在面板里看到并取消）
const allTags = computed(() =>
  [...new Set([...tagNames.value, ...tags.value])].sort((a, b) => a.localeCompare(b, 'zh')),
)

const filteredTags = computed(() => {
  const key = q.value.trim().toLowerCase()
  const list = allTags.value
  return key ? list.filter((t) => t.toLowerCase().includes(key)) : list
})

// 输入了池里没有的新名字 → 面板顶部出现「创建「xxx」」
const canCreate = computed(() => {
  const key = q.value.trim()
  return !!key && !allTags.value.includes(key)
})

function commit(next: string[]) {
  emit('update:modelValue', next)
}

function toggle(t: string) {
  const cur = [...tags.value]
  const i = cur.indexOf(t)
  if (i >= 0) cur.splice(i, 1)
  else cur.push(t)
  commit(cur)
  q.value = ''
  focusInput()
}

function remove(t: string) {
  const cur = [...tags.value]
  const i = cur.indexOf(t)
  if (i >= 0) {
    cur.splice(i, 1)
    commit(cur)
  }
}

// 新建标签：先进当前条目（本地立即生效），再 best-effort 登记进后端字典。
// 409 = 后端已有同名标签，忽略即可；其他失败也只刷新候选池，不打断录入
// （保存条目时后端会自动补登记，所以这里失败不影响落库）。
async function createTag() {
  const name = q.value.trim()
  if (!name) return
  if (!tags.value.includes(name)) commit([...tags.value, name])
  q.value = ''
  focusInput()
  try {
    await addTag(name)
  } catch {
    await reloadTags()
  }
}

// 键盘：回车/逗号建标签；空输入退格删最后一个；ESC 收起。
// isComposing = 中文输入法还在选词，此时回车交给输入法，别误建标签
function onKeydown(e: KeyboardEvent) {
  if (e.isComposing) return
  if (e.key === 'Enter') {
    if (!q.value.trim()) return
    e.preventDefault()
    createTag()
    return
  }
  if (e.key === ',') {
    if (!q.value.trim()) return
    e.preventDefault()
    createTag()
    return
  }
  if (e.key === 'Backspace' && !q.value && tags.value.length) {
    remove(tags.value[tags.value.length - 1])
    return
  }
  if (e.key === 'Escape') open.value = false
}

function focusInput() {
  nextTick(() => inputRef.value?.focus())
}

// 面板打开时刷新候选池（单例内部会合并并发请求，重开不会堆请求）
watch(open, (v) => {
  if (!v) return
  focusInput()
  reloadTags()
})

// 点外部收起（捕获阶段先判，点面板/输入框内不关闭）
function onDocClick(e: MouseEvent) {
  if (!open.value) return
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) open.value = false
}
onMounted(() => document.addEventListener('click', onDocClick, true))
onUnmounted(() => document.removeEventListener('click', onDocClick, true))
</script>

<style scoped>
.tag-selector {
  position: relative;
  min-height: 32px;
  width: 100%;
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

.tag-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  z-index: 3000;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  margin-top: 2px;
  max-height: 200px;
  overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
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
  width: 10px;
  height: 10px;
  border-radius: 50%;
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
