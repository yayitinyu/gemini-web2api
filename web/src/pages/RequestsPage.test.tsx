import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RequestsPage } from './RequestsPage'

const emptyPage = { items: [], total: 0, limit: 25, offset: 0 }
const settings = {
  settings: {
    default_model: 'gemini-3.7-flash', request_timeout_sec: 60, retry_attempts: 1,
    retry_delay_ms: 0, max_prompt_bytes: 0, fallback_anonymous: false, fallback_direct: false,
    gemini_bl: '', gemini_bl_auto: true, retention_days: 30,
  },
  available_models: ['gemini-3.7-flash', 'gemini-3.1-pro'],
  password_source: 'ADMIN_PASSWORD',
}

function json(data: unknown) {
  return new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

describe('RequestsPage', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('applies model filters immediately without a submit button', async () => {
    const fetchMock = vi.fn().mockImplementation((input: string) => {
      if (String(input).includes('/api/admin/settings')) return Promise.resolve(json(settings))
      return Promise.resolve(json(emptyPage))
    })
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    render(<RequestsPage />)

    expect(await screen.findByText('没有记录')).toBeTruthy()
    expect(screen.queryByRole('button', { name: '应用筛选' })).toBeNull()

    await user.click(screen.getByRole('combobox', { name: '模型' }))
    await user.click(screen.getByRole('option', { name: 'gemini-3.1-pro' }))

    await waitFor(() => {
      expect(fetchMock.mock.calls.some(([url]) => String(url).includes('model=gemini-3.1-pro'))).toBe(true)
    })
  })
})
