export function classNames(...values: Array<string | false | null | undefined>) {
  return values.filter(Boolean).join(' ')
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN', { notation: value >= 100_000 ? 'compact' : 'standard' }).format(value)
}

export function formatLatency(value: number | null): string {
  if (value === null) return '—'
  if (value >= 1000) return `${(value / 1000).toFixed(value >= 10_000 ? 0 : 1)} s`
  return `${value} ms`
}

export function formatDateTime(timestamp: number): string {
  if (!timestamp) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
    hour12: false,
  }).format(new Date(timestamp * 1000))
}

export function relativeTime(timestamp: number): string {
  if (!timestamp) return '从未'
  const delta = Math.max(0, Math.floor(Date.now() / 1000) - timestamp)
  if (delta < 60) return '刚刚'
  if (delta < 3600) return `${Math.floor(delta / 60)} 分钟前`
  if (delta < 86_400) return `${Math.floor(delta / 3600)} 小时前`
  return `${Math.floor(delta / 86_400)} 天前`
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '发生未知错误'
}
