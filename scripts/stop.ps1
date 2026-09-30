$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$runtimeDirectory = Join-Path $projectRoot '.runtime'
foreach ($serviceName in @('api', 'user', 'video', 'favorite', 'comment', 'relation', 'message')) {
    $pidPath = Join-Path $runtimeDirectory "${serviceName}.pid"
    if (-not (Test-Path -LiteralPath $pidPath)) { continue }
    $processId = [int](Get-Content -LiteralPath $pidPath)
    $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
    $binaryPath = Join-Path $projectRoot "bin/${serviceName}.exe"
    if ($process -and $process.Path -eq $binaryPath) { Stop-Process -Id $processId }
    Remove-Item -LiteralPath $pidPath
}
