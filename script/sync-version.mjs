import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const pkg = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'))
const version = String(pkg.version ?? '').trim()
if (!/^\d+\.\d+\.\d+/.test(version)) {
  throw new Error(`invalid package.json version: ${JSON.stringify(pkg.version)}`)
}

const dest = join(root, 'server/internal/config/version.go')
const body = `package config

// AppVersion 是控制台 SemVer，由仓库根 package.json 生成，不要手改。
// 执行 node script/sync-version.mjs 可重写本文件。
var AppVersion = "${version}"
`
writeFileSync(dest, body, 'utf8')
process.stdout.write(`AppVersion ${version}\n`)
