// Windows 盘符路径工具：Cloud 与 DirBrowser 共用（路径判断只写这一份）

// 只有盘符绝对路径可浏览；虚拟路径（ali:… / https:…）不在目录浏览范围内
export function canBrowsePath(type: string, path: string): boolean {
  const v = (path || '').trim()
  return type !== 'virtual' && /^[A-Za-z]:[\\/]/.test(v)
}

// 盘符绝对路径 → 盘符（大写，形如 "D:"）；虚拟路径 / 相对路径 / 空串 → ""
// 注意 D:foo 这种「盘符相对路径」不算绝对路径，必须带分隔符才算
export function driveOf(path: string): string {
  const m = /^([A-Za-z]):[\\/]/.exec((path || '').trim())
  return m ? m[1].toUpperCase() + ':' : ''
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
