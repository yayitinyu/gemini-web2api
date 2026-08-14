import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { CaretLeft } from '@phosphor-icons/react/dist/icons/CaretLeft'
import { CaretRight } from '@phosphor-icons/react/dist/icons/CaretRight'
import { FunnelSimple } from '@phosphor-icons/react/dist/icons/FunnelSimple'
import { Pulse } from '@phosphor-icons/react/dist/icons/Pulse'
import { apiRequest } from '../api/client'
import type { RequestPage } from '../api/types'
import { Button, EmptyState, LoadingView, PageHeader, Select, type SelectOption } from '../components/UI'
import { errorMessage, formatDateTime, formatLatency, formatNumber } from '../utils'

const limit = 25
const modelOptions = ['gemini-3.7-flash', 'gemini-3.6-flash', 'gemini-3.5-flash-lite', 'gemini-3.1-pro']
const modelSelectOptions: SelectOption[] = [{ value: '', label: '全部模型' }, ...modelOptions.map((model) => ({ value: model, label: model }))]
const statusSelectOptions: SelectOption[] = [
  { value: '', label: '全部状态' },
  { value: 'success', label: '仅成功' },
  { value: 'error', label: '仅错误' },
]

export function RequestsPage() {
  const [data, setData] = useState<RequestPage | null>(null)
  const [offset, setOffset] = useState(0)
  const [filters, setFilters] = useState({ model: '', status: '' })
  const [draft, setDraft] = useState(filters)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    const query = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (filters.model) query.set('model', filters.model)
    if (filters.status) query.set('status', filters.status)
    try {
      setError('')
      setData(await apiRequest<RequestPage>(`/api/admin/requests?${query}`))
    } catch (reason) { setError(errorMessage(reason)) }
  }, [filters, offset])

  useEffect(() => { void load() }, [load])

  function applyFilters(event: FormEvent) {
    event.preventDefault()
    setOffset(0)
    setFilters(draft)
  }

  if (!data && !error) return <LoadingView label="正在读取请求审计轨迹" />
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / limit))

  return (
    <div className="page">
      <PageHeader eyebrow="METADATA AUDIT" title="请求轨迹" description="排查状态、模型、账号与耗时；提示词和生成正文永远不会写入审计表。" />
      <form className="filter-bar" onSubmit={applyFilters}>
        <div className="filter-field"><span>模型</span><Select compact ariaLabel="模型" value={draft.model} onChange={(value) => setDraft({ ...draft, model: value })} options={modelSelectOptions} /></div>
        <div className="filter-field"><span>结果</span><Select compact ariaLabel="结果" value={draft.status} onChange={(value) => setDraft({ ...draft, status: value })} options={statusSelectOptions} /></div>
        <Button type="submit" variant="secondary" icon={<FunnelSimple size={17} weight="light" />}>应用筛选</Button>
        {(filters.model || filters.status) && <button type="button" className="text-link" onClick={() => { setDraft({ model: '', status: '' }); setFilters({ model: '', status: '' }); setOffset(0) }}>清除筛选</button>}
      </form>
      {error && <div className="inline-error"><span>{error}</span><Button variant="quiet" onClick={load}>重新读取</Button></div>}

      <section className="audit-panel">
        <header className="section-heading"><div><span>REQUEST INDEX</span><h2>审计元数据</h2></div><span className="section-count">{formatNumber(data?.total ?? 0)} 条记录</span></header>
        {data?.items.length === 0 ? (
          <EmptyState icon={<Pulse size={28} weight="light" />} title="当前筛选没有轨迹" description="尝试清除筛选，或从兼容端点发起一次请求。" />
        ) : <div className="table-scroll audit-table"><table>
          <thead><tr><th>时间 / 请求</th><th>端点 / 模型</th><th>路径</th><th>状态</th><th>TTFB</th><th>总耗时</th><th>Tokens</th></tr></thead>
          <tbody>{data?.items.map((item) => (
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
          <span>第 {page} / {pages} 页</span>
          <div><button className="icon-button" type="button" aria-label="上一页" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}><CaretLeft size={18} /></button><button className="icon-button" type="button" aria-label="下一页" disabled={!data || offset + limit >= data.total} onClick={() => setOffset(offset + limit)}><CaretRight size={18} /></button></div>
        </footer>
      </section>
    </div>
  )
}
