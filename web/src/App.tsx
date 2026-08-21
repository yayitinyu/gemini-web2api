import { useCallback, useEffect, useRef, useState } from 'react'
import { APIError, checkSession, logout, setUnauthorizedHandler } from './api/client'
import { AdminProvider } from './context'
import { pathFor, routeFromPath, type AppRoute } from './routes'
import { Shell } from './components/Shell'
import { LoadingView, ToastHost, type ToastKind, type ToastMessage } from './components/UI'
import { AccountsPage } from './pages/AccountsPage'
import { LoginPage } from './pages/LoginPage'
import { NetworkPage } from './pages/NetworkPage'
import { OverviewPage } from './pages/OverviewPage'
import { RequestsPage } from './pages/RequestsPage'
import { SettingsPage } from './pages/SettingsPage'

export default function App() {
  const [authentication, setAuthentication] = useState<'checking' | 'authenticated' | 'anonymous'>('checking')
  const [route, setRoute] = useState<AppRoute>(() => routeFromPath(window.location.pathname))
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const toastSeq = useRef(0)

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
    window.history.pushState({}, '', pathFor(next))
    setRoute(next)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [])

  useEffect(() => {
    const onPopState = () => setRoute(routeFromPath(window.location.pathname))
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const toast = useCallback((kind: ToastKind, text: string) => {
    const id = ++toastSeq.current
    setToasts((current) => [...current.slice(-2), { id, kind, text }])
    window.setTimeout(() => setToasts((current) => current.filter((item) => item.id !== id)), 4200)
  }, [])

  async function signOut() {
    try { await logout() } finally {
      setAuthentication('anonymous')
      window.history.replaceState({}, '', '/admin/')
      setRoute('overview')
    }
  }

  if (authentication === 'checking') {
    return <div className="app-boot"><LoadingView /></div>
  }
  if (authentication === 'anonymous') {
    return <LoginPage onAuthenticated={() => {
      setAuthentication('authenticated')
      const next = routeFromPath(window.location.pathname)
      window.history.replaceState({}, '', pathFor(next))
      setRoute(next)
    }} />
  }

  const page = {
    overview: <OverviewPage />,
    accounts: <AccountsPage />,
    network: <NetworkPage />,
    requests: <RequestsPage />,
    settings: <SettingsPage />,
  }[route]

  return (
    <AdminProvider value={{ route, navigate, toast }}>
      <Shell route={route} navigate={navigate} onLogout={signOut}>{page}</Shell>
      <ToastHost messages={toasts} dismiss={(id) => setToasts((current) => current.filter((item) => item.id !== id))} />
    </AdminProvider>
  )
}
