import type { CommandFamily } from './command-patch'

/** 批量构建后脚本默认模板（使用 AP_* 环境变量）。 */
export function defaultPostShellCommand(family: CommandFamily): string {
  const dest = 'dest="/home/appinstall/${AP_WORKSPACE_CODE}/${AP_PROJECT_NAME}"'
  if (family === 'maven') {
    return `${dest}\nmkdir -p "$dest" && cp -rf target/*.jar "$dest/"`
  }
  return `${dest}\nmkdir -p "$dest" && cp -rf ./dist/* "$dest/"`
}
