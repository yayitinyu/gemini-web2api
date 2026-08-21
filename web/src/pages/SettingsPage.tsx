import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Copy } from '@phosphor-icons/react/dist/icons/Copy'
import { FloppyDisk } from '@phosphor-icons/react/dist/icons/FloppyDisk'
import { Key } from '@phosphor-icons/react/dist/icons/Key'
import { apiRequest } from '../api/client'
import type { APIKeyRotation, APIKeyState, RuntimeSettings, SettingsResponse } from '../api/types'
import { Button, InlineError, LoadingView, Modal, PageHeader, Select } from '../components/UI'
import { useAdmin } from '../context'
import { errorMessage } from '../utils'

export function SettingsPage() {
  const { toast } = useAdmin()
  const [settings, setSettings] = useState<RuntimeSettings | null>(null)
  const [models, setModels] = useState<string[]>([])
  const [keyState, setKeyState] = useState<APIKeyState | null>(null)
  const [passwordSource, setPasswordSource] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [rotateOpen, setRotateOpen] = useState(false)
  const [confirm, setConfirm] = useState('')
  const [rotation, setRotation] = useState<APIKeyRotation | null>(null)

  const load = useCallback(async () => {
    try {
      setError('')
      const [settingsResult, keyResult] = await Promise.all([
        apiRequest<SettingsResponse>('/api/admin/settings'),
        apiRequest<APIKeyState>('/api/admin/api-key'),
      ])
      setSettings(settingsResult.settings); setModels(settingsResult.available_models)
      setPasswordSource(settingsResult.password_source); setKeyState(keyResult)
    } catch (reason) { setError(errorMessage(reason)) }
  }, [])
  useEffect(() => { void load() }, [load])

  function update<K extends keyof RuntimeSettings>(key: K, value: RuntimeSettings[K]) {
    setSettings((current) => current ? { ...current, [key]: value } : current)
  }

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!settings) return
    setBusy(true); setError('')
    try {
      const result = await apiRequest<{ settings: RuntimeSettings }>('/api/admin/settings', { method: 'PUT', body: JSON.stringify(settings) })
      setSettings(result.settings); toast('success', '已保存')
    } catch (reason) { setError(errorMessage(reason)) }
    finally { setBusy(false) }
  }

  async function rotate() {
    setBusy(true); setError('')
    try {
      const result = await apiRequest<APIKeyRotation>('/api/admin/api-key', { method: 'POST', body: JSON.stringify({ confirm }) })
      setRotation(result); setKeyState({ hint: result.hint, external: false }); setConfirm('')
      toast('success', '密钥已轮换')
    } catch (reason) { setError(errorMessage(reason)) }
    finally { setBusy(false) }
  }

  async function copyKey() {
    if (!rotation) return
    try { await navigator.clipboard.writeText(rotation.key); toast('success', '已复制') }
    catch { toast('error', '无法写入剪贴板') }
  }

  if (!settings && !error) return <LoadingView />

  return (
    <div className="page">
      <PageHeader title="设置" />
      {error && <InlineError message={error} onRetry={load} />}

      {settings && <form className="settings-layout" onSubmit={save}>
        <section className="settings-section">
          <header className="section-heading"><h2>生成</h2></header>
          <div className="form-grid">
            <div className="field"><span>默认模型</span><Select ariaLabel="默认模型" value={settings.default_model} onChange={(value) => update('default_model', value)} options={models.map((model) => ({ value: model, label: model, description: model.includes('pro') ? '更强推理' : '低延迟' }))} /></div>
            <label className="field"><span>超时 <small>10–600 秒</small></span><input type="number" min={10} max={600} value={settings.request_timeout_sec} onChange={(e) => update('request_timeout_sec', Number(e.target.value))} /></label>
            <label className="field"><span>尝试次数 <small>1–4</small></span><input type="number" min={1} max={4} value={settings.retry_attempts} onChange={(e) => update('retry_attempts', Number(e.target.value))} /></label>
            <label className="field"><span>重试间隔 <small>毫秒</small></span><input type="number" min={0} max={10000} step={50} value={settings.retry_delay_ms} onChange={(e) => update('retry_delay_ms', Number(e.target.value))} /></label>
            <label className="field"><span>提示词上限 <small>字节，0 不限</small></span><input type="number" min={0} max={1000000} step={1024} value={settings.max_prompt_bytes} onChange={(e) => update('max_prompt_bytes', Number(e.target.value))} /></label>
            <label className="field"><span>审计保留 <small>1–365 天</small></span><input type="number" min={1} max={365} value={settings.retention_days} onChange={(e) => update('retention_days', Number(e.target.value))} /></label>
          </div>
        </section>

        <section className="settings-section">
          <header className="section-heading"><h2>回退</h2></header>
          <div className="switch-stack">
            <label className="switch-field"><span><strong>匿名访问</strong><small>全部账号失败后尝试无 Cookie 请求</small></span><input type="checkbox" checked={settings.fallback_anonymous} onChange={(e) => update('fallback_anonymous', e.target.checked)} /><i /></label>
            <label className="switch-field"><span><strong>直连</strong><small>代理不可用时绕过代理</small></span><input type="checkbox" checked={settings.fallback_direct} onChange={(e) => update('fallback_direct', e.target.checked)} /><i /></label>
            <label className="switch-field"><span><strong>自动发现 BL</strong><small>从 Gemini 页面刷新后端令牌</small></span><input type="checkbox" checked={settings.gemini_bl_auto} onChange={(e) => update('gemini_bl_auto', e.target.checked)} /><i /></label>
          </div>
          <label className="field field--wide"><span>Gemini BL</span><input value={settings.gemini_bl} onChange={(e) => update('gemini_bl', e.target.value)} disabled={settings.gemini_bl_auto} spellCheck={false} /></label>
        </section>

        <div className="settings-save"><Button type="submit" variant="primary" busy={busy} icon={<FloppyDisk size={18} weight="light" />}>保存</Button></div>
      </form>}

      <section className="security-section">
        <header className="section-heading"><h2>凭据</h2></header>
        <div className="security-row"><div><span>管理密码</span><strong>{passwordSource || 'ADMIN_PASSWORD'}</strong><small>修改环境变量后重启生效。</small></div></div>
        <div className="security-row"><div><span>API 密钥</span><strong><code>{keyState?.hint || '未生成'}</code></strong><small>{keyState?.external ? '由 API_KEY 托管，无法在此轮换。' : '只保存摘要，明文不会再次显示。'}</small></div><Button variant="secondary" icon={<Key size={18} weight="light" />} disabled={keyState?.external} onClick={() => { setRotation(null); setConfirm(''); setRotateOpen(true) }}>轮换</Button></div>
      </section>

      <Modal open={rotateOpen} onClose={() => setRotateOpen(false)} size="small" title={rotation ? '保存新密钥' : '轮换密钥'} description={rotation ? '关闭后无法再次查看完整密钥。' : '旧密钥立即失效。'}
        footer={rotation ? <Button variant="primary" onClick={() => setRotateOpen(false)}>已保存</Button> : <><Button variant="quiet" onClick={() => setRotateOpen(false)}>取消</Button><Button variant="danger" busy={busy} disabled={confirm !== 'ROTATE'} onClick={rotate}>确认</Button></>}>
        {rotation ? <div className="secret-reveal"><div><code>{rotation.key}</code><button className="icon-button" type="button" aria-label="复制新 API 密钥" onClick={copyKey}><Copy size={17} weight="light" /></button></div>{rotation.notice && <p>{rotation.notice}</p>}</div> : <label className="field field--wide"><span>输入 ROTATE 确认</span><input value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="off" placeholder="ROTATE" /></label>}
      </Modal>
    </div>
  )
}
