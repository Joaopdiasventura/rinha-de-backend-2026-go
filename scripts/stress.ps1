param(
    [switch]$Build,
    [switch]$SkipVerify,
    [string]$ProjectName = "rinha-stress",
    [int]$PublicPort = 9999,
    [string]$ArtifactsDir = ".artifacts/stress"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

$env:COMPOSE_PROJECT_NAME = $ProjectName
$env:PUBLIC_PORT = "$PublicPort"
$env:DIAGNOSTICS_ENABLED = "1"
$env:DIAGNOSTICS_PORT = "6060"
$networkName = "$ProjectName-app-network"
$script:StressFailed = $false

function Wait-ServiceHealth {
    param([string]$ServiceName)

    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        $containerId = (docker compose ps -q $ServiceName).Trim()
        if ($containerId) {
            $inspect = docker inspect $containerId | ConvertFrom-Json
            $state = $inspect[0].State
            $status = if ($state.Health) { $state.Health.Status } else { $state.Status }
            if ($status -eq "healthy" -or $status -eq "running") {
                return
            }
        }
        Start-Sleep -Seconds 2
    }

    throw "service $ServiceName did not become healthy"
}

function Wait-ReadyEndpoint {
    param([string]$Url)

    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        try {
            Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 | Out-Null
            return
        } catch {
            Start-Sleep -Seconds 2
        }
    }

    throw "endpoint did not become ready: $Url"
}

function Capture-Runtime {
    param([string]$ServiceName, [string]$OutputFile)

    docker compose exec -T $ServiceName wget -qO- "http://127.0.0.1:6060/debug/runtime/metrics" | Set-Content $OutputFile
}

function Capture-Profile {
    param([string]$ServiceName, [string]$Url, [string]$OutputFile)

    $containerId = (docker compose ps -q $ServiceName).Trim()
    $outputDir = Split-Path $OutputFile -Parent
    if (-not (Test-Path $outputDir)) {
        New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
    }

    $tempFile = "/tmp/" + [System.IO.Path]::GetFileName($OutputFile)
    docker compose exec -T $ServiceName sh -lc "wget -qO '$tempFile' '$Url'"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    docker cp "${containerId}:$tempFile" $OutputFile | Out-Null
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    docker compose exec -T $ServiceName rm -f $tempFile | Out-Null
}

function Run-K6 {
    param(
        [string]$Scenario,
        [string]$Rate,
        [string]$Duration
    )

    $summaryFile = Join-Path $ArtifactsDir "$Scenario-summary.json"
    $payloadDir = (Resolve-Path "test/load").Path
    $artifactDirAbs = (Resolve-Path $ArtifactsDir).Path

    docker run --rm `
        --network $networkName `
        -e K6_NO_USAGE_REPORT=true `
        -e TARGET_BASE_URL=http://nginx `
        -e RINHA_SCENARIO=$Scenario `
        -e RINHA_RATE=$Rate `
        -e RINHA_DURATION=$Duration `
        -e RINHA_PRE_ALLOCATED_VUS=100 `
        -e RINHA_MAX_VUS=250 `
        -e RETRY_ON_503=1 `
        -e RETRY_DELAY_MS=50 `
        -v "${payloadDir}:/load" `
        -v "${artifactDirAbs}:/artifacts" `
        grafana/k6:latest run --summary-export "/artifacts/$Scenario-summary.json" /load/stress.js | Tee-Object -FilePath (Join-Path $ArtifactsDir "$Scenario.txt")
    if ($LASTEXITCODE -ne 0) { $script:StressFailed = $true }

    go run ./cmd/loadreport -file $summaryFile | Tee-Object -FilePath (Join-Path $ArtifactsDir "$Scenario-report.txt")
    if ($LASTEXITCODE -ne 0) { $script:StressFailed = $true }
}

try {
    New-Item -ItemType Directory -Force -Path $ArtifactsDir | Out-Null

    if (-not $SkipVerify) {
        & (Join-Path $PSScriptRoot "verify.ps1")
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }

    & (Join-Path $PSScriptRoot "bench.ps1") -ArtifactsDir (Join-Path $ArtifactsDir "bench")
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    if ($Build) {
        docker compose up -d --build
    } else {
        docker compose up -d
    }
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Wait-ServiceHealth "app1"
    Wait-ServiceHealth "app2"
    Wait-ServiceHealth "nginx"
    Wait-ReadyEndpoint "http://127.0.0.1:$PublicPort/ready"

    Capture-Runtime "app1" (Join-Path $ArtifactsDir "app1-runtime-before.json")
    Capture-Runtime "app2" (Join-Path $ArtifactsDir "app2-runtime-before.json")

    Run-K6 "burst" "300" "20s"
    Run-K6 "ramp" "600" "30s"
    Run-K6 "soak" "150" "45s"

    $job = Start-Job -ScriptBlock {
        param($repoRoot)
        Set-Location $repoRoot
        Start-Sleep -Seconds 5
        docker compose stop app2 *> $null
        Start-Sleep -Seconds 5
        docker compose start app2 *> $null
    } -ArgumentList $root.Path

    Run-K6 "degrade" "120" "20s"
    Receive-Job $job -Wait -AutoRemoveJob -ErrorAction SilentlyContinue | Out-Null
    Wait-ServiceHealth "app2"

    Capture-Runtime "app1" (Join-Path $ArtifactsDir "app1-runtime-after.json")
    Capture-Runtime "app2" (Join-Path $ArtifactsDir "app2-runtime-after.json")

    go run ./cmd/runtimecompare -before (Join-Path $ArtifactsDir "app1-runtime-before.json") -after (Join-Path $ArtifactsDir "app1-runtime-after.json") | Tee-Object -FilePath (Join-Path $ArtifactsDir "app1-runtime-check.txt")
    if ($LASTEXITCODE -ne 0) { $script:StressFailed = $true }

    go run ./cmd/runtimecompare -before (Join-Path $ArtifactsDir "app2-runtime-before.json") -after (Join-Path $ArtifactsDir "app2-runtime-after.json") | Tee-Object -FilePath (Join-Path $ArtifactsDir "app2-runtime-check.txt")
    if ($LASTEXITCODE -ne 0) { $script:StressFailed = $true }

    if ($script:StressFailed) {
        exit 1
    }
} finally {
    docker compose ps | Set-Content (Join-Path $ArtifactsDir "compose-ps.txt")
    docker compose logs --no-color | Set-Content (Join-Path $ArtifactsDir "compose.log")
    docker stats --no-stream | Set-Content (Join-Path $ArtifactsDir "docker-stats.txt")
    docker compose down -v --remove-orphans | Set-Content (Join-Path $ArtifactsDir "compose-down.txt")
}
