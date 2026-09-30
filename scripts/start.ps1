param([string]$Config = 'config/kratos.yml')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$runtimeDirectory = Join-Path $projectRoot '.runtime'
New-Item -ItemType Directory -Path $runtimeDirectory -Force | Out-Null
$configPath = [IO.Path]::GetFullPath((Join-Path $projectRoot $Config))
foreach ($serviceName in @('user', 'video', 'favorite', 'comment', 'relation', 'message', 'api')) {
    $binaryPath = Join-Path $projectRoot "bin/${serviceName}.exe"
    if (-not (Test-Path -LiteralPath $binaryPath)) { throw "Missing $binaryPath; run scripts/build.ps1 first" }
    $pidPath = Join-Path $runtimeDirectory "${serviceName}.pid"
    if (Test-Path -LiteralPath $pidPath) {
        $processId = [int](Get-Content -LiteralPath $pidPath)
        $existingProcess = Get-Process -Id $processId -ErrorAction SilentlyContinue
        if ($existingProcess -and $existingProcess.Path -eq $binaryPath) { throw "$serviceName is already running" }
    }
    $process = Start-Process -FilePath $binaryPath -ArgumentList @('-config', "`"$configPath`"") -WorkingDirectory $projectRoot -WindowStyle Hidden -RedirectStandardOutput "${runtimeDirectory}/${serviceName}.log" -RedirectStandardError "${runtimeDirectory}/${serviceName}.error.log" -PassThru
    Set-Content -LiteralPath $pidPath -Value $process.Id -Encoding utf8
}
