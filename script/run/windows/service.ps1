#Requires -Version 5.1
<#
.SYNOPSIS
  发行包内进程启停（由根目录 start/stop/restart.bat 调用）。
#>
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('start', 'stop', 'restart')]
    [string]$Action,

    [Parameter(Mandatory = $true)]
    [string]$Home
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)

function Resolve-Home([string]$Path) {
    if (-not $Path) {
        throw 'Home is required'
    }
    $full = [System.IO.Path]::GetFullPath($Path.TrimEnd('\', '/'))
    if (-not (Test-Path -LiteralPath $full -PathType Container)) {
        throw "app home not found: $full"
    }
    return $full
}

function Get-PidPath([string]$Root) {
    return (Join-Path $Root 'run\autopack.pid')
}

function Get-ExePath([string]$Root) {
    $exe = Join-Path $Root 'bin\autopack.exe'
    if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
        $exe = Join-Path $Root 'bin\autopack'
    }
    if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) {
        throw "binary not found: $Root\bin\autopack.exe"
    }
    return $exe
}

function Test-PidRunning([int]$ProcId) {
    try {
        $p = Get-Process -Id $ProcId -ErrorAction Stop
        return $null -ne $p
    } catch {
        return $false
    }
}

function Get-RunningPid([string]$Root) {
    $pidFile = Get-PidPath $Root
    if (-not (Test-Path -LiteralPath $pidFile -PathType Leaf)) {
        return 0
    }
    $raw = (Get-Content -LiteralPath $pidFile -TotalCount 1 -ErrorAction SilentlyContinue)
    if (-not $raw) {
        return 0
    }
    $procId = 0
    if (-not [int]::TryParse($raw.Trim(), [ref]$procId) -or $procId -le 0) {
        return 0
    }
    if (Test-PidRunning $procId) {
        return $procId
    }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
    return 0
}

function Initialize-Runtime([string]$Root) {
    foreach ($name in @('data', 'logs', 'run')) {
        $dir = Join-Path $Root $name
        if (-not (Test-Path -LiteralPath $dir)) {
            New-Item -ItemType Directory -Path $dir | Out-Null
        }
    }
    $cfg = Join-Path $Root 'configs\app.yaml'
    $example = Join-Path $Root 'configs\app.yaml.example'
    if (-not (Test-Path -LiteralPath $cfg -PathType Leaf)) {
        if (-not (Test-Path -LiteralPath $example -PathType Leaf)) {
            throw "missing $cfg and $example"
        }
        Copy-Item -LiteralPath $example -Destination $cfg
        Write-Host "created configs\app.yaml from example; change secrets before production"
    }
}

function Start-App([string]$Root) {
    $alive = Get-RunningPid $Root
    if ($alive -gt 0) {
        Write-Host "already running (pid $alive)"
        return
    }
    Initialize-Runtime $Root
    $exe = Get-ExePath $Root
    $cfgDir = Join-Path $Root 'configs'
    $stdout = Join-Path $Root 'logs\stdout.log'
    $stderr = Join-Path $Root 'logs\stderr.log'
    $proc = Start-Process -FilePath $exe -ArgumentList @('-config', $cfgDir) -WorkingDirectory $Root -RedirectStandardOutput $stdout -RedirectStandardError $stderr -WindowStyle Hidden -PassThru
    if (-not $proc) {
        throw 'failed to start autopack'
    }
    $pidFile = Get-PidPath $Root
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($pidFile, [string]$proc.Id, $utf8)
    Write-Host "started pid $($proc.Id)"
    Write-Host "console http://127.0.0.1:8080/autopack/"
}

function Stop-App([string]$Root) {
    $alive = Get-RunningPid $Root
    if ($alive -le 0) {
        Write-Host 'not running'
        return
    }
    Write-Host "stopping pid $alive"
    try {
        Stop-Process -Id $alive -ErrorAction Stop
    } catch {
        Write-Host "stop signal failed: $($_.Exception.Message)"
    }
    $deadline = (Get-Date).AddSeconds(12)
    while ((Get-Date) -lt $deadline) {
        if (-not (Test-PidRunning $alive)) {
            break
        }
        Start-Sleep -Milliseconds 200
    }
    if (Test-PidRunning $alive) {
        Stop-Process -Id $alive -Force -ErrorAction SilentlyContinue
    }
    $pidFile = Get-PidPath $Root
    if (Test-Path -LiteralPath $pidFile) {
        Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
    }
    Write-Host 'stopped'
}

$appHome = Resolve-Home $Home
switch ($Action) {
    'start' { Start-App $appHome }
    'stop' { Stop-App $appHome }
    'restart' {
        Stop-App $appHome
        Start-Sleep -Milliseconds 400
        Start-App $appHome
    }
}
