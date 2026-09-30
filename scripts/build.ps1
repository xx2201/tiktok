$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
    New-Item -ItemType Directory -Path "${projectRoot}/bin" -Force | Out-Null
    foreach ($serviceName in @('api', 'user', 'video', 'favorite', 'comment', 'relation', 'message')) {
        & go build '-trimpath' '-o' "${projectRoot}/bin/${serviceName}.exe" "./cmd/${serviceName}"
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $serviceName" }
    }
} finally { Pop-Location }
