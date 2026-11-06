param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$agentRoot = Join-Path $repoRoot 'agent'
$tempBase = [System.IO.Path]::GetTempPath()
$smokeRoot = Join-Path $tempBase ('vit-c1-frequency-cleanup-smoke-' + [guid]::NewGuid().ToString('N'))
$priorMixboardRoot = $env:VIT_MIXBOARD_ROOT

New-Item -ItemType Directory -Path $smokeRoot | Out-Null
try {
    $env:VIT_MIXBOARD_ROOT = $smokeRoot
    Push-Location $agentRoot
    try {
        # G3 (SMOKE-SCRIPT-HYGIENE-1, 260): Windows PowerShell does not honor
        # $ErrorActionPreference for native-command exit codes, so an earlier
        # failing `go test` group used to fall through to a false-green exit 0.
        go test ./internal/frequencycleanup ./internal/capabilitycontext ./internal/capabilityadapters
        if ($LASTEXITCODE -ne 0) { throw "go test frequencycleanup/capabilitycontext/capabilityadapters failed with exit code $LASTEXITCODE" }
        go test ./internal/orchestration ./internal/orchestrationruntime
        if ($LASTEXITCODE -ne 0) { throw "go test orchestration/orchestrationruntime failed with exit code $LASTEXITCODE" }
        go test ./internal/chat ./internal/harness ./internal/mixboard
        if ($LASTEXITCODE -ne 0) { throw "go test chat/harness/mixboard failed with exit code $LASTEXITCODE" }
    }
    finally {
        Pop-Location
    }
    git -C $repoRoot diff --check
}
finally {
    $env:VIT_MIXBOARD_ROOT = $priorMixboardRoot
    $resolvedSmoke = (Resolve-Path -LiteralPath $smokeRoot -ErrorAction SilentlyContinue)
    if ($null -ne $resolvedSmoke -and $resolvedSmoke.Path.StartsWith($tempBase, [System.StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $resolvedSmoke.Path -Recurse -Force
    }
}
