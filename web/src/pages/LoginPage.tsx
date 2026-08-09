import { useState, type FormEvent } from 'react'
import { ArrowRight } from '@phosphor-icons/react/dist/icons/ArrowRight'
import { Eye } from '@phosphor-icons/react/dist/icons/Eye'
import { EyeSlash } from '@phosphor-icons/react/dist/icons/EyeSlash'
import { LockKey } from '@phosphor-icons/react/dist/icons/LockKey'
import { ShieldCheck } from '@phosphor-icons/react/dist/icons/ShieldCheck'
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
      <div className="login-page__aurora" aria-hidden="true" />
      <div className="login-page__orbit login-page__orbit--one" aria-hidden="true" />
      <div className="login-page__orbit login-page__orbit--two" aria-hidden="true" />

      <header className="login-page__brand"><Brand /></header>

      <section className="login-intro">
        <p className="eyebrow">SELF-HOSTED GATEWAY</p>
        <h1>把网页能力，<br /><em>折叠成接口。</em></h1>
        <p>一个克制、安全的 Gemini Web 兼容层。凭据留在你的服务器，调用方式回到熟悉的 OpenAI 协议。</p>
        <div className="login-intro__proof">
          <span><ShieldCheck size={18} weight="light" />本地加密存储</span>
          <span><LockKey size={18} weight="light" />受保护的管理域</span>
        </div>
      </section>

      <section className="login-panel" aria-labelledby="login-title">
        <div className="login-panel__bezel" />
        <div className="login-panel__content">
          <div className="login-panel__index">01 / ACCESS</div>
          <h2 id="login-title">进入控制台</h2>
          <p>使用部署时设置的管理密码继续。</p>

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
                placeholder="输入管理密码"
              />
              <button type="button" aria-label={visible ? '隐藏密码' : '显示密码'} onClick={() => setVisible((value) => !value)}>
                {visible ? <EyeSlash size={19} weight="light" /> : <Eye size={19} weight="light" />}
              </button>
            </div>
            {error && <p className="form-error" id="login-error" role="alert">{error}</p>}
            <Button type="submit" variant="primary" busy={busy} disabled={!password} icon={<ArrowRight size={19} weight="light" />}>
              验证并进入
            </Button>
          </form>
          <p className="login-panel__note">会话 Cookie 仅限同源访问，且不会暴露给页面脚本。</p>
        </div>
      </section>

      <footer className="login-page__footer">PRIVATE INFRASTRUCTURE / OPENAI-COMPATIBLE SURFACE</footer>
    </main>
  )
}
