/** Primary console navigation (horizontal top nav, enterprise console pattern). */
export interface AppNavItem {
  path: string
  labelKey: string
  routeName: string
  devOnly?: boolean
}

export const APP_NAV_ITEMS: AppNavItem[] = [
  { path: '/projects', labelKey: 'projectNav', routeName: 'projects' },
  { path: '/env', labelKey: 'envNav', routeName: 'env' },
  { path: '/users', labelKey: 'userNav', routeName: 'users' },
  { path: '/operation-logs', labelKey: 'opLogNav', routeName: 'operation-logs' },
  { path: '/dev-test', labelKey: 'devTestPanel', routeName: 'dev-test', devOnly: true },
]
