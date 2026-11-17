// Windows 盘符路径工具：Cloud 与 DirBrowser 共用（路径判断只写这一份）

// 只有盘符绝对路径可浏览；虚拟路径（ali:… / https:…）不在目录浏览范围内
export function canBrowsePath(type: string, path: string): boolean {
  const v = (path || '').trim()
  return type !== 'virtual' && /^[A-Za-z]:[\\/]/.test(v)
}

export function normPath(p: string): string {
  return (p || '').replace(/\//g, '\\').replace(/\\+$/, '').toLowerCase()
}

export function joinPath(base: string, name: string): string {
  if (!base) return name
  return base.endsWith('\\') ? base + name : base + '\\' + name
}

export function parentOf(p: string): string {
  const s = (p || '').replace(/\//g, '\\').replace(/\\+$/, '')
  if (!s) return ''
  if (/^[A-Za-z]:$/.test(s)) return '' // 盘符根没有上级
  const i = s.lastIndexOf('\\')
  if (i <= 0) return ''
  const head = s.slice(0, i)
  return /^[A-Za-z]:$/.test(head) ? head + '\\' : head
}
