param(
    [switch]$SkipRace
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

Write-Host "==> go build ./cmd/main.go"
go build ./cmd/main.go
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> go test ./..."
go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if (-not $SkipRace) {
    if ($IsLinux) {
        Write-Host "==> CGO_ENABLED=1 go test -race ./..."
        $env:CGO_ENABLED = "1"
        go test -race ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } else {
        Write-Host "==> skipping go test -race ./... outside Linux; CI workflow enforces it"
    }
}
