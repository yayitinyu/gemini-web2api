import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Copy } from '@phosphor-icons/react/dist/icons/Copy'
import { FloppyDisk } from '@phosphor-icons/react/dist/icons/FloppyDisk'
import { Key } from '@phosphor-icons/react/dist/icons/Key'
import { ShieldCheck } from '@phosphor-icons/react/dist/icons/ShieldCheck'
import { WarningCircle } from '@phosphor-icons/react/dist/icons/WarningCircle'
import { apiRequest } from '../api/client'
import type { APIKeyRotation, APIKeyState, RuntimeSettings, SettingsResponse } from '../api/types'
import { Button, LoadingView, Modal, PageHeader, Select, type ToastKind } from '../components/UI'
import { errorMessage } from '../utils'

export function SettingsPage({ toast }: { toast: (kind: ToastKind, text: string) => void }) {
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
      setSettings(result.settings); toast('success', '运行设置已保存，新请求将立即使用')
    } catch (reason) { setError(errorMessage(reason)) }
    finally { setBusy(false) }
  }

  async function rotate() {
    setBusy(true); setError('')
    try {
      const result = await apiRequest<APIKeyRotation>('/api/admin/api-key', { method: 'POST', body: JSON.stringify({ confirm }) })
      setRotation(result); setKeyState({ hint: result.hint, external: false }); setConfirm('')
      toast('success', 'API 密钥已轮换，旧密钥立即失效')
    } catch (reason) { setError(errorMessage(reason)) }
    finally { setBusy(false) }
  }

  async function copyKey() {
    if (!rotation) return
    try { await navigator.clipboard.writeText(rotation.key); toast('success', '新密钥已复制') }
    catch { toast('error', '浏览器未允许写入剪贴板') }
  }

  if (!settings && !error) return <LoadingView label="正在读取运行设置" />

  return (
    <div className="page">
      <PageHeader eyebrow="RUNTIME POLICY" title="网关设置" description="调整失败策略、超时和审计保留期；保存后无需重启容器。" />
      {error && <div className="inline-error" role="alert"><span>{error}</span><Button variant="quiet" onClick={load}>重新读取</Button></div>}

      {settings && <form className="settings-layout" onSubmit={save}>
        <section className="settings-section">
          <header className="section-heading"><div><span>GENERATION</span><h2>生成策略</h2></div></header>
          <div className="form-grid">
            <div className="field"><span>默认模型</span><Select ariaLabel="默认模型" value={settings.default_model} onChange={(value) => update('default_model', value)} options={models.map((model) => ({ value: model, label: model, description: model.includes('pro') ? '更强推理与复杂任务' : '低延迟通用生成' }))} /></div>
            <label className="field"><span>请求超时 <small>10–600 秒</small></span><input type="number" min={10} max={600} value={settings.request_timeout_sec} onChange={(e) => update('request_timeout_sec', Number(e.target.value))} /></label>
            <label className="field"><span>尝试次数 <small>1–4 次</small></span><input type="number" min={1} max={4} value={settings.retry_attempts} onChange={(e) => update('retry_attempts', Number(e.target.value))} /></label>
            <label className="field"><span>重试间隔 <small>毫秒</small></span><input type="number" min={0} max={10000} step={50} value={settings.retry_delay_ms} onChange={(e) => update('retry_delay_ms', Number(e.target.value))} /></label>
            <label className="field"><span>提示词上限 <small>字节，0 为不限</small></span><input type="number" min={0} max={1000000} step={1024} value={settings.max_prompt_bytes} onChange={(e) => update('max_prompt_bytes', Number(e.target.value))} /></label>
            <label className="field"><span>审计保留 <small>1–365 天</small></span><input type="number" min={1} max={365} value={settings.retention_days} onChange={(e) => update('retention_days', Number(e.target.value))} /></label>
          </div>
        </section>

        <section className="settings-section">
          <header className="section-heading"><div><span>FAILURE BOUNDARY</span><h2>回退边界</h2></div></header>
          <div className="switch-stack">
            <label className="switch-field"><span><strong>匿名访问回退</strong><small>所有账号失败后，尝试无 Cookie 请求</small></span><input type="checkbox" checked={settings.fallback_anonymous} onChange={(e) => update('fallback_anonymous', e.target.checked)} /><i /></label>
            <label className="switch-field"><span><strong>直接连接回退</strong><small>代理池不可用时允许绕过代理</small></span><input type="checkbox" checked={settings.fallback_direct} onChange={(e) => update('fallback_direct', e.target.checked)} /><i /></label>
            <label className="switch-field"><span><strong>自动发现 Gemini BL</strong><small>从 Gemini 页面动态刷新后端版本令牌</small></span><input type="checkbox" checked={settings.gemini_bl_auto} onChange={(e) => update('gemini_bl_auto', e.target.checked)} /><i /></label>
          </div>
          <label className="field field--wide"><span>Gemini BL 令牌</span><input value={settings.gemini_bl} onChange={(e) => update('gemini_bl', e.target.value)} disabled={settings.gemini_bl_auto} spellCheck={false} /></label>
          <div className="settings-note"><WarningCircle size={19} weight="light" /><p>回退默认关闭，避免请求意外改变出口或身份边界。重试只发生在尚未向客户端发送内容之前。</p></div>
        </section>

        <div className="settings-save"><span>设置写入持久化数据库，不需要重建镜像。</span><Button type="submit" variant="primary" busy={busy} icon={<FloppyDisk size={18} weight="light" />}>保存运行设置</Button></div>
      </form>}

      <section className="security-section">
        <header className="section-heading"><div><span>ACCESS CONTROL</span><h2>访问凭据</h2></div><ShieldCheck size={24} weight="light" /></header>
        <div className="security-row"><div><span>管理面板密码</span><strong>由 {passwordSource || 'ADMIN_PASSWORD'} 提供</strong><small>仅保存 bcrypt 校验值；修改环境变量后重启生效。</small></div><span className="secure-chip">ENV MANAGED</span></div>
        <div className="security-row"><div><span>OpenAI 兼容 API 密钥</span><strong><code>{keyState?.hint || '未生成'}</code></strong><small>{keyState?.external ? '由 API_KEY 环境变量托管，面板无法轮换。' : '数据库只保存 SHA-256 摘要，明文不会再次显示。'}</small></div><Button variant="secondary" icon={<Key size={18} weight="light" />} disabled={keyState?.external} onClick={() => { setRotation(null); setConfirm(''); setRotateOpen(true) }}>轮换密钥</Button></div>
      </section>

      <Modal open={rotateOpen} onClose={() => setRotateOpen(false)} size="small" title={rotation ? '保存新的 API 密钥' : '轮换 API 密钥'} description={rotation ? '关闭窗口后将无法再次查看完整密钥。' : '旧密钥会立即失效，现有客户端需要同步更新。'}
        footer={rotation ? <Button variant="primary" onClick={() => setRotateOpen(false)}>我已安全保存</Button> : <><Button variant="quiet" onClick={() => setRotateOpen(false)}>取消</Button><Button variant="danger" busy={busy} disabled={confirm !== 'ROTATE'} onClick={rotate}>确认轮换</Button></>}>
        {rotation ? <div className="secret-reveal"><span>NEW API KEY</span><div><code>{rotation.key}</code><button className="icon-button" type="button" aria-label="复制新 API 密钥" onClick={copyKey}><Copy size={17} weight="light" /></button></div><p>{rotation.notice}</p></div> : <label className="field field--wide"><span>输入 ROTATE 确认</span><input value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="off" placeholder="ROTATE" /></label>}
      </Modal>
    </div>
  )
}
