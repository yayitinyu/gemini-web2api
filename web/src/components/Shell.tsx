import type { ReactNode } from 'react'
import { Gear } from '@phosphor-icons/react/dist/icons/Gear'
import { Globe } from '@phosphor-icons/react/dist/icons/Globe'
import { House } from '@phosphor-icons/react/dist/icons/House'
import { Pulse } from '@phosphor-icons/react/dist/icons/Pulse'
import { SignOut } from '@phosphor-icons/react/dist/icons/SignOut'
import { UsersThree } from '@phosphor-icons/react/dist/icons/UsersThree'
import { Brand } from './Brand'
import { classNames } from '../utils'

export type AppRoute = 'overview' | 'accounts' | 'network' | 'requests' | 'settings'

const navigation = [
  { id: 'overview' as const, label: '概览', icon: House },
  { id: 'accounts' as const, label: '账号', icon: UsersThree },
  { id: 'network' as const, label: '网络', icon: Globe },
  { id: 'requests' as const, label: '请求', icon: Pulse },
  { id: 'settings' as const, label: '设置', icon: Gear },
]

function Navigation({ route, navigate, mobile = false }: {
  route: AppRoute
  navigate: (route: AppRoute) => void
  mobile?: boolean
}) {
  return (
    <nav className={mobile ? 'mobile-nav' : 'side-nav'} aria-label="管理导航">
      {navigation.map(({ id, label, icon: Icon }) => (
        <button
          type="button"
          className={classNames('nav-item', route === id && 'is-active')}
          aria-current={route === id ? 'page' : undefined}
          onClick={() => navigate(id)}
          key={id}
        >
          <Icon size={mobile ? 21 : 19} weight={route === id ? 'regular' : 'light'} />
          <span>{label}</span>
        </button>
      ))}
    </nav>
  )
}

export function Shell({ route, navigate, onLogout, children }: {
  route: AppRoute
  navigate: (route: AppRoute) => void
  onLogout: () => void
  children: ReactNode
}) {
  return (
    <div className="app-shell">
      <aside className="rail">
        <Brand />
        <Navigation route={route} navigate={navigate} />
        <div className="rail__footer">
          <div className="rail__status"><span />网关在线</div>
          <button type="button" className="nav-item" onClick={onLogout}>
            <SignOut size={19} weight="light" />
            <span>退出</span>
          </button>
        </div>
      </aside>

      <header className="mobile-header">
        <Brand compact />
        <div className="mobile-header__title">Gemini Web2API</div>
        <button type="button" className="icon-button" aria-label="退出管理面板" onClick={onLogout}>
          <SignOut size={19} weight="light" />
        </button>
      </header>

      <main className="main-canvas">{children}</main>
      <Navigation mobile route={route} navigate={navigate} />
    </div>
  )
}
