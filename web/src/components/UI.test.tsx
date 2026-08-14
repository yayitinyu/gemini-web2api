import { useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { Button, Modal, Select } from './UI'

function ModalHarness() {
  const [open, setOpen] = useState(true)
  return (
    <div data-testid="transformed-page">
      <Modal
        open={open}
        title="测试弹窗"
        description="验证弹窗脱离页面动画的包含块。"
        onClose={() => setOpen(false)}
        footer={<Button onClick={() => setOpen(false)}>取消</Button>}
      >
        <label>显示名称<input /></label>
      </Modal>
    </div>
  )
}

describe('Modal', () => {
  it('portals to the document body and closes from the header action', async () => {
    const user = userEvent.setup()
    render(<ModalHarness />)

    const dialog = screen.getByRole('dialog', { name: '测试弹窗' })
    expect(dialog.parentElement?.classList.contains('modal-layer')).toBe(true)
    expect(dialog.parentElement?.parentElement).toBe(document.body)
    expect(document.body.classList.contains('modal-open')).toBe(true)

    await user.click(screen.getByRole('button', { name: '关闭' }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: '测试弹窗' })).toBeNull())
    expect(document.body.classList.contains('modal-open')).toBe(false)
  })
})

function SelectHarness() {
  const [value, setValue] = useState('gemini-3.7-flash')
  return <Select ariaLabel="默认模型" value={value} onChange={setValue} options={[
    { value: 'gemini-3.7-flash', label: 'gemini-3.7-flash', description: '低延迟通用生成' },
    { value: 'gemini-3.1-pro', label: 'gemini-3.1-pro', description: '更强推理与复杂任务' },
  ]} />
}

describe('Select', () => {
  it('renders a custom listbox portal and supports pointer selection', async () => {
    const user = userEvent.setup()
    const { container } = render(<SelectHarness />)
    const trigger = screen.getByRole('combobox', { name: '默认模型' })

    expect(container.querySelector('select')).toBeNull()
    await user.click(trigger)

    const listbox = screen.getByRole('listbox', { name: '默认模型' })
    expect(document.body.classList.contains('select-open')).toBe(true)
    expect(listbox.parentElement).toBe(document.body.lastElementChild)
    expect(screen.getByRole('option', { name: 'gemini-3.7-flash' }).getAttribute('aria-selected')).toBe('true')

    await user.click(screen.getByRole('option', { name: 'gemini-3.1-pro' }))
    expect(trigger.textContent).toContain('gemini-3.1-pro')
    expect(screen.queryByRole('listbox', { name: '默认模型' })).toBeNull()
    expect(document.body.classList.contains('select-open')).toBe(false)
  })

  it('supports arrow, enter, and escape keyboard controls', async () => {
    const user = userEvent.setup()
    render(<SelectHarness />)
    const trigger = screen.getByRole('combobox', { name: '默认模型' })

    trigger.focus()
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')
    expect(trigger.textContent).toContain('gemini-3.1-pro')

    await user.keyboard('{ArrowDown}{Escape}')
    expect(screen.queryByRole('listbox', { name: '默认模型' })).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })
})
