// 展示层用的格式化与文案映射，两个前端共用。

export function fmtDate(value, withTime = false) {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  const pad = (n) => String(n).padStart(2, '0')
  const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  if (!withTime) return date
  return `${date} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function fmtTime(value) {
  return fmtDate(value, true)
}

export function pct(v) {
  if (v === undefined || v === null) return '0%'
  return `${Math.round(Number(v) * 100)}%`
}

export function fmtSize(bytes) {
  const n = Number(bytes || 0)
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

export const docStatus = {
  parsing: { text: '解析中', type: 'warning' },
  pending_review: { text: '待审核', type: 'primary' },
  published: { text: '已发布', type: 'success' },
  rejected: { text: '已驳回', type: 'info' },
  failed: { text: '解析失败', type: 'danger' },
}

export const issueStatus = {
  reported: { text: '已上报', type: 'warning' },
  accepted: { text: '已受理', type: 'primary' },
  rejected: { text: '已驳回', type: 'info' },
  rectified: { text: '待验收', type: 'warning' },
  verified: { text: '已验收', type: 'success' },
  closed: { text: '已闭环', type: 'success' },
}

export const severityText = { low: '一般', normal: '较重', high: '重大' }

export const roleText = { admin: '平台管理员', dept_leader: '区队长', employee: '员工' }

export const examStatus = { draft: '草稿', published: '已发布', closed: '已结束' }

export const broadcastStatus = { draft: '草稿', published: '已发布', closed: '已结束' }

export const taskStatus = {
  pending: { text: '排队中', type: 'info' },
  running: { text: '执行中', type: 'warning' },
  succeeded: { text: '完成', type: 'success' },
  failed: { text: '失败', type: 'danger' },
}

export const learnStatus = {
  not_started: { text: '未开始', type: 'info' },
  in_progress: { text: '学习中', type: 'warning' },
  completed: { text: '已完成', type: 'success' },
}
