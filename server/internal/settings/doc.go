// Package settings 是平台环境设置：系统属性与全局变量。
//
// SDK / 凭据仍走 internal/env（按工作空间）。本包只存进程级开关与 KEY=VALUE。
// HTTP 走 web/settings。作业执行面可读 Snapshot / ProcEnv，不查用户表。
package settings
