$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$toolsDirectory = Join-Path $projectRoot '.tools'
New-Item -ItemType Directory -Path $toolsDirectory -Force | Out-Null
$originalGoBin = $env:GOBIN
try {
    $env:GOBIN = $toolsDirectory
    & go install 'google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11'
    if ($LASTEXITCODE -ne 0) { throw 'Installing protoc-gen-go failed' }
    & go install 'google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1'
    if ($LASTEXITCODE -ne 0) { throw 'Installing protoc-gen-go-grpc failed' }
} finally {
    $env:GOBIN = $originalGoBin
}
Push-Location -LiteralPath $projectRoot
try {
    $protoFiles = Get-ChildItem -LiteralPath "${projectRoot}/api" -Filter '*.proto' -Recurse
    foreach ($protoFile in $protoFiles) {
        & protoc "--proto_path=${projectRoot}" "--plugin=protoc-gen-go=${toolsDirectory}/protoc-gen-go.exe" "--plugin=protoc-gen-go-grpc=${toolsDirectory}/protoc-gen-go-grpc.exe" '--go_out=.' '--go_opt=paths=source_relative' '--go-grpc_out=.' '--go-grpc_opt=paths=source_relative' $protoFile.FullName
        if ($LASTEXITCODE -ne 0) { throw "Protobuf generation failed: $($protoFile.Name)" }
    }
} finally {
    Pop-Location
}
