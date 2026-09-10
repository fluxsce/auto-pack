#Requires -Version 5.1
<#
.SYNOPSIS
  全局构建入口（Windows）：前端 + 后端 + 按平台组装发行目录。
.DESCRIPTION
  产物：build/dist/auto-pack-{version}-{os}-{arch}/
  以及同名 zip。不打包源码、node_modules、本机 app.yaml。
#>
param(
    [ValidateSet('windows', 'linux')]
    [string]$OS = 'windows',

    [ValidateSet('amd64', 'arm64')]
    [string]$Arch = 'amd64',

    [switch]$SkipWeb,
    [switch]$SkipArchive
)

$ErrorActionPreference = 'Stop'
# Cursor / Windows Terminal 里 Copy-Item、Compress-Archive 的进度条会触发
# Write-Progress IndexOutOfRangeException；关掉进度并改用 robocopy / tar。
$ProgressPreference = 'SilentlyContinue'
$Utf8 = New-Object System.Text.UTF8Encoding $false

function Write-Step([string]$Message) {
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
}

function Read-AppVersion([string]$Repo) {
    $pkg = Join-Path $Repo 'package.json'
    $raw = [System.IO.File]::ReadAllText($pkg, $Utf8)
    if ($raw -match '"version"\s*:\s*"([^"]+)"') {
        return $Matches[1]
    }
    throw "version not found in $pkg"
}

function Sync-AppVersion([string]$Repo) {
    $script = Join-Path $Repo 'script\sync-version.mjs'
    $node = Get-Command node -ErrorAction SilentlyContinue
    if (-not $node) {
        Write-Host 'node not found; AppVersion will come from ldflags' -ForegroundColor Yellow
        return
    }
    Write-Step 'sync AppVersion from package.json'
    & node $script
    if ($LASTEXITCODE -ne 0) {
        throw "sync-version failed ($LASTEXITCODE)"
    }
}

function Invoke-Pnpm([string]$WorkDir, [string[]]$PnpmArgs) {
    Push-Location $WorkDir
    try {
        & pnpm @PnpmArgs
        if ($LASTEXITCODE -ne 0) {
            throw "pnpm $($PnpmArgs -join ' ') failed ($LASTEXITCODE) in $WorkDir"
        }
    } finally {
        Pop-Location
    }
}

function Ensure-WebDeps([string]$Repo) {
    $pnpm = Get-Command pnpm -ErrorAction SilentlyContinue
    if (-not $pnpm) {
        throw 'pnpm not found. Install Node >= 20 and pnpm 9.'
    }
    $vueTsc = Join-Path $Repo 'node_modules\.bin\vue-tsc'
    $vueTscCmd = Join-Path $Repo 'node_modules\.bin\vue-tsc.cmd'
    if (-not (Test-Path -LiteralPath (Join-Path $Repo 'node_modules') -PathType Container) -or
        (-not (Test-Path -LiteralPath $vueTsc) -and -not (Test-Path -LiteralPath $vueTscCmd))) {
        Write-Step 'pnpm install (node_modules missing)'
        $lock = Join-Path $Repo 'pnpm-lock.yaml'
        if (Test-Path -LiteralPath $lock -PathType Leaf) {
            Invoke-Pnpm $Repo @('install', '--frozen-lockfile')
        } else {
            Invoke-Pnpm $Repo @('install')
        }
    }
}

function Invoke-Frontend([string]$Repo) {
    Ensure-WebDeps $Repo
    if (-not $env:NODE_OPTIONS) {
        $env:NODE_OPTIONS = '--max-old-space-size=4096'
    }
    Write-Step 'pnpm build:web (web -> server/web)'
    Invoke-Pnpm $Repo @('build:web')
    $index = Join-Path $Repo 'server\web\index.html'
    if (-not (Test-Path -LiteralPath $index -PathType Leaf)) {
        throw "frontend missing: $index"
    }
}

function Invoke-Backend([string]$Repo, [string]$Version, [string]$Goos, [string]$Goarch, [string]$OutFile) {
    Write-Step "go build $Goos/$Goarch -> $OutFile"
    $dir = Split-Path -Parent $OutFile
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
    $ld = "-s -w -X auto-pack/server/internal/config.AppVersion=$Version"
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    $env:CGO_ENABLED = '0'
    try {
        & go build -C (Join-Path $Repo 'server') -trimpath -ldflags $ld -o $OutFile ./cmd
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed ($LASTEXITCODE)"
        }
    } finally {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    }
}

function Copy-PackagedExample([string]$Repo, [string]$Dst) {
    $src = Join-Path $Repo 'script\package\app.yaml.example'
    if (-not (Test-Path -LiteralPath $src -PathType Leaf)) {
        throw "packaged config template missing: $src"
    }
    $parent = Split-Path -Parent $Dst
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent | Out-Null
    }
    Copy-Item -LiteralPath $src -Destination $Dst -Force
}

function Copy-DbScripts([string]$Repo, [string]$DestDb) {
    foreach ($engine in @('sqlite', 'mysql', 'postgres')) {
        $src = Join-Path $Repo "script\db\$engine"
        $dst = Join-Path $DestDb $engine
        if (-not (Test-Path -LiteralPath $src -PathType Container)) {
            throw "DDL missing: $src"
        }
        New-Item -ItemType Directory -Force -Path $dst | Out-Null
        foreach ($name in @('init.sql', '98_patch.sql', '99_seed.sql')) {
            $file = Join-Path $src $name
            if (Test-Path -LiteralPath $file -PathType Leaf) {
                Copy-Item -LiteralPath $file -Destination (Join-Path $dst $name) -Force
            }
        }
        if (-not (Test-Path -LiteralPath (Join-Path $dst 'init.sql') -PathType Leaf)) {
            throw "init.sql missing for $engine"
        }
    }
}

function Copy-RunScripts([string]$Repo, [string]$Pkg, [string]$TargetOS) {
    $runDst = Join-Path $Pkg 'script\run'
    New-Item -ItemType Directory -Force -Path $runDst | Out-Null
    if ($TargetOS -eq 'windows') {
        Copy-Item -LiteralPath (Join-Path $Repo 'script\run\windows\service.ps1') -Destination (Join-Path $runDst 'service.ps1') -Force
        foreach ($name in @('start.bat', 'stop.bat', 'restart.bat')) {
            Copy-Item -LiteralPath (Join-Path $Repo "script\run\windows\$name") -Destination (Join-Path $Pkg $name) -Force
        }
        return
    }
    Copy-Item -LiteralPath (Join-Path $Repo 'script\run\linux\common.sh') -Destination (Join-Path $runDst 'common.sh') -Force
    foreach ($name in @('start.sh', 'stop.sh', 'restart.sh')) {
        Copy-Item -LiteralPath (Join-Path $Repo "script\run\linux\$name") -Destination (Join-Path $Pkg $name) -Force
    }
}

function Copy-Tree([string]$Src, [string]$Dst) {
    if (-not (Test-Path -LiteralPath $Src -PathType Container)) {
        throw "copy source missing: $Src"
    }
    New-Item -ItemType Directory -Force -Path $Dst | Out-Null
    $robocopy = Get-Command robocopy -ErrorAction SilentlyContinue
    if ($robocopy) {
        # 0-7 是成功（有文件拷贝 / 有额外文件等）；>=8 才是失败。
        & robocopy $Src $Dst /E /NFL /NDL /NJH /NJS /NC /NS /NP /R:1 /W:1 | Out-Null
        if ($LASTEXITCODE -ge 8) {
            throw "robocopy failed ($LASTEXITCODE): $Src -> $Dst"
        }
        return
    }
    Copy-Item -Path (Join-Path $Src '*') -Destination $Dst -Recurse -Force
}

function New-Archive([string]$DistDir, [string]$FolderName, [string]$ArchiveName, [string]$TargetOS) {
    $src = Join-Path $DistDir $FolderName
    $tar = Get-Command tar -ErrorAction SilentlyContinue
    if ($TargetOS -eq 'windows') {
        $zip = Join-Path $DistDir "$ArchiveName.zip"
        if (Test-Path -LiteralPath $zip) {
            Remove-Item -LiteralPath $zip -Force
        }
        if ($tar) {
            Push-Location $DistDir
            try {
                & tar -a -cf $zip $FolderName
                if ($LASTEXITCODE -ne 0) {
                    throw "tar zip failed ($LASTEXITCODE)"
                }
            } finally {
                Pop-Location
            }
        } else {
            Compress-Archive -Path $src -DestinationPath $zip -Force
        }
        Write-Host "archive $zip"
        return
    }
    $tgz = Join-Path $DistDir "$ArchiveName.tar.gz"
    if (Test-Path -LiteralPath $tgz) {
        Remove-Item -LiteralPath $tgz -Force
    }
    if (-not $tar) {
        Write-Warning "tar not found; skip $tgz (directory is ready)"
        return
    }
    Push-Location $DistDir
    try {
        & tar -czf $tgz $FolderName
        if ($LASTEXITCODE -ne 0) {
            throw "tar failed ($LASTEXITCODE)"
        }
    } finally {
        Pop-Location
    }
    Write-Host "archive $tgz"
}

$Repo = Get-RepoRoot
$Version = Read-AppVersion $Repo
Sync-AppVersion $Repo
# 压缩包文件名带版本；解压后目录固定 auto-pack，版本只写在 VERSION 文件里。
$StageName = 'auto-pack'
$ArchiveName = "auto-pack-$Version-$OS-$Arch"
$DistDir = Join-Path $Repo 'build\dist'
$Pkg = Join-Path $DistDir $StageName

Write-Step "package $ArchiveName"
if (Test-Path -LiteralPath $Pkg) {
    Remove-Item -LiteralPath $Pkg -Recurse -Force
}

if (-not $SkipWeb) {
    Invoke-Frontend $Repo
} else {
    $index = Join-Path $Repo 'server\web\index.html'
    if (-not (Test-Path -LiteralPath $index -PathType Leaf)) {
        throw "SkipWeb set but $index missing"
    }
}

$binName = if ($OS -eq 'windows') { 'autopack.exe' } else { 'autopack' }
$binOut = Join-Path $Pkg "bin\$binName"
Invoke-Backend $Repo $Version $OS $Arch $binOut

Write-Step "stage $Pkg"
$webSrc = Join-Path $Repo 'server\web'
$webDst = Join-Path $Pkg 'web'
Copy-Tree $webSrc $webDst
Get-ChildItem -LiteralPath $webDst -Recurse -Filter '*.map' -File -ErrorAction SilentlyContinue | ForEach-Object {
    [System.IO.File]::Delete($_.FullName)
}

Copy-PackagedExample $Repo (Join-Path $Pkg 'configs\app.yaml.example')
Copy-DbScripts $Repo (Join-Path $Pkg 'script\db')
Copy-RunScripts $Repo $Pkg $OS
Copy-Item -LiteralPath (Join-Path $Repo 'script\package\README.txt') -Destination (Join-Path $Pkg 'README.txt') -Force
[System.IO.File]::WriteAllText((Join-Path $Pkg 'VERSION'), "$Version`n", $Utf8)

if (-not $SkipArchive) {
    New-Archive $DistDir $StageName $ArchiveName $OS
}

Write-Host "ready $Pkg"
Write-Host "start: $(Join-Path $Pkg $(if ($OS -eq 'windows') { 'start.bat' } else { 'start.sh' }))"
