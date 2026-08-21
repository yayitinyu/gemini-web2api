import { useCallback, useState } from 'react'
import { CaretLeft } from '@phosphor-icons/react/dist/icons/CaretLeft'
import { CaretRight } from '@phosphor-icons/react/dist/icons/CaretRight'
import { Pulse } from '@phosphor-icons/react/dist/icons/Pulse'
import { apiRequest } from '../api/client'
import type { RequestPage, SettingsResponse } from '../api/types'
import { EmptyState, InlineError, LoadingView, PageHeader, Select, type SelectOption } from '../components/UI'
import { useLoad } from '../useLoad'
import { formatDateTime, formatLatency, formatNumber } from '../utils'

const limit = 25
const statusSelectOptions: SelectOption[] = [
  { value: '', label: '全部状态' },
  { value: 'success', label: '成功' },
  { value: 'error', label: '错误' },
]

export function RequestsPage() {
  const [offset, setOffset] = useState(0)
  const [filters, setFilters] = useState({ model: '', status: '' })

  const modelsLoader = useCallback(async () => {
    const result = await apiRequest<SettingsResponse>('/api/admin/settings')
    return result.available_models
  }, [])
  const { data: models } = useLoad(modelsLoader)

  const listLoader = useCallback(async () => {
    const query = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (filters.model) query.set('model', filters.model)
    if (filters.status) query.set('status', filters.status)
    return apiRequest<RequestPage>(`/api/admin/requests?${query}`)
  }, [filters, offset])
  const { data, error, load } = useLoad(listLoader)

  const modelSelectOptions: SelectOption[] = [
    { value: '', label: '全部模型' },
    ...(models ?? []).map((model) => ({ value: model, label: model })),
  ]

  function updateFilter(key: 'model' | 'status', value: string) {
    setOffset(0)
    setFilters((current) => ({ ...current, [key]: value }))
  }

  if (!data && !error) return <LoadingView />
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / limit))
  const filtered = Boolean(filters.model || filters.status)

  return (
    <div className="page">
      <PageHeader title="请求" />
      <form className="filter-bar" onSubmit={(event) => event.preventDefault()}>
        <div className="filter-field"><span>模型</span><Select compact ariaLabel="模型" value={filters.model} onChange={(value) => updateFilter('model', value)} options={modelSelectOptions} /></div>
        <div className="filter-field"><span>结果</span><Select compact ariaLabel="结果" value={filters.status} onChange={(value) => updateFilter('status', value)} options={statusSelectOptions} /></div>
        {filtered && <button type="button" className="text-link" onClick={() => { setFilters({ model: '', status: '' }); setOffset(0) }}>清除</button>}
      </form>
      {error && <InlineError message={error} onRetry={load} />}

      <section className="audit-panel">
        <header className="section-heading">
          <h2>记录</h2>
          <span className="section-count">{formatNumber(data?.total ?? 0)}</span>
        </header>
        {!data ? null : data.items.length === 0 ? (
          <EmptyState icon={<Pulse size={28} weight="light" />} title="没有记录" />
        ) : <div className="table-scroll audit-table"><table>
          <thead><tr><th>时间 / 请求</th><th>端点 / 模型</th><th>路径</th><th>状态</th><th>TTFB</th><th>总耗时</th><th>Tokens</th></tr></thead>
          <tbody>{data.items.map((item) => (
            <tr key={item.id} title={item.error_message || undefined}>
              <td><strong>{formatDateTime(item.created_at)}</strong><code>{item.request_id.slice(0, 18)}</code></td>
              <td><strong>{item.endpoint}</strong><code>{item.model}</code></td>
              <td><span>{item.account_label || '匿名'}</span><small>{item.proxy_label || '直连'}</small></td>
              <td><span className={item.status_code >= 200 && item.status_code < 300 ? 'code-status is-ok' : 'code-status is-error'}>{item.status_code}</span>{item.error_code && <small>{item.error_code}</small>}</td>
              <td>{formatLatency(item.ttfb_ms)}</td><td>{formatLatency(item.latency_ms)}</td>
              <td><strong>{formatNumber(item.output_tokens)}</strong><small>in {formatNumber(item.input_tokens)}</small></td>
            </tr>
          ))}</tbody>
        </table></div>}
        <footer className="pagination">
          <span>{page} / {pages}</span>
          <div><button className="icon-button" type="button" aria-label="上一页" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}><CaretLeft size={18} /></button><button className="icon-button" type="button" aria-label="下一页" disabled={!data || offset + limit >= data.total} onClick={() => setOffset(offset + limit)}><CaretRight size={18} /></button></div>
        </footer>
      </section>
    </div>
  )
}
