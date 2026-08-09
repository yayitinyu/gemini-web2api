import { useEffect, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { CheckCircle } from '@phosphor-icons/react/dist/icons/CheckCircle'
import { Info } from '@phosphor-icons/react/dist/icons/Info'
import { WarningCircle } from '@phosphor-icons/react/dist/icons/WarningCircle'
import { X } from '@phosphor-icons/react/dist/icons/X'
import { classNames } from '../utils'

type ButtonVariant = 'primary' | 'secondary' | 'quiet' | 'danger'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  busy?: boolean
  icon?: ReactNode
}

export function Button({ className, variant = 'secondary', busy, icon, children, disabled, ...props }: ButtonProps) {
  return (
    <button
      className={classNames('button', `button--${variant}`, busy && 'is-busy', className)}
      disabled={disabled || busy}
      {...props}
    >
      {busy ? <span className="button__spinner" aria-hidden="true" /> : icon}
      {children && <span>{children}</span>}
    </button>
  )
}

interface PageHeaderProps {
  eyebrow?: string
  title: string
  description: string
  action?: ReactNode
}

export function PageHeader({ eyebrow, title, description, action }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div>
        {eyebrow && <p className="eyebrow">{eyebrow}</p>}
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action && <div className="page-header__action">{action}</div>}
    </header>
  )
}

export function StatusBadge({ status, enabled = true }: { status: string, enabled?: boolean }) {
  const normalized = !enabled ? 'disabled' : status
  const labels: Record<string, string> = {
    healthy: '健康', unhealthy: '异常', cooldown: '冷却中', unknown: '未检测', disabled: '已停用',
  }
  return (
    <span className={classNames('status-badge', `status-badge--${normalized}`)}>
      <span className="status-badge__dot" />
      {labels[normalized] ?? normalized}
    </span>
  )
}

interface ModalProps {
  open: boolean
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
  size?: 'small' | 'medium'
}

export function Modal({ open, title, description, children, footer, onClose, size = 'medium' }: ModalProps) {
  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.body.classList.add('modal-open')
    window.addEventListener('keydown', onKeyDown)
    return () => {
      document.body.classList.remove('modal-open')
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="modal-layer" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className={classNames('modal', `modal--${size}`)} role="dialog" aria-modal="true" aria-labelledby="modal-title">
        <header className="modal__header">
          <div>
            <h2 id="modal-title">{title}</h2>
            {description && <p>{description}</p>}
          </div>
          <button className="icon-button" type="button" aria-label="关闭" onClick={onClose}>
            <X size={19} weight="light" />
          </button>
        </header>
        <div className="modal__body">{children}</div>
        {footer && <footer className="modal__footer">{footer}</footer>}
      </section>
    </div>
  )
}

export function EmptyState({ title, description, action, icon }: {
  title: string
  description: string
  action?: ReactNode
  icon?: ReactNode
}) {
  return (
    <div className="empty-state">
      {icon && <div className="empty-state__icon">{icon}</div>}
      <h3>{title}</h3>
      <p>{description}</p>
      {action}
    </div>
  )
}

export type ToastKind = 'success' | 'error' | 'info'
export interface ToastMessage { id: number, kind: ToastKind, text: string }

export function ToastHost({ messages, dismiss }: { messages: ToastMessage[], dismiss: (id: number) => void }) {
  const icons = {
    success: <CheckCircle size={20} weight="light" />,
    error: <WarningCircle size={20} weight="light" />,
    info: <Info size={20} weight="light" />,
  }
  return (
    <div className="toast-host" aria-live="polite">
      {messages.map((message) => (
        <div className={classNames('toast', `toast--${message.kind}`)} key={message.id}>
          {icons[message.kind]}
          <span>{message.text}</span>
          <button type="button" aria-label="关闭通知" onClick={() => dismiss(message.id)}><X size={16} /></button>
        </div>
      ))}
    </div>
  )
}

export function LoadingView({ label = '正在读取网关状态' }: { label?: string }) {
  return (
    <div className="loading-view" role="status">
      <span className="loading-orbit"><span /></span>
      <p>{label}</p>
    </div>
  )
}
