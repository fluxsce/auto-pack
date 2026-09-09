# Build frontend dist. Skips npm ci when node_modules is present (avoids EPERM on locked native bindings).
function Build-Frontend {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FrontendDir
    )

    $ErrorActionPreference = "Stop"
    Push-Location $FrontendDir
    try {
        $viteJs = Join-Path $FrontendDir "node_modules\vite\bin\vite.js"
        $rolldownBinding = Join-Path $FrontendDir "node_modules\@rolldown\binding-win32-x64-msvc"
        $needCi = $false
        if (-not (Test-Path $viteJs)) {
            Write-Host "    npm ci (vite missing)"
            $needCi = $true
        } elseif (-not (Test-Path $rolldownBinding)) {
            Write-Host "    npm ci (windows rolldown binding missing — reinstall for this OS)"
            $needCi = $true
        }
        if ($needCi) {
            npm ci
            if ($LASTEXITCODE -ne 0) {
                throw "npm ci failed with exit code $LASTEXITCODE"
            }
        } else {
            Write-Host "    skip npm ci (deps OK on this machine)"
        }

        Write-Host "    vite build"
        & node $viteJs build
        if ($LASTEXITCODE -ne 0) {
            throw "vite build failed with exit code $LASTEXITCODE"
        }

        $distDir = Join-Path $FrontendDir "dist"
        $jsFiles = @(Get-ChildItem -Path $distDir -Recurse -Filter "*.js" -File -ErrorAction SilentlyContinue)
        if ($jsFiles.Count -eq 0) {
            throw "frontend dist has no JS under $distDir"
        }
        Write-Host "    dist OK ($($jsFiles.Count) JS file(s))"
    } finally {
        Pop-Location
    }
}
