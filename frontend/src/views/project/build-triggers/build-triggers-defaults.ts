import { DEFAULT_BUILD_TRIGGERS, type BuildTriggers } from '@/types/build-triggers'

export function parseBuildTriggers(raw: unknown): BuildTriggers {
  if (!raw || typeof raw !== 'object') {
    return structuredClone(DEFAULT_BUILD_TRIGGERS)
  }
  const o = raw as Record<string, unknown>
  const depends_on = Array.isArray(o.depends_on)
    ? o.depends_on.map((v) => String(v).trim()).filter(Boolean)
    : []
  const schedRaw = o.schedule && typeof o.schedule === 'object' ? (o.schedule as Record<string, unknown>) : {}
  return {
    depends_on,
    schedule: {
      enabled: schedRaw.enabled === true,
      cron: String(schedRaw.cron ?? DEFAULT_BUILD_TRIGGERS.schedule.cron).trim() || DEFAULT_BUILD_TRIGGERS.schedule.cron,
    },
  }
}
