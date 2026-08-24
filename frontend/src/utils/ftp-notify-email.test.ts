import { describe, expect, it } from 'vitest'
import {
  buildFtpNotifyEmailBody,
  buildFtpNotifyEmailHTML,
  buildFtpNotifyEmailSubject,
  buildMergedFtpNotifyFields,
  dayGreeting,
  escapeHtml,
  formatNotifyUpdateContent,
  FTP_NOTIFY_UPDATE_METHOD,
  normalizeNotifyRecipient,
  normalizeNotifyRecipients,
  resolveNotifyProjectLabel,
  warPackageDisplayName,
} from './ftp-notify-email'

describe('ftp-notify-email', () => {
  it('builds ops notify body from template fields', () => {
    const body = buildFtpNotifyEmailBody(
      {
        projectLabel: 'NIKECLC',
        updateContent: '申请更新20260617145048000708, 20260622110412000341',
        taskNo: '20260630160644000990',
        ftpVersionSource: '/开发/耐克/WMS-V10.0.0-p4/sce-vcom-test/common',
        updateMethod: FTP_NOTIFY_UPDATE_METHOD,
        needRestart: '是',
        browserCacheRefresh: '是',
        impactScope: '全局',
        updateScope: 'DATAHUB界面',
        affectsApi: '是',
        warPackage: 'sce-vcom-login',
        other: '',
        backup: '/开发/耐克/WMS-V10.0.0-p4/sce-vcom-test/common/bak',
      },
      new Date('2026-06-30T10:00:00'),
    )
    expect(body).toContain('上午好。【NIKECLC】申请更新20260617145048000708, 20260622110412000341, FTP已更新')
    expect(body).toContain('更新方式：整包更新')
    expect(body).toContain('更新范围：DATAHUB界面')
    expect(body).toContain('是否影响接口运行：是')
    expect(body).toContain('是否重启：是')
    expect(body).toContain('备份：/开发/耐克/WMS-V10.0.0-p4/sce-vcom-test/common/bak')
    expect(body).not.toContain('前端or后端')
    expect(body).not.toContain('资源来源：')
    expect(body).not.toContain('其他：')

    const html = buildFtpNotifyEmailHTML(
      {
        projectLabel: 'NIKECLC',
        updateContent: '申请更新1',
        taskNo: 'T1',
        ftpVersionSource: '/a',
        updateMethod: FTP_NOTIFY_UPDATE_METHOD,
        needRestart: '是',
        browserCacheRefresh: '是',
        impactScope: '全局',
        updateScope: 'DATAHUB服务',
        affectsApi: '是',
        warPackage: 'app',
        other: '',
        backup: '/a/bak',
      },
      new Date('2026-06-30T10:00:00'),
    )
    expect(html).toContain('<!DOCTYPE html>')
    expect(html).toContain('DATAHUB 更新通知')
    expect(html).toContain(escapeHtml('【NIKECLC】申请更新1'))
    expect(html).toContain('更新war包')
    expect(html).toContain('是否影响接口运行')
    expect(html).toContain('#e8f1fa')
    expect(html).toContain('#0f4c81')
    expect(html).toContain('width:128px')
    expect(html).not.toContain('资源来源')
    expect(html).not.toContain('FTP 更新告知')
  })

  it('auto-completes incomplete recipients with @flux.com.cn', () => {
    expect(normalizeNotifyRecipient('zhangsan')).toBe('zhangsan@flux.com.cn')
    expect(normalizeNotifyRecipient('lisi@')).toBe('lisi@flux.com.cn')
    expect(normalizeNotifyRecipient('wangwu@example.com')).toBe('wangwu@example.com')
    expect(normalizeNotifyRecipients('a; b@corp.com, c@')).toBe(
      'a@flux.com.cn, b@corp.com, c@flux.com.cn',
    )
  })

  it('auto-formats update content and skips generic workspace labels', () => {
    expect(formatNotifyUpdateContent('20260617145048000708')).toBe('申请更新20260617145048000708')
    expect(formatNotifyUpdateContent('申请更新已有')).toBe('申请更新已有')
    expect(resolveNotifyProjectLabel('default', 'default', 'flux-datahub')).toBe('flux-datahub')
    expect(resolveNotifyProjectLabel('NIKECLC', 'Nike', 'app')).toBe('NIKECLC')
    expect(warPackageDisplayName('sce-vcom-login.war')).toBe('sce-vcom-login')
    expect(buildFtpNotifyEmailSubject({ projectLabel: 'A', updateContent: '申请更新1' })).toBe(
      '【A】申请更新1, FTP已更新',
    )
    expect(dayGreeting(new Date('2026-06-30T15:00:00'))).toBe('下午好')
  })

  it('merges multi-package push into one notify payload', () => {
    const fields = buildMergedFtpNotifyFields(
      [
        {
          projectName: 'cache',
          projectVersion: '1.0.0',
          projectType: 'maven',
          fileName: 'a.jar',
          path: 'target/a.jar',
          pushPath: '/war/test',
          taskRef: '20260727001',
          backupPath: '/war/test/BAK/1/a.jar',
        },
        {
          projectName: 'api',
          projectVersion: '1.0.0',
          projectType: 'maven',
          fileName: 'b.war',
          path: 'target/b.war',
          pushPath: '/war/test',
          taskRef: '20260727001',
          backupPath: '/war/test/BAK/1/b.war',
        },
      ],
      { workspaceCode: 'DATAHUB', workspaceName: 'datahub' },
    )
    expect(fields?.warPackage).toBe('a、b')
    expect(fields?.updateContent).toBe('申请更新20260727001')
    expect(fields?.updateMethod).toBe('整包更新')
    expect(fields?.updateScope).toBe('DATAHUB服务')
    expect(fields?.affectsApi).toBe('是')
    expect(fields?.backup).toContain('a.jar')
    expect(fields?.backup).toContain('b.war')
  })
})
