import { describe, expect, it } from 'vitest'
import { pathFor, routeFromPath } from './routes'

describe('admin routes', () => {
  it('reads a known admin segment and falls back to overview', () => {
    expect(routeFromPath('/admin/settings')).toBe('settings')
    expect(routeFromPath('/admin/settings/extra')).toBe('settings')
    expect(routeFromPath('/admin/')).toBe('overview')
    expect(routeFromPath('/admin/unknown')).toBe('overview')
    expect(pathFor('accounts')).toBe('/admin/accounts')
  })
})
