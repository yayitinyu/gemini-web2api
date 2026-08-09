import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiRequest, clearSession, login } from './client'

describe('admin API client', () => {
  beforeEach(() => {
    clearSession()
    vi.restoreAllMocks()
  })

  it('attaches the CSRF token only after login for mutating requests', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        authenticated: true, csrf_token: 'csrf-123', expires_at: 2_000_000_000,
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), {
        status: 200, headers: { 'Content-Type': 'application/json' },
      }))
    vi.stubGlobal('fetch', fetchMock)

    await login('a-strong-admin-password')
    await apiRequest('/api/admin/probe', { method: 'POST', body: '{}' })

    const secondCall = fetchMock.mock.calls[1]
    const headers = secondCall[1].headers as Headers
    expect(headers.get('X-CSRF-Token')).toBe('csrf-123')
    expect(headers.get('Content-Type')).toBe('application/json')
  })
})
