export type SettingsSection = 'sdk' | 'props' | 'vars' | 'upgrade'

export const SETTINGS_SECTIONS: { id: SettingsSection; icon: string; labelKey: string; hintKey: string }[] = [
  { id: 'sdk', icon: 'boxes', labelKey: 'settings.sdk.title', hintKey: 'settings.sdk.hint' },
  { id: 'props', icon: 'sliders-horizontal', labelKey: 'settings.props.title', hintKey: 'settings.props.hint' },
  { id: 'vars', icon: 'list', labelKey: 'settings.vars.title', hintKey: 'settings.vars.hint' },
  { id: 'upgrade', icon: 'upload', labelKey: 'settings.upgrade.title', hintKey: 'settings.upgrade.hint' },
]
