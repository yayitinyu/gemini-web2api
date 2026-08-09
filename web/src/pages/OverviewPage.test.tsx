import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { OverviewPage } from './OverviewPage'

describe('OverviewPage', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('renders honest empty-state metrics for a fresh installation', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      stats: { requests: 0, success_rate: null, p50_latency_ms: null, output_tokens: 0, accounts: 0, healthy_accounts: 0, proxies: 0 },
      timeseries: [], recent: [], accounts: [], api_key: { hint: 'gw_…abcd', external: false }, range_hours: 24,
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })))

    render(<OverviewPage toast={vi.fn()} />)
    expect(await screen.findByText('暂无请求脉冲')).toBeTruthy()
    expect(screen.getByText('还没有 Gemini 账号')).toBeTruthy()
    expect(screen.getByText('审计轨迹仍是空的')).toBeTruthy()
    expect(screen.getByText('面板托管')).toBeTruthy()
  })
})
