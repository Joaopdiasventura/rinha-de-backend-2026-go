param(
    [string]$ArtifactsDir = ".artifacts/bench",
    [string]$BaselineFile = ""
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

New-Item -ItemType Directory -Force -Path $ArtifactsDir | Out-Null
$currentFile = Join-Path $ArtifactsDir "current.txt"
$benchstatFile = Join-Path $ArtifactsDir "benchstat.txt"

Write-Host "==> go test -run '^$' -bench . -benchmem ./internal/score ./internal/vector"
go test -run '^$' -bench . -benchmem ./internal/score ./internal/vector | Tee-Object -FilePath $currentFile
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($BaselineFile -and (Test-Path $BaselineFile)) {
    Write-Host "==> go run golang.org/x/perf/cmd/benchstat@latest `"$BaselineFile`" `"$currentFile`""
    go run golang.org/x/perf/cmd/benchstat@latest $BaselineFile $currentFile | Tee-Object -FilePath $benchstatFile
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
