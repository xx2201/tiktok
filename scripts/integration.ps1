param([string]$Docker = 'docker')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$originalPath = $env:PATH
Push-Location -LiteralPath $projectRoot
try {
    # PATH 中可能存在多个 Docker 路径，只使用优先匹配的一个。
    $Docker = (Get-Command -Name $Docker -CommandType Application -TotalCount 1 -ErrorAction Stop).Source
    # Docker 凭据助手通过 PATH 查找；只为本次脚本加入所选 CLI 的目录。
    $env:PATH = (Split-Path -Parent $Docker) + [System.IO.Path]::PathSeparator + $originalPath
    & $Docker compose up '-d' '--wait'
    if ($LASTEXITCODE -ne 0) { throw 'Integration dependencies failed to start' }
    & $Docker compose exec '-T' 'mysql' 'mysql' '-uroot' '-plocal-root-only' '-e' "CREATE DATABASE IF NOT EXISTS db_kratos_integration; CREATE DATABASE IF NOT EXISTS db_kratos_replica_integration; GRANT ALL PRIVILEGES ON db_kratos_integration.* TO 'tiktokDB'@'%'; GRANT ALL PRIVILEGES ON db_kratos_replica_integration.* TO 'tiktokDB'@'%';"
    if ($LASTEXITCODE -ne 0) { throw 'Cannot prepare isolated integration database' }
    $vhosts = & $Docker compose exec '-T' 'rabbitmq' 'rabbitmqctl' 'list_vhosts' '-q' 'name'
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect integration message namespace' }
    if ('kratos-integration' -notin $vhosts) {
        & $Docker compose exec '-T' 'rabbitmq' 'rabbitmqctl' 'add_vhost' 'kratos-integration'
        if ($LASTEXITCODE -ne 0) { throw 'Cannot create integration message namespace' }
    }
    & $Docker compose exec '-T' 'rabbitmq' 'rabbitmqctl' 'set_permissions' '-p' 'kratos-integration' 'tiktokRMQ' '.*' '.*' '.*'
    if ($LASTEXITCODE -ne 0) { throw 'Cannot authorize integration message namespace' }
    New-Item -ItemType Directory -Path "${projectRoot}/.runtime" -Force | Out-Null
    $originalGoOS, $originalGoArch, $originalCgo = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
    try {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', '0'
        & go test '-c' '-tags=integration' '-o' '.runtime/integration.test' './internal/integration'
        if ($LASTEXITCODE -ne 0) { throw 'Integration test build failed' }
        & go test '-c' '-tags=integration' '-o' '.runtime/data-integration.test' './internal/data'
        if ($LASTEXITCODE -ne 0) { throw 'Data integration test build failed' }
    } finally { $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $originalGoOS, $originalGoArch, $originalCgo }
    & $Docker build '-f' 'dockerfiles/integration.Dockerfile' '-t' 'tiktok-kratos-integration:local' '.'
    if ($LASTEXITCODE -ne 0) { throw 'Integration runtime build failed' }
    & $Docker run '--rm' '--network' 'tiktok-kratos-dev_default' '-v' "${projectRoot}:/workspace" '-e' 'TIKTOK_PROJECT_ROOT=/workspace' '-e' 'TIKTOK_TEST_DSN=tiktokDB:tiktokDB@tcp(mysql:3306)/db_kratos_integration?charset=utf8mb4&parseTime=True&loc=UTC' 'tiktok-kratos-integration:local' '-test.v' '-test.timeout=120s'
    if ($LASTEXITCODE -ne 0) { throw 'Integration acceptance failed' }
    & $Docker run '--rm' '--network' 'tiktok-kratos-dev_default' '-v' "${projectRoot}:/workspace" '--entrypoint' '/workspace/.runtime/data-integration.test' '-e' 'TIKTOK_TEST_DSN=tiktokDB:tiktokDB@tcp(mysql:3306)/db_kratos_integration?charset=utf8mb4&parseTime=True&loc=UTC' '-e' 'TIKTOK_TEST_REPLICA_DSN=tiktokDB:tiktokDB@tcp(mysql:3306)/db_kratos_replica_integration?charset=utf8mb4&parseTime=True&loc=UTC' 'tiktok-kratos-integration:local' '-test.v' '-test.timeout=120s'
    if ($LASTEXITCODE -ne 0) { throw 'Data consistency acceptance failed' }
} finally {
    $env:PATH = $originalPath
    Pop-Location
}
