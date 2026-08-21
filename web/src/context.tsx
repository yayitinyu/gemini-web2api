import { createContext, useContext, type ReactNode } from 'react'
import type { AppRoute } from './routes'
import type { ToastKind } from './components/UI'

export interface AdminApp {
  route: AppRoute
  navigate: (route: AppRoute) => void
  toast: (kind: ToastKind, text: string) => void
}

const AdminContext = createContext<AdminApp | null>(null)

export function AdminProvider({ value, children }: { value: AdminApp, children: ReactNode }) {
  return <AdminContext.Provider value={value}>{children}</AdminContext.Provider>
}

export function useAdmin(): AdminApp {
  const value = useContext(AdminContext)
  if (!value) throw new Error('useAdmin must be used within AdminProvider')
  return value
}
