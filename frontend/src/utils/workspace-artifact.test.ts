import { describe, expect, it } from 'vitest'
import {
  isVdatahubStaticDirName,
  isVdatahubStaticPath,
  pickTargetArtifact,
  VDATAHUB_STATIC_ROOT,
  vdatahubZipFileName,
} from './workspace-artifact'
import type { ProjectWorkspaceEntry } from '@/types'

function entry(name: string, isDir = false): ProjectWorkspaceEntry {
  return { name, path: `target/${name}`, is_dir: isDir, size: isDir ? 0 : 10 }
}

describe('isVdatahubStaticDirName', () => {
  it('matches sce-vdatahub and prefixed variants', () => {
    expect(isVdatahubStaticDirName('sce-vdatahub')).toBe(true)
    expect(isVdatahubStaticDirName('sce-vdatahub-web')).toBe(true)
    expect(isVdatahubStaticDirName('SCE-VDATAHUB_admin')).toBe(true)
    expect(isVdatahubStaticDirName('sce-vdatahub.zip')).toBe(true)
    expect(isVdatahubStaticDirName('sce-vcom')).toBe(false)
    expect(isVdatahubStaticDirName('vdatahub')).toBe(false)
  })
})

describe('isVdatahubStaticPath', () => {
  it('uses the last path segment', () => {
    expect(isVdatahubStaticPath('target/sce-vdatahub')).toBe(true)
    expect(isVdatahubStaticPath('mod-a/target/sce-vdatahub-web')).toBe(true)
    expect(isVdatahubStaticPath('target/app.war')).toBe(false)
  })
})

describe('pickTargetArtifact', () => {
  it('prefers war over jar and static dirs', () => {
    const got = pickTargetArtifact([
      entry('sce-vdatahub', true),
      entry('app.jar'),
      entry('app.war'),
    ])
    expect(got?.name).toBe('app.war')
  })

  it('picks exact sce-vdatahub when no war/jar', () => {
    const got = pickTargetArtifact([entry('sce-vdatahub-web', true), entry('sce-vdatahub', true)])
    expect(got?.name).toBe(VDATAHUB_STATIC_ROOT)
  })

  it('returns null when nothing matches', () => {
    expect(pickTargetArtifact([entry('readme.txt'), entry('classes', true)])).toBeNull()
  })
})

describe('vdatahubZipFileName', () => {
  it('is sce-vdatahub.zip', () => {
    expect(vdatahubZipFileName()).toBe('sce-vdatahub.zip')
  })
})
