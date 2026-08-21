import { useCallback, useMemo, useState } from 'react'
import { Copy } from '@phosphor-icons/react/dist/icons/Copy'
import { PlugsConnected } from '@phosphor-icons/react/dist/icons/PlugsConnected'
import { Pulse } from '@phosphor-icons/react/dist/icons/Pulse'
import { UsersThree } from '@phosphor-icons/react/dist/icons/UsersThree'
import { apiRequest } from '../api/client'
import type { OverviewResponse, ProbeResult, TimePoint } from '../api/types'
import { Button, EmptyState, InlineError, LoadingView, PageHeader, StatusBadge } from '../components/UI'
import { useAdmin } from '../context'
import { useLoad } from '../useLoad'
import { errorMessage, formatDateTime, formatLatency, formatNumber, relativeTime } from '../utils'

function chartPath(points: TimePoint[], key: 'requests' | 'failures', width = 760, height = 210): string {
  if (points.length === 0) return ''
  const max = Math.max(1, ...points.map((point) => point.requests))
  if (points.length === 1) {
    const y = height - (points[0][key] / max) * (height - 20) - 10
    const baseline = height - 10
    return `M0,${baseline} L330,${baseline} L365,${y.toFixed(1)} L400,${baseline} L${width},${baseline}`
  }
  const mapped = points.map((point, index) => ({
    x: (index / (points.length - 1)) * width,
    y: height - (point[key] / max) * (height - 20) - 10,
  }))
  return mapped.map((point, index) => `${index === 0 ? 'M' : 'L'}${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(' ')
}

function PulseChart({ points }: { points: TimePoint[] }) {
  const requests = chartPath(points, 'requests')
  const failures = chartPath(points, 'failures')
  if (!requests) {
    return (
      <div className="chart-empty">
        <span className="chart-empty__line" />
        <Pulse size={25} weight="light" />
        <p>暂无请求</p>
      </div>
    )
  }
  return (
    <div className="chart-wrap">
      <svg viewBox="0 0 760 240" role="img" aria-label="最近 24 小时请求和失败趋势">
        <defs>
          <linearGradient id="pulse-line" x1="0" x2="1">
            <stop stopColor="#8b6cff" />
            <stop offset="1" stopColor="#47d8ff" />
          </linearGradient>
          <linearGradient id="pulse-area" x1="0" y1="0" x2="0" y2="1">
            <stop stopColor="#745cff" stopOpacity=".24" />
            <stop offset="1" stopColor="#745cff" stopOpacity="0" />
          </linearGradient>
        </defs>
        <g className="chart-grid">
          <line x1="0" x2="760" y1="20" y2="20" /><line x1="0" x2="760" y1="82" y2="82" />
          <line x1="0" x2="760" y1="145" y2="145" /><line x1="0" x2="760" y1="208" y2="208" />
        </g>
        <path d={`${requests} L760,220 L0,220 Z`} fill="url(#pulse-area)" />
        <path className="chart-line" d={requests} stroke="url(#pulse-line)" />
        <path className="chart-line chart-line--error" d={failures} />
      </svg>
      <div className="chart-legend"><span><i />请求</span><span><i />失败</span></div>
    </div>
  )
}

export function OverviewPage() {
  const { toast, navigate } = useAdmin()
  const [probing, setProbing] = useState(false)
  const loader = useCallback(() => apiRequest<OverviewResponse>('/api/admin/overview?hours=24'), [])
  const { data, error, load } = useLoad(loader)

  async function probe() {
    setProbing(true)
    try {
      const result = await apiRequest<ProbeResult>('/api/admin/probe', {
        method: 'POST', body: JSON.stringify({ model: 'gemini-3.7-flash' }),
      })
      toast('success', `连通正常 · ${formatLatency(result.latency_ms)}`)
      void load()
    } catch (reason) {
      toast('error', `检测失败：${errorMessage(reason)}`)
    } finally {
      setProbing(false)
    }
  }

  const endpoint = useMemo(() => `${window.location.origin}/v1`, [])

  async function copyEndpoint() {
    try {
      await navigator.clipboard.writeText(endpoint)
      toast('success', '已复制')
    } catch {
      toast('error', '无法写入剪贴板')
    }
  }

  if (!data && !error) return <LoadingView />

  return (
    <div className="page page--overview">
      <PageHeader
        title="概览"
        action={<Button variant="primary" busy={probing} icon={<PlugsConnected size={18} weight="light" />} onClick={probe}>检测</Button>}
      />

      {error && <InlineError message={error} onRetry={load} />}

      {data && <>
        <section className="kpi-band" aria-label="最近 24 小时指标">
          <div><span>24 小时请求</span><strong>{formatNumber(data.stats.requests)}</strong></div>
          <div><span>成功率</span><strong>{data.stats.success_rate === null ? '—' : `${data.stats.success_rate.toFixed(1)}%`}</strong></div>
          <div><span>P50 延迟</span><strong>{formatLatency(data.stats.p50_latency_ms)}</strong></div>
          <div><span>输出 Tokens</span><strong>{formatNumber(data.stats.output_tokens)}</strong></div>
        </section>

        <div className="overview-grid">
          <section className="signal-panel signal-panel--chart">
            <header className="section-heading"><h2>请求</h2></header>
            <PulseChart points={data.timeseries} />
          </section>

          <section className="access-panel">
            <header className="section-heading"><h2>接入</h2></header>
            <div className="endpoint-line">
              <span>地址</span>
              <code title={endpoint}>{endpoint}</code>
              <button type="button" className="icon-button" aria-label="复制 API 地址" onClick={copyEndpoint}><Copy size={17} weight="light" /></button>
            </div>
            <div className="access-meta">
              <div><span>密钥</span><strong>{data.api_key.hint || '未生成'}</strong></div>
              <div><span>托管</span><strong>{data.api_key.external ? '环境变量' : '面板'}</strong></div>
            </div>
            <button className="text-link" type="button" onClick={() => navigate('settings')}>设置</button>
          </section>
        </div>

        <div className="overview-lower">
          <section className="plain-section account-health">
            <header className="section-heading">
              <h2>账号</h2>
              <button type="button" className="section-count" onClick={() => navigate('accounts')}>
                {data.stats.healthy_accounts} / {data.stats.accounts} 健康
              </button>
            </header>
            {data.accounts.length === 0 ? (
              <EmptyState icon={<UsersThree size={26} weight="light" />} title="还没有账号" />
            ) : (
              <div className="health-list">
                {data.accounts.slice(0, 4).map((account) => (
                  <div className="health-row" key={account.id}>
                    <div className="health-row__identity"><span>{account.label.slice(0, 1).toUpperCase()}</span><div><strong>{account.label}</strong><small>{account.cookie_summary}</small></div></div>
                    <div className="health-row__time"><span>最近成功</span><strong>{relativeTime(account.last_success_at)}</strong></div>
                    <StatusBadge status={account.status} enabled={account.enabled} />
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="plain-section recent-requests">
            <header className="section-heading">
              <h2>最近</h2>
              <button type="button" className="section-count" onClick={() => navigate('requests')}>全部</button>
            </header>
            {data.recent.length === 0 ? (
              <EmptyState title="暂无记录" />
            ) : (
              <div className="table-scroll"><table>
                <thead><tr><th>时间</th><th>模型</th><th>状态</th><th>耗时</th></tr></thead>
                <tbody>{data.recent.map((request) => (
                  <tr key={request.id}>
                    <td>{formatDateTime(request.created_at)}</td>
                    <td><code>{request.model}</code></td>
                    <td><span className={request.status_code >= 200 && request.status_code < 300 ? 'code-status is-ok' : 'code-status is-error'}>{request.status_code}</span></td>
                    <td>{formatLatency(request.latency_ms)}</td>
                  </tr>
                ))}</tbody>
              </table></div>
            )}
          </section>
        </div>
      </>}
    </div>
  )
}
