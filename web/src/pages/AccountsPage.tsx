import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { CheckCircle } from '@phosphor-icons/react/dist/icons/CheckCircle'
import { PencilSimple } from '@phosphor-icons/react/dist/icons/PencilSimple'
import { Plus } from '@phosphor-icons/react/dist/icons/Plus'
import { TestTube } from '@phosphor-icons/react/dist/icons/TestTube'
import { TrashSimple } from '@phosphor-icons/react/dist/icons/TrashSimple'
import { UsersThree } from '@phosphor-icons/react/dist/icons/UsersThree'
import { apiRequest } from '../api/client'
import type { Account, ProbeResult, Proxy } from '../api/types'
import { Button, EmptyState, LoadingView, Modal, PageHeader, StatusBadge, type ToastKind } from '../components/UI'
import { errorMessage, relativeTime } from '../utils'

interface AccountDraft {
  label: string
  cookie: string
  enabled: boolean
  note: string
  proxy_id: number
}

function AccountDialog({ account, proxies, open, onClose, onSaved, toast }: {
  account: Account | null
  proxies: Proxy[]
  open: boolean
  onClose: () => void
  onSaved: () => void
  toast: (kind: ToastKind, text: string) => void
}) {
  const [draft, setDraft] = useState<AccountDraft>({ label: '', cookie: '', enabled: true, note: '', proxy_id: 0 })
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')

  useEffect(() => {
    if (!open) return
    setDraft({
      label: account?.label ?? '', cookie: '', enabled: account?.enabled ?? true,
      note: account?.note ?? '', proxy_id: account?.proxy_id ?? 0,
    })
    setFormError('')
  }, [account, open])

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setFormError('')
    try {
      const payload: Record<string, unknown> = {
        label: draft.label, enabled: draft.enabled, note: draft.note, proxy_id: draft.proxy_id,
      }
      if (!account || draft.cookie.trim()) payload.cookie = draft.cookie
      await apiRequest<Account>(account ? `/api/admin/accounts/${account.id}` : '/api/admin/accounts', {
        method: account ? 'PUT' : 'POST', body: JSON.stringify(payload),
      })
      toast('success', account ? '账号设置已更新' : '账号已加入凭据池')
      onSaved()
      onClose()
    } catch (reason) {
      setFormError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={account ? '编辑 Gemini 账号' : '添加 Gemini 账号'} description="Cookie 会在写入数据库前使用 AES-GCM 加密。"
      footer={<><Button variant="quiet" type="button" onClick={onClose}>取消</Button><Button variant="primary" type="submit" form="account-form" busy={busy}>{account ? '保存更改' : '添加账号'}</Button></>}>
      <form id="account-form" className="form-grid" onSubmit={submit}>
        <label className="field field--wide"><span>显示名称</span><input value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} required maxLength={80} placeholder="例如：主账号" /></label>
        <label className="field field--wide"><span>Google Cookie {account && <small>留空则保持原值</small>}</span><textarea value={draft.cookie} onChange={(e) => setDraft({ ...draft, cookie: e.target.value })} required={!account} rows={5} placeholder="粘贴包含 __Secure-1PSID 与 __Secure-1PSIDTS 的 Cookie" spellCheck={false} /></label>
        <label className="field"><span>固定出口</span><select value={draft.proxy_id} onChange={(e) => setDraft({ ...draft, proxy_id: Number(e.target.value) })}><option value={0}>自动选择 / 直连</option>{proxies.map((proxy) => <option value={proxy.id} key={proxy.id}>{proxy.label}</option>)}</select></label>
        <label className="switch-field"><span><strong>启用账号</strong><small>关闭后不参与轮询</small></span><input type="checkbox" checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} /><i /></label>
        <label className="field field--wide"><span>备注</span><input value={draft.note} onChange={(e) => setDraft({ ...draft, note: e.target.value })} maxLength={200} placeholder="可选，仅管理员可见" /></label>
        {formError && <p className="form-error field--wide" role="alert">{formError}</p>}
      </form>
    </Modal>
  )
}

export function AccountsPage({ toast }: { toast: (kind: ToastKind, text: string) => void }) {
  const [accounts, setAccounts] = useState<Account[] | null>(null)
  const [proxies, setProxies] = useState<Proxy[]>([])
  const [editing, setEditing] = useState<Account | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleting, setDeleting] = useState<Account | null>(null)
  const [actionID, setActionID] = useState<number | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      setError('')
      const [accountResult, proxyResult] = await Promise.all([
        apiRequest<{ items: Account[] }>('/api/admin/accounts'),
        apiRequest<{ items: Proxy[] }>('/api/admin/proxies'),
      ])
      setAccounts(accountResult.items)
      setProxies(proxyResult.items)
    } catch (reason) {
      setError(errorMessage(reason))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  function addAccount() { setEditing(null); setDialogOpen(true) }
  function editAccount(account: Account) { setEditing(account); setDialogOpen(true) }

  async function test(account: Account) {
    setActionID(account.id)
    try {
      const result = await apiRequest<ProbeResult>(`/api/admin/accounts/${account.id}/test`, { method: 'POST' })
      toast('success', `${account.label} 连接正常 · ${result.latency_ms} ms`)
      void load()
    } catch (reason) {
      toast('error', `检测失败：${errorMessage(reason)}`)
      void load()
    } finally {
      setActionID(null)
    }
  }

  async function remove() {
    if (!deleting) return
    setActionID(deleting.id)
    try {
      await apiRequest<void>(`/api/admin/accounts/${deleting.id}`, { method: 'DELETE' })
      toast('success', '账号已从凭据池移除')
      setDeleting(null)
      void load()
    } catch (reason) {
      toast('error', errorMessage(reason))
    } finally {
      setActionID(null)
    }
  }

  if (accounts === null && !error) return <LoadingView label="正在读取账号凭据池" />

  return (
    <div className="page">
      <PageHeader eyebrow="CREDENTIAL ORCHESTRATION" title="账号池" description="轮询多个 Gemini Web 会话，并独立观察每个凭据的健康状态。" action={<Button variant="primary" icon={<Plus size={18} />} onClick={addAccount}>添加账号</Button>} />
      {error && <div className="inline-error"><span>{error}</span><Button variant="quiet" onClick={load}>重新读取</Button></div>}

      <section className="resource-summary">
        <div><span>账号总数</span><strong>{accounts?.length ?? 0}</strong></div>
        <div><span>参与轮询</span><strong>{accounts?.filter((item) => item.enabled).length ?? 0}</strong></div>
        <div><span>健康会话</span><strong>{accounts?.filter((item) => item.enabled && item.status === 'healthy').length ?? 0}</strong></div>
        <p>凭据以最少最近使用策略分配；失败会切换到下一个可用账号。</p>
      </section>

      <section className="resource-panel">
        <header className="section-heading"><div><span>ENCRYPTED CREDENTIALS</span><h2>Gemini Web 会话</h2></div></header>
        {accounts?.length === 0 ? (
          <EmptyState icon={<UsersThree size={28} weight="light" />} title="等待第一个账号" description="添加账号后，可以先运行一次单独检测，再开放 API 流量。" action={<Button variant="secondary" icon={<Plus size={17} />} onClick={addAccount}>添加账号</Button>} />
        ) : (
          <div className="resource-list">
            {accounts?.map((account) => (
              <article className="resource-row" key={account.id}>
                <div className="resource-row__mark">{account.label.slice(0, 2).toUpperCase()}</div>
                <div className="resource-row__main">
                  <div className="resource-row__title"><h3>{account.label}</h3><StatusBadge status={account.status} enabled={account.enabled} /></div>
                  <code>{account.cookie_summary}</code>
                  {account.note && <p>{account.note}</p>}
                </div>
                <div className="resource-row__stat"><span>最近成功</span><strong>{relativeTime(account.last_success_at)}</strong></div>
                <div className="resource-row__stat"><span>连续失败</span><strong>{account.failure_count}</strong></div>
                <div className="resource-row__actions">
                  <Button variant="quiet" busy={actionID === account.id} icon={actionID === account.id ? undefined : <TestTube size={17} weight="light" />} onClick={() => test(account)}>检测</Button>
                  <button className="icon-button" type="button" aria-label={`编辑 ${account.label}`} onClick={() => editAccount(account)}><PencilSimple size={18} weight="light" /></button>
                  <button className="icon-button icon-button--danger" type="button" aria-label={`删除 ${account.label}`} onClick={() => setDeleting(account)}><TrashSimple size={18} weight="light" /></button>
                </div>
                {account.last_error && <div className="resource-row__error">{account.last_error}</div>}
              </article>
            ))}
          </div>
        )}
      </section>

      <AccountDialog account={editing} proxies={proxies} open={dialogOpen} onClose={() => setDialogOpen(false)} onSaved={load} toast={toast} />
      <Modal open={Boolean(deleting)} onClose={() => setDeleting(null)} size="small" title="移除账号？" description="该凭据会立即停止参与新请求，历史审计元数据会保留。"
        footer={<><Button variant="quiet" onClick={() => setDeleting(null)}>取消</Button><Button variant="danger" busy={Boolean(deleting && actionID === deleting.id)} icon={<TrashSimple size={17} />} onClick={remove}>确认移除</Button></>}>
        <div className="confirm-copy"><CheckCircle size={24} weight="light" /><p>即将移除 <strong>{deleting?.label}</strong>。加密 Cookie 会一并删除，无法从面板恢复。</p></div>
      </Modal>
    </div>
  )
}
