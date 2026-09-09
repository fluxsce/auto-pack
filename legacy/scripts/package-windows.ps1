# Full release build for Windows (amd64). Requires Go and Node.js (pure Go SQLite, no CGO).
$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$AppName = "autopack"
$GoOs = "windows"
$GoArch = "amd64"
$PkgId = "$AppName-$GoOs-$GoArch"
$Staging = Join-Path $Root "build\$PkgId"
$Archive = Join-Path $Root "build\$PkgId.zip"
$WebDir = Join-Path $Staging "web"
$FrontendDir = Join-Path $Root "frontend"

Write-Host "==> [1/4] Build frontend"
. (Join-Path $PSScriptRoot "lib\package-frontend.ps1")
Build-Frontend -FrontendDir $FrontendDir

Write-Host "==> [2/4] Build Go binary ($GoOs/$GoArch)"
if (Test-Path $Staging) { Remove-Item -Recurse -Force $Staging }
New-Item -ItemType Directory -Force -Path (Join-Path $Staging "scripts\sqlite") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Staging "data") | Out-Null
Push-Location (Join-Path $Root "backend")
$env:GOOS = $GoOs
$env:GOARCH = $GoArch
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o (Join-Path $Staging "$AppName.exe") ./cmd
if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
Pop-Location

Write-Host "==> [3/4] Stage runtime files (web static dir + scripts)"
New-Item -ItemType Directory -Force -Path $WebDir | Out-Null
Copy-Item -Path (Join-Path $Root "frontend\dist\*") -Destination $WebDir -Recurse -Force
$bundledJs = Get-ChildItem -Path $WebDir -Recurse -Filter "*.js" -File
if (-not $bundledJs) {
    throw "No JS files under $WebDir — frontend dist copy failed."
}
Write-Host "    Static files: $WebDir ($($bundledJs.Count) JS file(s), served by autopack.exe)"

Copy-Item (Join-Path $Root "backend\configs") (Join-Path $Staging "configs") -Recurse -Force
Copy-Item (Join-Path $Root "scripts\sqlite\*.sql") (Join-Path $Staging "scripts\sqlite\") -Force
$winScripts = Join-Path $Root "scripts\package\windows"
Copy-Item (Join-Path $winScripts "*") $Staging -Force
Copy-Item (Join-Path $Root "scripts\package\README.txt") (Join-Path $Staging "README.txt") -Force

Write-Host "==> [4/4] Compress to $Archive"
if (Test-Path $Archive) { Remove-Item -Force $Archive }
Compress-Archive -Path $Staging -DestinationPath $Archive -Force

Write-Host "Done."
Write-Host "  Directory: $Staging"
Write-Host "  Archive:   $Archive"
Write-Host "  Run:       cd $Staging; .\start.bat"
