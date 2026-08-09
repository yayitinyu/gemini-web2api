import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import { CaretDown } from '@phosphor-icons/react/dist/icons/CaretDown'
import { Check } from '@phosphor-icons/react/dist/icons/Check'
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

export interface SelectOption {
  value: string
  label: string
  description?: string
}

interface SelectProps {
  value: string
  options: SelectOption[]
  onChange: (value: string) => void
  ariaLabel: string
  placeholder?: string
  disabled?: boolean
  compact?: boolean
  className?: string
}

interface SelectMenuPosition {
  left: number
  width: number
  maxHeight: number
  placement: 'top' | 'bottom'
  top?: number
  bottom?: number
}

export function Select({ value, options, onChange, ariaLabel, placeholder = '请选择', disabled = false, compact = false, className }: SelectProps) {
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const [menuPosition, setMenuPosition] = useState<SelectMenuPosition | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listboxID = useId()
  const selectedIndex = options.findIndex((option) => option.value === value)
  const selectedOption = selectedIndex >= 0 ? options[selectedIndex] : null
  const isDisabled = disabled || options.length === 0

  function openMenu(preferredIndex = selectedIndex) {
    if (isDisabled) return
    setActiveIndex(preferredIndex >= 0 ? preferredIndex : 0)
    setOpen(true)
  }

  function closeMenu(restoreFocus = false) {
    setOpen(false)
    setMenuPosition(null)
    if (restoreFocus) triggerRef.current?.focus()
  }

  function choose(index: number) {
    const option = options[index]
    if (!option) return
    onChange(option.value)
    setActiveIndex(index)
    closeMenu(true)
  }

  function moveActive(step: number) {
    setActiveIndex((current) => {
      if (current < 0) return step > 0 ? 0 : options.length - 1
      return (current + step + options.length) % options.length
    })
  }

  function handleKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>) {
    if (isDisabled) return
    if (!open) {
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        openMenu(event.key === 'ArrowUp' && selectedIndex < 0 ? options.length - 1 : selectedIndex)
      }
      return
    }

    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      moveActive(event.key === 'ArrowDown' ? 1 : -1)
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      setActiveIndex(event.key === 'Home' ? 0 : options.length - 1)
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      choose(activeIndex)
    } else if (event.key === 'Escape') {
      event.preventDefault()
      closeMenu(true)
    } else if (event.key === 'Tab') {
      closeMenu()
    }
  }

  useLayoutEffect(() => {
    if (!open) return
    document.body.classList.add('select-open')
    const updatePosition = () => {
      const trigger = triggerRef.current
      if (!trigger) return
      const rect = trigger.getBoundingClientRect()
      const viewportWidth = document.documentElement.clientWidth || window.innerWidth
      const viewportHeight = document.documentElement.clientHeight || window.innerHeight
      const edge = 12
      const gap = 8
      const width = Math.min(Math.max(rect.width, 220), viewportWidth - edge * 2)
      const left = Math.min(Math.max(rect.left, edge), viewportWidth - edge - width)
      const desiredHeight = Math.min(options.length * 58 + 12, 302)
      const spaceBelow = viewportHeight - rect.bottom - gap - edge
      const spaceAbove = rect.top - gap - edge
      const placement = spaceBelow < Math.min(desiredHeight, 174) && spaceAbove > spaceBelow ? 'top' : 'bottom'
      const availableHeight = placement === 'bottom' ? spaceBelow : spaceAbove
      const maxHeight = Math.max(110, Math.min(desiredHeight, availableHeight))

      setMenuPosition({
        left,
        width,
        maxHeight,
        placement,
        ...(placement === 'bottom' ? { top: rect.bottom + gap } : { bottom: viewportHeight - rect.top + gap }),
      })
    }

    updatePosition()
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)
    return () => {
      document.body.classList.remove('select-open')
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
    }
  }, [open, options.length])

  useEffect(() => {
    if (open && isDisabled) closeMenu()
  }, [isDisabled, open])

  const menuStyle: CSSProperties | undefined = menuPosition ? {
    left: menuPosition.left,
    width: menuPosition.width,
    maxHeight: menuPosition.maxHeight,
    top: menuPosition.top,
    bottom: menuPosition.bottom,
  } : undefined

  return (
    <div className={classNames('select-control', compact && 'select-control--compact', open && 'is-open', className)}>
      <button
        ref={triggerRef}
        type="button"
        className="select-trigger"
        role="combobox"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listboxID}
        aria-activedescendant={open && activeIndex >= 0 ? `${listboxID}-option-${activeIndex}` : undefined}
        disabled={isDisabled}
        onClick={() => open ? closeMenu() : openMenu()}
        onKeyDown={handleKeyDown}
      >
        <span className={classNames('select-trigger__value', !selectedOption && 'is-placeholder')}>{selectedOption?.label ?? placeholder}</span>
        <span className="select-trigger__ornament" aria-hidden="true"><CaretDown size={15} weight="bold" /></span>
      </button>
      {open && menuPosition && createPortal(
        <div className="select-popover-layer" onPointerDown={(event) => {
          if (event.target !== event.currentTarget) return
          event.preventDefault()
          closeMenu(true)
        }}>
          <div className={classNames('select-menu', `select-menu--${menuPosition.placement}`)} id={listboxID} role="listbox" aria-label={ariaLabel} style={menuStyle}>
            {options.map((option, index) => (
              <button
                key={option.value}
                id={`${listboxID}-option-${index}`}
                type="button"
                role="option"
                aria-label={option.label}
                aria-selected={option.value === value}
                className={classNames('select-option', option.value === value && 'is-selected', index === activeIndex && 'is-active')}
                tabIndex={-1}
                onMouseEnter={() => setActiveIndex(index)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => choose(index)}
              >
                <span className="select-option__index" aria-hidden="true">{String(index + 1).padStart(2, '0')}</span>
                <span className="select-option__copy"><strong>{option.label}</strong>{option.description && <small>{option.description}</small>}</span>
                <span className="select-option__check" aria-hidden="true"><Check size={14} weight="bold" /></span>
              </button>
            ))}
          </div>
        </div>,
        document.body,
      )}
    </div>
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
  return createPortal(
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
    </div>,
    document.body,
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
