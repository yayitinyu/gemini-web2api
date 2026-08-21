export type AppRoute = 'overview' | 'accounts' | 'network' | 'requests' | 'settings'

export const APP_ROUTES: readonly AppRoute[] = ['overview', 'accounts', 'network', 'requests', 'settings']

export function routeFromPath(pathname: string): AppRoute {
  const segment = pathname.replace(/^\/admin\/?/, '').split('/')[0]
  return APP_ROUTES.includes(segment as AppRoute) ? segment as AppRoute : 'overview'
}

export function pathFor(route: AppRoute): string {
  return `/admin/${route}`
}
