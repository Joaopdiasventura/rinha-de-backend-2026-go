param(
    [switch]$SkipComposeBuild,
    [switch]$SkipVerify,
    [string]$ProjectName = "rinha-ci",
    [int]$PublicPort = 9999,
    [string]$ArtifactsDir = ".artifacts/official-tests",
    [string]$OfficialRepoUrl = "https://github.com/zanfranceschi/rinha-de-backend-2026.git",
    [string]$OfficialRef = "main"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

$env:COMPOSE_PROJECT_NAME = $ProjectName
$env:PUBLIC_PORT = "$PublicPort"
$officialSrcDir = Join-Path $ArtifactsDir "official-src"
$k6WorkDir = Join-Path $ArtifactsDir "k6"
$resultsFile = Join-Path $k6WorkDir "test/results.json"
$networkName = "$ProjectName-app-network"

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

function Assert-Shard {
    param(
        [string]$ServiceName,
        [string]$ShardId,
        [string]$ExpectedVecBytes,
        [string]$ExpectedLabelBytes
    )

    $vecPath = "/app/resources/$ShardId/references.vec"
    $labelPath = "/app/resources/$ShardId/references.labels"

    docker compose exec -T $ServiceName sh -lc "test -f '$vecPath' && test -f '$labelPath'" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "missing shard files for $ServiceName shard $ShardId" }

    $vecBytes = (docker compose exec -T $ServiceName sh -lc "wc -c < '$vecPath'").Trim()
    $labelBytes = (docker compose exec -T $ServiceName sh -lc "wc -c < '$labelPath'").Trim()

    if ($vecBytes -ne $ExpectedVecBytes) {
        throw "unexpected vec bytes for ${ServiceName} shard ${ShardId}: got $vecBytes want $ExpectedVecBytes"
    }

    if ($labelBytes -ne $ExpectedLabelBytes) {
        throw "unexpected label bytes for ${ServiceName} shard ${ShardId}: got $labelBytes want $ExpectedLabelBytes"
    }
}

try {
    if (-not $SkipVerify) {
        & (Join-Path $PSScriptRoot "verify.ps1")
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }

    New-Item -ItemType Directory -Force -Path $ArtifactsDir | Out-Null
    Remove-Item -Recurse -Force $officialSrcDir, $k6WorkDir -ErrorAction SilentlyContinue

    git clone --depth 1 --branch $OfficialRef $OfficialRepoUrl $officialSrcDir
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    New-Item -ItemType Directory -Force -Path (Join-Path $k6WorkDir "test") | Out-Null
    Copy-Item (Join-Path $officialSrcDir "test/smoke.js") (Join-Path $k6WorkDir "test/smoke.js")
    Copy-Item (Join-Path $officialSrcDir "test/test.js") (Join-Path $k6WorkDir "test/test.js")
    Copy-Item (Join-Path $officialSrcDir "test/test-data.json") (Join-Path $k6WorkDir "test/test-data.json")

    foreach ($file in @((Join-Path $k6WorkDir "test/smoke.js"), (Join-Path $k6WorkDir "test/test.js"))) {
        (Get-Content $file -Raw).Replace("http://localhost:9999", "http://nginx") | Set-Content $file
    }

    docker compose config | Set-Content (Join-Path $ArtifactsDir "docker-compose-config.yml")

    if ($SkipComposeBuild) {
        docker compose up -d --no-build
    } else {
        docker compose up -d --build
    }
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Wait-ServiceHealth "app1"
    Wait-ServiceHealth "app2"
    Wait-ServiceHealth "nginx"
    Wait-ReadyEndpoint "http://127.0.0.1:$PublicPort/ready"

    Assert-Shard "app1" "0" "84000000" "1500000"
    Assert-Shard "app2" "1" "84000000" "1500000"

    $mountPath = (Resolve-Path $k6WorkDir).Path
    docker run --rm --network $networkName -e K6_NO_USAGE_REPORT=true -v "${mountPath}:/work" -w /work grafana/k6:latest run /work/test/smoke.js | Tee-Object -FilePath (Join-Path $ArtifactsDir "k6-smoke.txt")
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    docker run --rm --network $networkName -e K6_NO_USAGE_REPORT=true -v "${mountPath}:/work" -w /work grafana/k6:latest run /work/test/test.js | Tee-Object -FilePath (Join-Path $ArtifactsDir "k6-official.txt")
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    go run ./cmd/officialcheck -file $resultsFile | Tee-Object -FilePath (Join-Path $ArtifactsDir "official-summary.txt")
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    docker compose ps | Set-Content (Join-Path $ArtifactsDir "compose-ps.txt")
    docker compose logs --no-color | Set-Content (Join-Path $ArtifactsDir "compose.log")
    docker stats --no-stream | Set-Content (Join-Path $ArtifactsDir "docker-stats.txt")
    if (Test-Path $resultsFile) {
        Copy-Item $resultsFile (Join-Path $ArtifactsDir "results.json") -Force
    }
    docker compose down -v --remove-orphans | Set-Content (Join-Path $ArtifactsDir "compose-down.txt")
}
