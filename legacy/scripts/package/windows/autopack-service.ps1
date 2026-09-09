# Start / stop / restart autopack in the background (Windows).
param(
    [Parameter(Position = 0)]
    [ValidateSet('start', 'stop', 'restart', 'status')]
    [string]$Action = 'start'
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$PidFile = Join-Path $Root 'data\autopack.pid'
$LogDir = Join-Path $Root 'logs'
$LogFile = Join-Path $LogDir 'autopack-stdout.log'
$Exe = Join-Path $Root 'autopack.exe'

function Get-StoredPid {
    if (-not (Test-Path $PidFile)) { return $null }
    $raw = (Get-Content -Path $PidFile -Raw -ErrorAction SilentlyContinue).Trim()
    if ($raw -match '^\d+$') { return [int]$raw }
    return $null
}

function Test-IsRunning {
    $pid = Get-StoredPid
    if (-not $pid) { return $false }
    return $null -ne (Get-Process -Id $pid -ErrorAction SilentlyContinue)
}

function Start-Autopack {
    if (Test-IsRunning) {
        Write-Host "autopack already running (pid $(Get-StoredPid))"
        return
    }
    if (-not (Test-Path $Exe)) {
        throw "autopack.exe not found: $Exe"
    }
    New-Item -ItemType Directory -Force -Path $LogDir, (Split-Path $PidFile) | Out-Null
    $proc = Start-Process `
        -FilePath $Exe `
        -WorkingDirectory $Root `
        -WindowStyle Hidden `
        -PassThru
    Set-Content -Path $PidFile -Value $proc.Id -Encoding ascii -NoNewline
    Write-Host "autopack started (pid $($proc.Id), log $LogFile)"
    Write-Host "application logs also under configs/app.yaml log.file (default ./logs/autopack.log)"
}

function Stop-Autopack {
    if (-not (Test-IsRunning)) {
        Remove-Item -Path $PidFile -Force -ErrorAction SilentlyContinue
        Write-Host 'autopack is not running'
        return
    }
    $pid = Get-StoredPid
    Write-Host "stopping autopack (pid $pid)..."
    Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
    $deadline = (Get-Date).AddSeconds(15)
    while ((Get-Date) -lt $deadline) {
        if (-not (Get-Process -Id $pid -ErrorAction SilentlyContinue)) { break }
        Start-Sleep -Milliseconds 500
    }
    if (Get-Process -Id $pid -ErrorAction SilentlyContinue) {
        Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -Path $PidFile -Force -ErrorAction SilentlyContinue
    Write-Host 'autopack stopped'
}

function Show-Status {
    if (Test-IsRunning) {
        Write-Host "autopack running (pid $(Get-StoredPid))"
    } else {
        Write-Host 'autopack stopped'
        exit 1
    }
}

switch ($Action) {
    'start' { Start-Autopack }
    'stop' { Stop-Autopack }
    'restart' { Stop-Autopack; Start-Sleep -Seconds 1; Start-Autopack }
    'status' { Show-Status }
}
