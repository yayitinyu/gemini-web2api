import { useState, type FormEvent } from 'react'
import { Eye } from '@phosphor-icons/react/dist/icons/Eye'
import { EyeSlash } from '@phosphor-icons/react/dist/icons/EyeSlash'
import { login } from '../api/client'
import { Brand } from '../components/Brand'
import { Button } from '../components/UI'
import { errorMessage } from '../utils'

export function LoginPage({ onAuthenticated }: { onAuthenticated: () => void }) {
  const [password, setPassword] = useState('')
  const [visible, setVisible] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!password || busy) return
    setBusy(true)
    setError('')
    try {
      await login(password)
      onAuthenticated()
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel" aria-labelledby="login-title">
        <Brand />
        <h1 id="login-title">登录</h1>
        <form onSubmit={submit}>
          <label htmlFor="admin-password">管理密码</label>
          <div className="password-field">
            <input
              id="admin-password"
              type={visible ? 'text' : 'password'}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              autoFocus
              required
              aria-invalid={Boolean(error)}
              aria-describedby={error ? 'login-error' : undefined}
            />
            <button type="button" aria-label={visible ? '隐藏密码' : '显示密码'} onClick={() => setVisible((value) => !value)}>
              {visible ? <EyeSlash size={19} weight="light" /> : <Eye size={19} weight="light" />}
            </button>
          </div>
          {error && <p className="form-error" id="login-error" role="alert">{error}</p>}
          <Button type="submit" variant="primary" busy={busy} disabled={!password}>
            进入
          </Button>
        </form>
      </section>
    </main>
  )
}
