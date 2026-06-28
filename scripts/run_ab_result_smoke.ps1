[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}

function Fail {
    param([string]$Message)
    throw $Message
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$AgentRoot = Join-Path $RepoRoot "agent"
if (-not (Test-Path -LiteralPath (Join-Path $AgentRoot "go.mod"))) {
    Fail ("agent go.mod not found under " + $AgentRoot)
}

$runPattern = "TestRequestObservationComputesABResultFromL2RenderProbeAcrossRounds|TestRequestObservationComputesABResultFromExplicitPreviousObservationAcrossSessions|TestRequestObservationMarksABResultStaleWhenRenderRevisionIsReused|TestABResultLayerUsesCompactRenderProbeResult|TestPendingMixTickReportIncludesReadyABResult|TestPendingMixTickReportDoesNotTrustMissingABResult|TestProjectResultCardIncludesReadyABResult|TestProjectResultCardIncludesMissingABResult|TestPluginPrepWorkerConfirmationWritesAndReobserves"

Write-Step "Run MOM v1.4 AB result smoke"
Push-Location $AgentRoot
try {
    & go test ./internal/mixboard ./internal/mom ./internal/chat -run $runPattern -count=1
    if ($LASTEXITCODE -ne 0) {
        Fail ("AB result smoke failed with exit code " + $LASTEXITCODE)
    }
}
finally {
    Pop-Location
}

Write-Ok "AB result smoke passed"
