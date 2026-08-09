import { useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowRight } from '@phosphor-icons/react/dist/icons/ArrowRight'
import { CheckCircle } from '@phosphor-icons/react/dist/icons/CheckCircle'
import { Copy } from '@phosphor-icons/react/dist/icons/Copy'
import { PlugsConnected } from '@phosphor-icons/react/dist/icons/PlugsConnected'
import { Pulse } from '@phosphor-icons/react/dist/icons/Pulse'
import { UsersThree } from '@phosphor-icons/react/dist/icons/UsersThree'
import { apiRequest } from '../api/client'
import type { OverviewResponse, ProbeResult, TimePoint } from '../api/types'
import { Button, EmptyState, LoadingView, PageHeader, StatusBadge, type ToastKind } from '../components/UI'
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
        <p>暂无请求脉冲</p>
        <span>首个 API 请求会在这里留下轨迹</span>
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

export function OverviewPage({ toast }: { toast: (kind: ToastKind, text: string) => void }) {
  const [data, setData] = useState<OverviewResponse | null>(null)
  const [error, setError] = useState('')
  const [probing, setProbing] = useState(false)

  const load = useCallback(async () => {
    try {
      setError('')
      setData(await apiRequest<OverviewResponse>('/api/admin/overview?hours=24'))
    } catch (reason) {
      setError(errorMessage(reason))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function probe() {
    setProbing(true)
    try {
      const result = await apiRequest<ProbeResult>('/api/admin/probe', {
        method: 'POST', body: JSON.stringify({ model: 'gemini-3.6-flash' }),
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
      toast('success', 'API 地址已复制')
    } catch {
      toast('error', '浏览器未允许写入剪贴板')
    }
  }

  if (!data && !error) return <LoadingView />

  return (
    <div className="page page--overview">
      <PageHeader
        eyebrow="LIVE CONTROL SURFACE"
        title="运行概览"
        description="观察流量、凭据健康与网关路径，不记录提示词正文。"
        action={<Button variant="primary" busy={probing} icon={<PlugsConnected size={18} weight="light" />} onClick={probe}>连通性检测</Button>}
      />

      {error && (
        <div className="inline-error" role="alert">
          <span>{error}</span><Button variant="quiet" onClick={load}>重新读取</Button>
        </div>
      )}

      {data && <>
        <section className="kpi-band" aria-label="最近 24 小时指标">
          <div><span>24 小时请求</span><strong>{formatNumber(data.stats.requests)}</strong><small>REQUESTS</small></div>
          <div><span>成功率</span><strong>{data.stats.success_rate === null ? '—' : `${data.stats.success_rate.toFixed(1)}%`}</strong><small>SUCCESS</small></div>
          <div><span>P50 延迟</span><strong>{formatLatency(data.stats.p50_latency_ms)}</strong><small>LATENCY</small></div>
          <div><span>输出 Tokens</span><strong>{formatNumber(data.stats.output_tokens)}</strong><small>ESTIMATED</small></div>
        </section>

        <div className="overview-grid">
          <section className="signal-panel signal-panel--chart">
            <header className="section-heading">
              <div><span>24H SIGNAL</span><h2>请求脉冲</h2></div>
              <div className="live-indicator"><span />实时元数据</div>
            </header>
            <PulseChart points={data.timeseries} />
          </section>

          <section className="access-panel">
            <div className="access-panel__signal" aria-hidden="true" />
            <header className="section-heading"><div><span>OPENAI SURFACE</span><h2>接入信息</h2></div></header>
            <div className="endpoint-line">
              <span>BASE URL</span>
              <code title={endpoint}>{endpoint}</code>
              <button type="button" className="icon-button" aria-label="复制 API 地址" onClick={copyEndpoint}><Copy size={17} weight="light" /></button>
            </div>
            <div className="access-meta">
              <div><span>API KEY</span><strong>{data.api_key.hint || '未生成'}</strong></div>
              <div><span>MODE</span><strong>{data.api_key.external ? '环境变量托管' : '面板托管'}</strong></div>
            </div>
            <p>支持 Chat Completions 与 Responses 的文本和函数工具调用。</p>
            <button className="text-link" type="button" onClick={() => window.dispatchEvent(new CustomEvent('navigate-admin', { detail: 'settings' }))}>
              查看接入设置 <ArrowRight size={16} weight="light" />
            </button>
          </section>
        </div>

        <div className="overview-lower">
          <section className="plain-section account-health">
            <header className="section-heading">
              <div><span>CREDENTIAL POOL</span><h2>账号健康</h2></div>
              <span className="section-count">{data.stats.healthy_accounts} / {data.stats.accounts} 健康</span>
            </header>
            {data.accounts.length === 0 ? (
              <EmptyState icon={<UsersThree size={26} weight="light" />} title="还没有 Gemini 账号" description="添加 Google Cookie 后，网关才会接受上游生成请求。" />
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
            <header className="section-heading"><div><span>AUDIT TRACE</span><h2>最近请求</h2></div></header>
            {data.recent.length === 0 ? (
              <EmptyState icon={<CheckCircle size={26} weight="light" />} title="审计轨迹仍是空的" description="这里只保存模型、耗时和状态等元数据。" />
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
