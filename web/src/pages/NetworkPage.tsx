import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { ArrowsClockwise } from '@phosphor-icons/react/dist/icons/ArrowsClockwise'
import { Globe } from '@phosphor-icons/react/dist/icons/Globe'
import { PencilSimple } from '@phosphor-icons/react/dist/icons/PencilSimple'
import { Plus } from '@phosphor-icons/react/dist/icons/Plus'
import { TestTube } from '@phosphor-icons/react/dist/icons/TestTube'
import { TrashSimple } from '@phosphor-icons/react/dist/icons/TrashSimple'
import { WarningCircle } from '@phosphor-icons/react/dist/icons/WarningCircle'
import { apiRequest } from '../api/client'
import type { ProbeResult, Proxy } from '../api/types'
import { Button, EmptyState, InlineError, LoadingView, Modal, PageHeader, StatusBadge } from '../components/UI'
import { useAdmin } from '../context'
import { useLoad } from '../useLoad'
import { errorMessage, relativeTime } from '../utils'

function ProxyDialog({ proxy, open, onClose, onSaved }: {
  proxy: Proxy | null, open: boolean, onClose: () => void, onSaved: () => void,
}) {
  const { toast } = useAdmin()
  const [label, setLabel] = useState('')
  const [url, setURL] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')
  useEffect(() => {
    if (!open) return
    setLabel(proxy?.label ?? '')
    setURL('')
    setEnabled(proxy?.enabled ?? true)
    setFormError('')
  }, [open, proxy])

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setFormError('')
    try {
      const payload: Record<string, unknown> = { label, enabled }
      if (!proxy || url.trim()) payload.url = url
      await apiRequest<Proxy>(proxy ? `/api/admin/proxies/${proxy.id}` : '/api/admin/proxies', {
        method: proxy ? 'PUT' : 'POST', body: JSON.stringify(payload),
      })
      toast('success', proxy ? '已保存' : '已添加')
      onSaved(); onClose()
    } catch (reason) {
      setFormError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={proxy ? '编辑代理' : '添加代理'}
      footer={<><Button variant="quiet" onClick={onClose}>取消</Button><Button variant="primary" type="submit" form="proxy-form" busy={busy}>{proxy ? '保存' : '添加'}</Button></>}>
      <form id="proxy-form" className="form-grid" onSubmit={submit}>
        <label className="field field--wide"><span>名称</span><input value={label} onChange={(e) => setLabel(e.target.value)} required maxLength={80} /></label>
        <label className="field field--wide"><span>URL {proxy && <small>留空则保持原值</small>}</span><input value={url} onChange={(e) => setURL(e.target.value)} required={!proxy} placeholder="socks5h://user:password@host:1080" spellCheck={false} /></label>
        <label className="switch-field field--wide"><span><strong>启用</strong><small>关闭后不会被自动选择</small></span><input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} /><i /></label>
        {formError && <p className="form-error field--wide" role="alert">{formError}</p>}
      </form>
    </Modal>
  )
}

export function NetworkPage() {
  const { toast } = useAdmin()
  const [editing, setEditing] = useState<Proxy | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleting, setDeleting] = useState<Proxy | null>(null)
  const [actionID, setActionID] = useState<number | null>(null)

  const loader = useCallback(async () => {
    const result = await apiRequest<{ items: Proxy[] }>('/api/admin/proxies')
    return result.items
  }, [])
  const { data: proxies, error, load } = useLoad(loader)

  async function act(proxy: Proxy, action: 'test' | 'reset') {
    setActionID(proxy.id)
    try {
      if (action === 'test') {
        const result = await apiRequest<ProbeResult>(`/api/admin/proxies/${proxy.id}/test`, { method: 'POST' })
        toast('success', `${proxy.label} · ${result.latency_ms} ms`)
      } else {
        await apiRequest<void>(`/api/admin/proxies/${proxy.id}/reset`, { method: 'POST' })
        toast('success', '已重置熔断')
      }
      void load()
    } catch (reason) {
      toast('error', errorMessage(reason)); void load()
    } finally { setActionID(null) }
  }

  async function remove() {
    if (!deleting) return
    setActionID(deleting.id)
    try {
      await apiRequest<void>(`/api/admin/proxies/${deleting.id}`, { method: 'DELETE' })
      toast('success', '已删除'); setDeleting(null); void load()
    } catch (reason) { toast('error', errorMessage(reason)) }
    finally { setActionID(null) }
  }

  if (proxies === null && !error) return <LoadingView />
  return (
    <div className="page">
      <PageHeader title="网络" action={<Button variant="primary" icon={<Plus size={18} />} onClick={() => { setEditing(null); setDialogOpen(true) }}>添加</Button>} />
      {error && <InlineError message={error} onRetry={load} />}

      <section className="resource-panel">
        <header className="section-heading">
          <h2>代理</h2>
          <span className="section-count">{proxies?.filter((item) => item.enabled).length ?? 0} 启用</span>
        </header>
        {proxies?.length === 0 ? (
          <EmptyState icon={<Globe size={28} weight="light" />} title="当前直连" />
        ) : <div className="resource-list">{proxies?.map((proxy) => (
          <article className="resource-row" key={proxy.id}>
            <div className="resource-row__mark resource-row__mark--network"><Globe size={21} weight="light" /></div>
            <div className="resource-row__main"><div className="resource-row__title"><h3>{proxy.label}</h3><StatusBadge status={proxy.status} enabled={proxy.enabled} /></div><code>{proxy.url_summary}</code></div>
            <div className="resource-row__stat"><span>最近成功</span><strong>{relativeTime(proxy.last_success_at)}</strong></div>
            <div className="resource-row__stat"><span>连续失败</span><strong>{proxy.failure_count}</strong></div>
            <div className="resource-row__actions">
              {proxy.status === 'cooldown' && <button className="icon-button" type="button" aria-label={`重置 ${proxy.label} 熔断`} onClick={() => act(proxy, 'reset')}><ArrowsClockwise size={18} weight="light" /></button>}
              <Button variant="quiet" busy={actionID === proxy.id} icon={<TestTube size={17} weight="light" />} onClick={() => act(proxy, 'test')}>检测</Button>
              <button className="icon-button" type="button" aria-label={`编辑 ${proxy.label}`} onClick={() => { setEditing(proxy); setDialogOpen(true) }}><PencilSimple size={18} weight="light" /></button>
              <button className="icon-button icon-button--danger" type="button" aria-label={`删除 ${proxy.label}`} onClick={() => setDeleting(proxy)}><TrashSimple size={18} weight="light" /></button>
            </div>
            {proxy.last_error && <div className="resource-row__error">{proxy.last_error}</div>}
          </article>
        ))}</div>}
      </section>

      <ProxyDialog proxy={editing} open={dialogOpen} onClose={() => setDialogOpen(false)} onSaved={load} />
      <Modal open={Boolean(deleting)} onClose={() => setDeleting(null)} size="small" title="删除代理？" description="绑定该出口的账号将改回自动选择。" footer={<><Button variant="quiet" onClick={() => setDeleting(null)}>取消</Button><Button variant="danger" busy={Boolean(deleting && actionID === deleting.id)} icon={<TrashSimple size={17} />} onClick={remove}>删除</Button></>}>
        <div className="confirm-copy"><WarningCircle size={24} weight="light" /><p>即将删除 <strong>{deleting?.label}</strong>。</p></div>
      </Modal>
    </div>
  )
}
