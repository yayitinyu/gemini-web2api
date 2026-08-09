import type { SessionResponse } from './types'

interface ErrorPayload {
  error?: {
    code?: string
    message?: string
  } | string
  message?: string
}

export class APIError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'APIError'
    this.status = status
    this.code = code
  }
}

let csrfToken = ''
let unauthorizedHandler: (() => void) | undefined

export function setUnauthorizedHandler(handler: (() => void) | undefined) {
  unauthorizedHandler = handler
}

export function clearSession() {
  csrfToken = ''
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
  options: { ignoreUnauthorized?: boolean } = {},
): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  if (init.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method) && csrfToken) {
    headers.set('X-CSRF-Token', csrfToken)
  }

  const response = await fetch(path, {
    ...init,
    method,
    headers,
    credentials: 'same-origin',
  })

  if (response.status === 401 && !options.ignoreUnauthorized) {
    clearSession()
    unauthorizedHandler?.()
  }
  if (!response.ok) {
    let payload: ErrorPayload = {}
    try {
      payload = await response.json() as ErrorPayload
    } catch {
      // The status text is still useful when an intermediary returns non-JSON.
    }
    const nested = typeof payload.error === 'object' ? payload.error : undefined
    const message = nested?.message ?? payload.message ?? (typeof payload.error === 'string' ? payload.error : '')
    throw new APIError(response.status, nested?.code ?? 'request_failed', message || response.statusText || '请求失败')
  }

  if (response.status === 204) {
    return undefined as T
  }
  return await response.json() as T
}

export async function checkSession(): Promise<SessionResponse> {
  const session = await apiRequest<SessionResponse>('/api/admin/auth/session', {}, { ignoreUnauthorized: true })
  csrfToken = session.csrf_token
  return session
}

export async function login(password: string): Promise<SessionResponse> {
  const session = await apiRequest<SessionResponse>('/api/admin/auth/login', {
    method: 'POST',
    body: JSON.stringify({ password }),
  }, { ignoreUnauthorized: true })
  csrfToken = session.csrf_token
  return session
}

export async function logout(): Promise<void> {
  try {
    await apiRequest<void>('/api/admin/auth/logout', { method: 'POST' })
  } finally {
    clearSession()
  }
}
