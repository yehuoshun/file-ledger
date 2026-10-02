// 标签颜色与标签池：单一来源，Cloud 表格与 TagSelector 共用
// （此前双份定义且逻辑不一致：同一标签在表格/选择器里颜色可能不同）
import { ref } from 'vue'

export const TAG_COLORS: Record<string, { bg: string; text: string }> = {
  '工作': { bg: '#d4eaff', text: '#1565c0' },
  '重要': { bg: '#e6d4f5', text: '#6a1b9a' },
  '娱乐': { bg: '#f5e6d0', text: '#8a5a2a' },
  '文档': { bg: '#d4eaff', text: '#1565c0' },
  '项目': { bg: '#d4eaff', text: '#1565c0' },
  '备份': { bg: '#fcedd4', text: '#8a6d2b' },
}

// 预置语义色之外的轮询池（新标签分配，模块级缓存保证跨组件同色）
const EXTRA_COLORS = [
  { bg: '#fae3c8', text: '#b35d0a' },
  { bg: '#d9f0d9', text: '#2e7d32' },
  { bg: '#fce4ec', text: '#c2185b' },
  { bg: '#fee2e2', text: '#b71c1c' },
]

// 共享标签池（模拟后端），新标签在此登记
export const tagPool = ref<string[]>(Object.keys(TAG_COLORS))

const dynamicColors = new Map<string, { bg: string; text: string }>()
let nextColorIdx = 0

export function getTagColor(name: string): { bg: string; text: string } {
  const preset = TAG_COLORS[name]
  if (preset) return preset
  if (!dynamicColors.has(name)) {
    dynamicColors.set(name, EXTRA_COLORS[nextColorIdx % EXTRA_COLORS.length])
    nextColorIdx++
    if (!tagPool.value.includes(name)) tagPool.value.push(name)
  }
  return dynamicColors.get(name)!
}
