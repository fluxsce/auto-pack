/** 运维告知邮件。推送后告知仍由前端拼装原文发送；project_update 仅给环境页人为改动通知。 */

import { normalizeRemoteDir, resolvePushLayout } from './workspace-ftp'

export type NotifyMailTemplate = 'ftp_update' | 'project_update'

export type FtpNotifyEmailFields = {
  projectLabel: string
  updateContent: string
  taskNo: string
  ftpVersionSource: string
  updateMethod: string
  needRestart: string
  browserCacheRefresh: string
  impactScope: string
  updateScope: string
  affectsApi: string
  warPackage: string
  other: string
  backup: string
}

export const FTP_NOTIFY_UPDATE_METHOD = '整包更新'
export const FTP_NOTIFY_FROM_NAME = 'DATAHUB 更新通知'
export const PROJECT_UPDATE_FROM_NAME = '项目更新通知'
export const FTP_NOTIFY_SCOPE_OPTIONS = ['DATAHUB共享lib', 'DATAHUB界面', 'DATAHUB服务'] as const

const TEMPLATE_META: Record<NotifyMailTemplate, { status: string; brand: string; fromName: string }> = {
  ftp_update: { status: 'FTP已更新', brand: 'DATAHUB 更新通知', fromName: FTP_NOTIFY_FROM_NAME },
  project_update: { status: '项目已更新', brand: '项目更新通知', fromName: PROJECT_UPDATE_FROM_NAME },
}

export function notifyTemplateMeta(template: NotifyMailTemplate = 'ftp_update') {
  return TEMPLATE_META[template] ?? TEMPLATE_META.ftp_update
}

/** JSON fields for POST /envs/{id}/mail template send. */
export function toUpdateMailFieldsPayload(fields: FtpNotifyEmailFields) {
  return {
    project_label: fields.projectLabel,
    update_content: fields.updateContent,
    task_no: fields.taskNo,
    ftp_version_source: fields.ftpVersionSource,
    update_method: fields.updateMethod,
    need_restart: fields.needRestart,
    browser_cache_refresh: fields.browserCacheRefresh,
    impact_scope: fields.impactScope,
    update_scope: fields.updateScope,
    affects_api: fields.affectsApi,
    war_package: fields.warPackage,
    other: fields.other,
    backup: fields.backup,
  }
}

const GENERIC_WORKSPACE_CODES = new Set(['default', 'defaults', 'workspace', 'ws'])

export function dayGreeting(at = new Date()): string {
  const h = at.getHours()
  if (h < 12) return '上午好'
  if (h < 18) return '下午好'
  return '晚上好'
}

export function formatNotifyUpdateContent(taskRef: string): string {
  const s = taskRef.trim()
  if (!s) return ''
  if (/^申请更新/.test(s)) return s
  return `申请更新${s}`
}

export function resolveNotifyProjectLabel(
  workspaceCode: string,
  workspaceName: string,
  projectName: string,
): string {
  const code = workspaceCode.trim()
  if (code && !GENERIC_WORKSPACE_CODES.has(code.toLowerCase())) return code
  const name = workspaceName.trim()
  if (name && !GENERIC_WORKSPACE_CODES.has(name.toLowerCase())) return name
  return projectName.trim()
}

export function buildFtpNotifySummary(fields: Pick<FtpNotifyEmailFields, 'projectLabel' | 'updateContent'>): string {
  const label = fields.projectLabel.trim()
  const content = fields.updateContent.trim()
  if (label && content) return `【${label}】${content}`
  if (content) return content
  if (label) return `【${label}】`
  return ''
}

export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export const DEFAULT_NOTIFY_EMAIL_DOMAIN = 'flux.com.cn'

export function normalizeNotifyRecipient(raw: string, domain = DEFAULT_NOTIFY_EMAIL_DOMAIN): string {
  const token = raw.trim()
  if (!token) return ''
  if (token.includes('@')) {
    const [local, host = ''] = token.split('@')
    if (!local.trim()) return ''
    if (!host.trim()) return `${local.trim()}@${domain}`
    return `${local.trim()}@${host.trim()}`
  }
  return `${token}@${domain}`
}

export function normalizeNotifyRecipients(raw: string, domain = DEFAULT_NOTIFY_EMAIL_DOMAIN): string {
  return raw
    .replaceAll(';', ',')
    .split(',')
    .map((part) => normalizeNotifyRecipient(part, domain))
    .filter(Boolean)
    .join(', ')
}

const HIGHLIGHT_LABELS = new Set(['更新内容', 'FTP版本来源', '更新war包'])

function notifyRows(fields: FtpNotifyEmailFields, summary: string): Array<[string, string]> {
  const rows: Array<[string, string]> = [
    ['更新内容', summary],
    ['任务编号', fields.taskNo.trim()],
    ['FTP版本来源', fields.ftpVersionSource.trim()],
    ['更新方式', fields.updateMethod.trim() || FTP_NOTIFY_UPDATE_METHOD],
    ['是否重启', fields.needRestart.trim() || '是'],
    ['浏览器缓存刷新', fields.browserCacheRefresh.trim() || '是'],
    ['影响范围', fields.impactScope.trim() || '全局'],
    ['更新范围', fields.updateScope.trim()],
    ['是否影响接口运行', fields.affectsApi.trim() || '是'],
    ['更新war包', fields.warPackage.trim()],
  ]
  const other = fields.other.trim()
  if (other) rows.push(['其他', other])
  rows.push(['备份', fields.backup.trim()])
  return rows
}

function splitPackageNames(raw: string): string[] {
  return raw
    .split(/[、,，;；]/)
    .map((s) => s.trim())
    .filter(Boolean)
}

function renderRowValue(label: string, value: string, highlight: boolean): string {
  const text = value.trim() || '—'
  if (label === '更新war包') {
    const pkgs = splitPackageNames(text)
    if (pkgs.length > 0) {
      return pkgs
        .map(
          (pkg) =>
            `<span style="display:inline-block;margin:0 6px 2px 0;padding:2px 8px;border-radius:999px;background:#dbeafe;color:#1e3a8a;font-size:12px;font-weight:700;line-height:1.4;">${escapeHtml(pkg)}</span>`,
        )
        .join('')
    }
  }
  if (highlight) {
    return `<span style="font-weight:700;color:#0f4c81;">${escapeHtml(text)}</span>`
  }
  return escapeHtml(text)
}

function renderRows(rows: Array<[string, string]>): string {
  return rows
    .map(([label, value], index) => {
      const highlight = HIGHLIGHT_LABELS.has(label)
      const bg = highlight ? '#e8f1fa' : index % 2 === 0 ? '#f7f9fc' : '#ffffff'
      const labelColor = highlight ? '#0f4c81' : '#5b677a'
      const labelWeight = highlight ? '700' : '400'
      const border = highlight ? '1px solid #c5d8eb' : '1px solid #e6ebf2'
      return `<tr>
  <td style="padding:10px 14px;border-bottom:${border};color:${labelColor};font-size:13px;font-weight:${labelWeight};width:128px;vertical-align:top;background:${bg};">${escapeHtml(label)}</td>
  <td style="padding:10px 14px;border-bottom:${border};color:#1f2937;font-size:13px;line-height:1.55;word-break:break-all;background:${bg};">${renderRowValue(label, value, highlight)}</td>
</tr>`
    })
    .join('')
}

export function buildFtpNotifyEmailBody(
  fields: FtpNotifyEmailFields,
  at = new Date(),
  template: NotifyMailTemplate = 'ftp_update',
): string {
  const summary = buildFtpNotifySummary(fields)
  const { status } = notifyTemplateMeta(template)
  const lines = [
    `${dayGreeting(at)}。${summary}${summary ? ', ' : ''}${status}`,
    ...notifyRows(fields, summary).map(([k, v]) => `${k}：${v}`),
  ]
  return lines.join('\n')
}

export function buildFtpNotifyEmailHTML(
  fields: FtpNotifyEmailFields,
  at = new Date(),
  template: NotifyMailTemplate = 'ftp_update',
): string {
  const summary = buildFtpNotifySummary(fields)
  const { status, brand } = notifyTemplateMeta(template)
  const greeting = `${dayGreeting(at)}。${summary}${summary ? ', ' : ''}${status}`
  const rowsHtml = renderRows(notifyRows(fields, summary))
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="utf-8" /><meta name="viewport" content="width=device-width,initial-scale=1" /></head>
<body style="margin:0;padding:0;background:#eef2f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;">
  <table role="presentation" cellpadding="0" cellspacing="0" width="100%" style="background:#eef2f7;padding:24px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" cellpadding="0" cellspacing="0" width="640" style="max-width:640px;width:100%;background:#ffffff;border:1px solid #d9e2ec;border-radius:10px;overflow:hidden;">
          <tr>
            <td style="padding:18px 22px;background:#0f4c81;color:#ffffff;">
              <div style="font-size:12px;letter-spacing:0.04em;opacity:0.85;margin-bottom:6px;">${escapeHtml(brand)}</div>
              <div style="font-size:18px;font-weight:600;line-height:1.4;">${escapeHtml(greeting)}</div>
            </td>
          </tr>
          <tr>
            <td style="padding:0;">
              <table role="presentation" cellpadding="0" cellspacing="0" width="100%" style="border-collapse:collapse;">
                ${rowsHtml}
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:14px 22px;background:#f8fafc;color:#7b8794;font-size:12px;border-top:1px solid #e6ebf2;">
              本邮件由自动打包系统发送，请相关责任人尽快处理。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`
}

export function buildFtpNotifyEmailSubject(
  fields: Pick<FtpNotifyEmailFields, 'projectLabel' | 'updateContent'>,
  template: NotifyMailTemplate = 'ftp_update',
): string {
  const summary = buildFtpNotifySummary(fields)
  const { status } = notifyTemplateMeta(template)
  return summary ? `${summary}, ${status}` : status
}

export function warPackageDisplayName(fileName: string): string {
  return fileName.trim().replace(/\\/g, '/').split('/').pop() ?? ''
}

export function inferUpdateScope(projectType: string): string {
  const t = projectType.trim().toLowerCase()
  if (t === 'npm' || t === 'vue' || t === 'nodejs' || t === 'frontend') return 'DATAHUB界面'
  if (t === 'maven' || t === 'gradle' || t === 'go' || t === 'java') return 'DATAHUB服务'
  return 'DATAHUB界面'
}

export function smtpFromNameFromConfig(config: Record<string, unknown> | undefined): string {
  if (!config) return ''
  return String(config.from_name ?? '').trim()
}

export type FtpNotifyPushSuccessLike = {
  projectName: string
  projectType: string
  fileName: string
  path: string
  pushPath: string
  taskRef: string
  backupPath: string
}

export function buildMergedFtpNotifyFields(
  items: FtpNotifyPushSuccessLike[],
  opts: { workspaceCode?: string; workspaceName?: string },
): FtpNotifyEmailFields | null {
  if (!items.length) return null
  const first = items[0]
  const warPackages = uniqueNonEmpty(items.map((it) => warPackageDisplayName(it.fileName || it.path)))
  const backups = uniqueNonEmpty(items.map((it) => it.backupPath.trim()))
  const taskRef = first.taskRef.trim() || items.map((it) => it.taskRef.trim()).find(Boolean) || ''
  const scopes = uniqueNonEmpty(items.map((it) => inferUpdateScope(it.projectType)))
  const ftpSource = normalizeRemoteDir(resolvePushLayout(first.pushPath, first.fileName || 'package').pushDir)
  return {
    projectLabel: resolveNotifyProjectLabel(opts.workspaceCode ?? '', opts.workspaceName ?? '', first.projectName),
    updateContent: formatNotifyUpdateContent(taskRef),
    taskNo: '',
    ftpVersionSource: ftpSource,
    updateMethod: FTP_NOTIFY_UPDATE_METHOD,
    needRestart: '是',
    browserCacheRefresh: '是',
    impactScope: '全局',
    updateScope: scopes.length > 1 ? scopes.join('、') : scopes[0] || 'DATAHUB界面',
    affectsApi: '是',
    warPackage: warPackages.join('、'),
    other: '',
    backup: backups.join('；'),
  }
}

function uniqueNonEmpty(values: string[]): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const raw of values) {
    const v = raw.trim()
    if (!v) continue
    const key = v.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(v)
  }
  return out
}
