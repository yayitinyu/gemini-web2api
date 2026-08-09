import { useCallback, useEffect, useState } from 'react'
import { APIError, checkSession, logout, setUnauthorizedHandler } from './api/client'
import { Shell, type AppRoute } from './components/Shell'
import { LoadingView, ToastHost, type ToastKind, type ToastMessage } from './components/UI'
import { AccountsPage } from './pages/AccountsPage'
import { LoginPage } from './pages/LoginPage'
import { NetworkPage } from './pages/NetworkPage'
import { OverviewPage } from './pages/OverviewPage'
import { RequestsPage } from './pages/RequestsPage'
import { SettingsPage } from './pages/SettingsPage'

const validRoutes: AppRoute[] = ['overview', 'accounts', 'network', 'requests', 'settings']

function routeFromLocation(): AppRoute {
  const segment = window.location.pathname.replace(/^\/admin\/?/, '').split('/')[0]
  return validRoutes.includes(segment as AppRoute) ? segment as AppRoute : 'overview'
}

export default function App() {
  const [authentication, setAuthentication] = useState<'checking' | 'authenticated' | 'anonymous'>('checking')
  const [route, setRoute] = useState<AppRoute>(routeFromLocation)
  const [toasts, setToasts] = useState<ToastMessage[]>([])

  const markAnonymous = useCallback(() => setAuthentication('anonymous'), [])

  useEffect(() => {
    setUnauthorizedHandler(markAnonymous)
    void checkSession()
      .then(() => setAuthentication('authenticated'))
      .catch((error: unknown) => {
        if (!(error instanceof APIError) || error.status !== 401) {
          // The login screen can surface a network/server error on the next attempt.
        }
        setAuthentication('anonymous')
      })
    return () => setUnauthorizedHandler(undefined)
  }, [markAnonymous])

  const navigate = useCallback((next: AppRoute) => {
    window.history.pushState({}, '', `/admin/${next}`)
    setRoute(next)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [])

  useEffect(() => {
    const onPopState = () => setRoute(routeFromLocation())
    const onNavigate = (event: Event) => {
      const next = (event as CustomEvent<string>).detail as AppRoute
      if (validRoutes.includes(next)) navigate(next)
    }
    window.addEventListener('popstate', onPopState)
    window.addEventListener('navigate-admin', onNavigate)
    return () => {
      window.removeEventListener('popstate', onPopState)
      window.removeEventListener('navigate-admin', onNavigate)
    }
  }, [navigate])

  const toast = useCallback((kind: ToastKind, text: string) => {
    const id = Date.now() + Math.floor(Math.random() * 1000)
    setToasts((current) => [...current.slice(-2), { id, kind, text }])
    window.setTimeout(() => setToasts((current) => current.filter((item) => item.id !== id)), 4200)
  }, [])

  async function signOut() {
    try { await logout() } finally {
      setAuthentication('anonymous')
      window.history.replaceState({}, '', '/admin/')
    }
  }

  if (authentication === 'checking') {
    return <div className="app-boot"><LoadingView label="正在建立安全会话" /></div>
  }
  if (authentication === 'anonymous') {
    return <LoginPage onAuthenticated={() => {
      setAuthentication('authenticated')
      window.history.replaceState({}, '', '/admin/overview')
      setRoute('overview')
    }} />
  }

  const page = {
    overview: <OverviewPage toast={toast} />,
    accounts: <AccountsPage toast={toast} />,
    network: <NetworkPage toast={toast} />,
    requests: <RequestsPage />,
    settings: <SettingsPage toast={toast} />,
  }[route]

  return (
    <>
      <Shell route={route} navigate={navigate} onLogout={signOut}>{page}</Shell>
      <ToastHost messages={toasts} dismiss={(id) => setToasts((current) => current.filter((item) => item.id !== id))} />
    </>
  )
}
