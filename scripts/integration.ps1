$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
try {
    & docker compose up '-d' '--wait'
    if ($LASTEXITCODE -ne 0) { throw 'Integration dependencies failed to start' }
    & docker compose exec '-T' 'mysql' 'mysql' '-uroot' '-plocal-root-only' '-e' "CREATE DATABASE IF NOT EXISTS db_kratos_integration; GRANT ALL PRIVILEGES ON db_kratos_integration.* TO 'tiktokDB'@'%';"
    if ($LASTEXITCODE -ne 0) { throw 'Cannot prepare isolated integration database' }
    New-Item -ItemType Directory -Path "${projectRoot}/.runtime" -Force | Out-Null
    $originalGoOS, $originalGoArch, $originalCgo = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
    try {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', '0'
        & go test '-c' '-tags=integration' '-o' '.runtime/integration.test' './internal/integration'
        if ($LASTEXITCODE -ne 0) { throw 'Integration test build failed' }
        & go test '-c' '-tags=integration' '-o' '.runtime/data-integration.test' './internal/data'
        if ($LASTEXITCODE -ne 0) { throw 'Data integration test build failed' }
    } finally { $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $originalGoOS, $originalGoArch, $originalCgo }
    & docker build '-f' 'dockerfiles/integration.Dockerfile' '-t' 'tiktok-kratos-integration:local' '.'
    if ($LASTEXITCODE -ne 0) { throw 'Integration runtime build failed' }
    & docker run '--rm' '--network' 'tiktok-kratos-dev_default' '-v' "${projectRoot}:/workspace" '-e' 'TIKTOK_PROJECT_ROOT=/workspace' '-e' 'TIKTOK_TEST_DSN=tiktokDB:tiktokDB@tcp(mysql:3306)/db_kratos_integration?charset=utf8mb4&parseTime=True&loc=UTC' 'tiktok-kratos-integration:local' '-test.v' '-test.timeout=120s'
    if ($LASTEXITCODE -ne 0) { throw 'Integration acceptance failed' }
    & docker run '--rm' '--network' 'tiktok-kratos-dev_default' '-v' "${projectRoot}:/workspace" '--entrypoint' '/workspace/.runtime/data-integration.test' '-e' 'TIKTOK_TEST_DSN=tiktokDB:tiktokDB@tcp(mysql:3306)/db_kratos_integration?charset=utf8mb4&parseTime=True&loc=UTC' 'tiktok-kratos-integration:local' '-test.v' '-test.timeout=120s'
    if ($LASTEXITCODE -ne 0) { throw 'Data consistency acceptance failed' }
} finally { Pop-Location }
