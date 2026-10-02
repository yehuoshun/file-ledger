// 路径类型（local/mount/virtual）：单一来源，表格列与新增/编辑弹窗共用
export const PATH_TYPE_LABEL: Record<string, string> = {
  local: '本地',
  mount: '挂载',
  virtual: '虚拟',
}

export const PATH_TYPE_TAG: Record<string, string> = {
  local: '',
  mount: 'warning',
  virtual: 'info',
}

export const PATH_TYPE_OPTIONS = [
  { value: 'local', label: '本地' },
  { value: 'mount', label: '挂载' },
  { value: 'virtual', label: '虚拟' },
]
