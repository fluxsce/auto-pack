export interface BuildSchedule {
  enabled: boolean
  cron: string
}

export interface BuildTriggers {
  depends_on: string[]
  schedule: BuildSchedule
}

export const DEFAULT_BUILD_TRIGGERS: BuildTriggers = {
  depends_on: [],
  schedule: { enabled: false, cron: '0 2 * * *' },
}
