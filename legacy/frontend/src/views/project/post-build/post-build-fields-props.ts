import type { EnvSetting } from '@/types'

/** Shared props for per-type post-build field components. */
export interface PostBuildFieldsProps {
  envOptions: (envType: string) => EnvSetting[]
}
