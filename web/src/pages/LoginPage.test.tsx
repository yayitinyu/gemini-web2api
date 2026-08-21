import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { clearSession } from '../api/client'
import { LoginPage } from './LoginPage'

describe('LoginPage', () => {
  beforeEach(() => {
    clearSession()
    vi.restoreAllMocks()
  })

  it('reveals the password only when requested and authenticates', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      authenticated: true,
      csrf_token: 'csrf-test',
      expires_at: 2_000_000_000,
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)
    const authenticated = vi.fn()
    const user = userEvent.setup()
    render(<LoginPage onAuthenticated={authenticated} />)

    expect(screen.queryByText(/SELF-HOSTED/)).toBeNull()
    expect(screen.queryByText(/折叠成接口/)).toBeNull()

    const password = screen.getByLabelText<HTMLInputElement>('管理密码')
    expect(password.type).toBe('password')
    await user.click(screen.getByRole('button', { name: '显示密码' }))
    expect(password.type).toBe('text')
    await user.type(password, 'a-strong-admin-password')
    await user.click(screen.getByRole('button', { name: '进入' }))

    await waitFor(() => expect(authenticated).toHaveBeenCalledOnce())
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/auth/login', expect.objectContaining({ method: 'POST' }))
  })

  it('surfaces an authentication error without leaving the form', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: { code: 'invalid_password', message: '管理密码不正确' },
    }), { status: 401, headers: { 'Content-Type': 'application/json' } })))
    const user = userEvent.setup()
    render(<LoginPage onAuthenticated={vi.fn()} />)

    await user.type(screen.getByLabelText('管理密码'), 'wrong-password')
    await user.click(screen.getByRole('button', { name: '进入' }))

    expect((await screen.findByRole('alert')).textContent).toContain('管理密码不正确')
  })
})
